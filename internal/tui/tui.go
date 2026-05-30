package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Thitipong-PP/palet/internal/executor"
	"github.com/Thitipong-PP/palet/internal/index"
	"github.com/Thitipong-PP/palet/internal/plugin"
	"github.com/Thitipong-PP/palet/internal/search"
	"github.com/Thitipong-PP/palet/internal/validator"
)

// ---------------------------------------------------------------------------
// Theme
// ---------------------------------------------------------------------------

var (
	cBrand   = lipgloss.Color("#8eb0ff") // brand pink
	cBrand2  = lipgloss.Color("#a8c2ff") // soft violet
	cBlue    = lipgloss.Color("#e2eafc")
	cText    = lipgloss.Color("231")
	cSubtle  = lipgloss.Color("252")
	cMuted   = lipgloss.Color("245")
	cFaint   = lipgloss.Color("240")
	cFainter = lipgloss.Color("237")
	cGreen   = lipgloss.Color("114")
	cRed     = lipgloss.Color("203")
	cSelBg   = lipgloss.Color("236")
	cCardBd  = lipgloss.Color("238")

	styleLogo     = lipgloss.NewStyle().Foreground(cBrand).Bold(true)
	styleSubtitle = lipgloss.NewStyle().Foreground(cFaint).Italic(true)
	styleDim      = lipgloss.NewStyle().Foreground(cFaint)
	styleMuted    = lipgloss.NewStyle().Foreground(cMuted)
	styleDesc     = lipgloss.NewStyle().Foreground(cMuted)
	styleCmd      = lipgloss.NewStyle().Foreground(cSubtle)
	stylePlugin   = lipgloss.NewStyle().Foreground(cBrand2).Bold(true)
	styleError    = lipgloss.NewStyle().Foreground(cRed)
	styleOk       = lipgloss.NewStyle().Foreground(cGreen).Bold(true)

	styleSelName = lipgloss.NewStyle().Foreground(cText).Bold(true)
	styleSelBar  = lipgloss.NewStyle().Foreground(cBrand)

	styleGroupBar  = lipgloss.NewStyle().Foreground(cBrand)
	styleGroupName = lipgloss.NewStyle().Foreground(cBrand2).Bold(true)
	styleCount     = lipgloss.NewStyle().Foreground(cFaint)

	styleSectionLabel = lipgloss.NewStyle().Foreground(cFaint).Bold(true)

	styleKeyCap = lipgloss.NewStyle().Foreground(cText).Background(cFainter).Bold(true).Padding(0, 1)
	styleKeyLbl = lipgloss.NewStyle().Foreground(cMuted)
)

// keyHint is a single footer key/label pair.
type keyHint struct{ key, label string }

// badge renders a small filled pill, e.g. an arg type or a tag.
func badge(text string, fg, bg lipgloss.Color) string {
	if text == "" {
		return ""
	}
	return lipgloss.NewStyle().Foreground(fg).Background(bg).Padding(0, 1).Render(text)
}

// ---------------------------------------------------------------------------
// Model
// ---------------------------------------------------------------------------

type viewState int

const (
	viewList viewState = iota
	viewForm
	viewResult
)

// group is one plugin's bucket of (filtered) entries, preserving the order in
// which the plugin first appeared in the search results.
type group struct {
	plugin  string
	entries []index.Entry
}

type model struct {
	all    []index.Entry // immutable full command set
	groups []group       // current filtered results, grouped by plugin
	flat   []index.Entry // filtered results flattened in display order

	state viewState

	// list view
	searchInput textinput.Model
	cursor      int // index into flat (entries only, never headers)
	listOffset  int // first visible display row
	lastQuery   string

	// form view
	selected index.Entry
	inputs   []textinput.Model
	focus    int
	formErr  string

	// result view
	builtCmd    string
	buildErr    error
	resultInput textinput.Model
	copyMsg     string
	copyErr     bool

	width  int
	height int

	scanCurrent int
	scanTotal   int
	scanName    string
	scanActive  bool

	// read by Start after the program exits.
	pendingAction string
}

// Start opens the Bubble Tea program and, once it closes, runs the built
// command if the user chose to execute it.
func Start(entries []index.Entry) error {
	si := textinput.New()
	si.Prompt = "⌕  "
	si.Placeholder = "Search commands…"
	si.PromptStyle = lipgloss.NewStyle().Foreground(cBrand)
	si.PlaceholderStyle = styleDim
	si.TextStyle = lipgloss.NewStyle().Foreground(cText)
	si.Cursor.Style = lipgloss.NewStyle().Foreground(cBrand)
	si.Focus()

	m := model{
		all:         entries,
		searchInput: si,
		state:       viewList,
	}
	m.applySearch("")

	p := tea.NewProgram(m, tea.WithAltScreen())
	go func() {
		for msg := range index.ScanProgressChan() {
			p.Send(msg)
		}
	}()
	if msg, ok := index.CurrentScanProgress(); ok {
		p.Send(msg)
	}
	final, err := p.Run()
	if err != nil {
		return err
	}

	fm, ok := final.(model)
	if !ok {
		return nil
	}
	if fm.pendingAction == "execute" && fm.builtCmd != "" {
		return executor.Run(fm.builtCmd)
	}
	return nil
}

// ---------------------------------------------------------------------------
// tea.Model
// ---------------------------------------------------------------------------

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.searchInput.Width = max(10, m.width-16)
		for i := range m.inputs {
			m.inputs[i].Width = max(10, m.width-14)
		}
		if m.resultInput.Value() != "" {
			m.resultInput.Width = max(10, m.width-14)
		}
		m.clampOffset()
		return m, nil

	case index.ScanProgress:
		m.scanCurrent = msg.Current
		m.scanTotal = msg.Total
		m.scanName = msg.Name
		m.scanActive = !msg.Done && msg.Total > 0
		return m, nil

	case tea.KeyMsg:
		switch m.state {
		case viewList:
			return m.updateList(msg)
		case viewForm:
			return m.updateForm(msg)
		case viewResult:
			return m.updateResult(msg)
		}
	}

	// Forward any other (non-key) message to the currently focused input.
	return m.forward(msg)
}

func (m model) View() string {
	switch m.state {
	case viewForm:
		return m.viewForm()
	case viewResult:
		return m.viewResult()
	default:
		return m.viewList()
	}
}

// ---------------------------------------------------------------------------
// List view
// ---------------------------------------------------------------------------

func (m model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "q":
		// Honour the "q quits" hint while the search box is empty; once the
		// user is typing a query, q is just another character.
		if m.searchInput.Value() == "" {
			return m, tea.Quit
		}

	case "esc":
		if m.searchInput.Value() != "" {
			m.searchInput.SetValue("")
			m.applySearch("")
			return m, nil
		}
		return m, tea.Quit

	case "up", "ctrl+p":
		m.moveCursor(-1)
		return m, nil

	case "down", "ctrl+n":
		m.moveCursor(1)
		return m, nil

	case "enter":
		if len(m.flat) > 0 {
			m.openForm(m.flat[m.cursor])
		}
		return m, nil
	}

	// Everything else is search input.
	prev := m.searchInput.Value()
	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	if m.searchInput.Value() != prev {
		m.applySearch(m.searchInput.Value())
	}
	return m, cmd
}

// applySearch refilters the entries and rebuilds the grouped + flat views.
func (m *model) applySearch(query string) {
	changed := query != m.lastQuery
	m.lastQuery = query
	results := search.Query(m.all, query)
	m.groups = buildGroups(results)

	m.flat = m.flat[:0]
	for _, g := range m.groups {
		m.flat = append(m.flat, g.entries...)
	}

	if changed {
		m.cursor = 0
	} else {
		if m.cursor >= len(m.flat) {
			m.cursor = len(m.flat) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
	}
	m.listOffset = 0
	m.clampOffset()
}

// buildGroups buckets entries by plugin, ordering groups by the plugin's first
// appearance in the (already ranked) results so grouping survives filtering.
func buildGroups(results []index.Entry) []group {
	var groups []group
	idx := make(map[string]int, len(results))
	for _, e := range results {
		name := e.Plugin.Name
		gi, ok := idx[name]
		if !ok {
			groups = append(groups, group{plugin: name})
			gi = len(groups) - 1
			idx[name] = gi
		}
		groups[gi].entries = append(groups[gi].entries, e)
	}
	return groups
}

func (m *model) moveCursor(delta int) {
	if len(m.flat) == 0 {
		return
	}
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.flat) {
		m.cursor = len(m.flat) - 1
	}
	m.clampOffset()
}

func (m model) viewList() string {
	var b strings.Builder
	b.WriteString("\n" + m.topBar(cwdLabel()) + "\n\n")
	b.WriteString(m.logoIcon() + "\n\n")
	b.WriteString(m.searchView() + "\n\n")

	if len(m.flat) == 0 {
		b.WriteString("  " + styleDim.Render("✗ no commands match your search") + "\n")
		b.WriteString("\n" + renderHints([]keyHint{{"esc", "clear"}, {"q", "quit"}}))
		return b.String()
	}

	rows, _ := m.listRows()
	top, bottom := m.windowBounds(len(rows))
	for _, r := range rows[top:bottom] {
		b.WriteString(r + "\n")
	}
	if m.scanActive {
		label := fmt.Sprintf("scan %d/%d", m.scanCurrent, m.scanTotal)
		if m.scanName != "" {
			label += " " + m.scanName
		}
		b.WriteString("\n  " + styleDim.Render(label) + "\n")
	}

	b.WriteString("\n" + renderHints([]keyHint{
		{"↑/↓", "navigate"}, {"enter", "select"}, {"esc", "clear"}, {"q", "quit"},
	}))
	return b.String()
}

// searchView wraps the search input in a rounded, accent-bordered field.
func (m model) searchView() string {
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cBrand).
		Padding(0, 1).
		MarginLeft(2).
		Width(max(12, m.w()-8))
	return box.Render(m.searchInput.View())
}

// listRows renders every group into display rows and reports the row index of
// the currently selected entry (so the viewport can keep it visible).
func (m model) listRows() (rows []string, cursorRow int) {
	flatIdx := 0
	for gi, g := range m.groups {
		if gi > 0 {
			rows = append(rows, "")
		}
		header := "  " + styleGroupBar.Render("▎") + " " +
			styleGroupName.Render(strings.ToUpper(g.plugin)) + "  " +
			styleCount.Render(fmt.Sprintf("%d", len(g.entries)))
		rows = append(rows, header)
		for _, e := range g.entries {
			selected := flatIdx == m.cursor
			if selected {
				cursorRow = len(rows)
			}
			rows = append(rows, m.renderEntry(e, selected))
			flatIdx++
		}
	}
	return rows, cursorRow
}

func (m model) renderEntry(e index.Entry, selected bool) string {
	const nameW = 16
	name := e.Command.Name
	if e.Plugin.Description == "system command" && e.Command.Name != e.Plugin.Name {
		name = e.Plugin.Name + " " + e.Command.Name
	}
	name = pad(name, nameW)
	desc := e.Command.Description

	if selected {
		inner := styleSelName.Background(cSelBg).Render(" " + name)
		if desc != "" {
			inner += lipgloss.NewStyle().Foreground(cSubtle).Background(cSelBg).Render("  " + desc)
		}
		row := lipgloss.NewStyle().Background(cSelBg).Width(max(4, m.w()-5)).Render(inner)
		return styleSelBar.Render("▌") + row
	}

	line := "   " + styleCmd.Render(name)
	if desc != "" {
		line += "  " + styleDesc.Render(desc)
	}
	return line
}

// ---------------------------------------------------------------------------
// Scrolling
// ---------------------------------------------------------------------------

// listHeight is how many list rows fit between the chrome (brand bar, bordered
// search field, footer).
func (m model) listHeight() int {
	logoH := lipgloss.Height(m.logoIcon())
	searchH := lipgloss.Height(m.searchView())
	// 1 blank + top bar + 2 blanks + logo + 2 blanks + search + 2 blanks + (blank + hints)
	chrome := logoH + searchH + 10
	if m.scanActive {
		chrome++
	}
	h := m.height - chrome
	if h < 3 {
		h = 3
	}
	return h
}

func (m *model) clampOffset() {
	rows, cursorRow := m.listRows()
	visible := m.listHeight()

	maxOffset := len(rows) - visible
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.listOffset > maxOffset {
		m.listOffset = maxOffset
	}
	if m.listOffset < 0 {
		m.listOffset = 0
	}

	// Keep the cursor (and the header just above it when possible) in view.
	if cursorRow < m.listOffset {
		m.listOffset = cursorRow
	}
	if cursorRow >= m.listOffset+visible {
		m.listOffset = cursorRow - visible + 1
	}
}

func (m model) windowBounds(total int) (top, bottom int) {
	visible := m.listHeight()
	top = m.listOffset
	if top > total {
		top = total
	}
	bottom = top + visible
	if bottom > total {
		bottom = total
	}
	return top, bottom
}

// ---------------------------------------------------------------------------
// Form view
// ---------------------------------------------------------------------------

func (m *model) openForm(e index.Entry) {
	m.selected = e
	m.formErr = ""
	m.focus = 0
	m.inputs = make([]textinput.Model, len(e.Command.Args))

	for i, arg := range e.Command.Args {
		ti := textinput.New()
		ti.Placeholder = placeholderFor(arg)
		ti.PlaceholderStyle = styleDim
		ti.TextStyle = lipgloss.NewStyle().Foreground(cText)
		ti.Cursor.Style = lipgloss.NewStyle().Foreground(cBrand)
		ti.Prompt = ""
		ti.SetValue(arg.Default)
		ti.CharLimit = 512
		ti.Width = max(10, m.w()-14)
		if i == 0 {
			ti.Focus()
		}
		m.inputs[i] = ti
	}
	m.state = viewForm
}

func placeholderFor(arg plugin.Arg) string {
	if len(arg.Choices) > 0 {
		return strings.Join(arg.Choices, " | ")
	}
	if arg.Description != "" {
		return arg.Description
	}
	return arg.Name
}

func (m model) updateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "esc":
		m.state = viewList
		return m, nil

	case "tab", "down":
		m.focusField(m.focus + 1)
		return m, nil

	case "shift+tab", "up":
		m.focusField(m.focus - 1)
		return m, nil

	case "enter":
		if m.focus < len(m.inputs)-1 {
			m.focusField(m.focus + 1)
			return m, nil
		}
		return m.submitForm()
	}

	// Forward to the focused field.
	if len(m.inputs) > 0 {
		var cmd tea.Cmd
		m.inputs[m.focus], cmd = m.inputs[m.focus].Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *model) focusField(i int) {
	if len(m.inputs) == 0 {
		return
	}
	if i < 0 {
		i = 0
	}
	if i >= len(m.inputs) {
		i = len(m.inputs) - 1
	}
	for j := range m.inputs {
		if j == i {
			m.inputs[j].Focus()
		} else {
			m.inputs[j].Blur()
		}
	}
	m.focus = i
}

func (m model) submitForm() (tea.Model, tea.Cmd) {
	vals := make(map[string]string, len(m.inputs))
	for i, arg := range m.selected.Command.Args {
		vals[arg.Name] = strings.TrimSpace(m.inputs[i].Value())
	}

	if missing := validator.MissingArgs(m.selected.Command, vals); len(missing) > 0 {
		m.formErr = "missing required: " + strings.Join(missing, ", ")
		return m, nil
	}

	cmd, err := executor.BuildCommand(m.selected.Command.Template, vals)
	m.builtCmd = cmd
	m.buildErr = err
	m.resultInput = textinput.New()
	m.resultInput.Prompt = "$ "
	m.resultInput.PromptStyle = lipgloss.NewStyle().Foreground(cGreen)
	m.resultInput.TextStyle = styleCmd
	m.resultInput.Cursor.Style = lipgloss.NewStyle().Foreground(cBrand)
	m.resultInput.CharLimit = 2048
	m.resultInput.Width = max(10, m.w()-14)
	m.resultInput.SetValue(cmd)
	m.resultInput.Focus()
	m.copyMsg = ""
	m.copyErr = false
	m.state = viewResult
	return m, nil
}

func (m model) viewForm() string {
	var b strings.Builder
	cmd := m.selected.Command

	b.WriteString("\n" + m.topBar(cwdLabel()) + "\n\n")
	b.WriteString(m.logoIcon() + "\n\n")

	// Breadcrumb: plugin › command, with the description underneath.
	b.WriteString("  " + stylePlugin.Render(m.selected.Plugin.Name) +
		styleDim.Render("  ›  ") + styleCmd.Render(cmd.Name) + "\n")
	if cmd.Description != "" {
		b.WriteString("  " + styleDesc.Render(cmd.Description) + "\n")
	}
	b.WriteString("\n")

	if len(cmd.Args) == 0 {
		b.WriteString(renderHints([]keyHint{{"enter", "build"}, {"esc", "back"}}))
		return b.String()
	}

	for i, arg := range cmd.Args {
		focused := i == m.focus

		// Label row: accent bar + name + type/required badges.
		var label string
		if focused {
			label = styleSelBar.Render("▌") + " " + styleSelName.Render(arg.Name)
		} else {
			label = "  " + styleMuted.Render(arg.Name)
		}
		if arg.Required {
			label += " " + badge("required", cText, cRed)
		} else {
			label += " " + badge("optional", cMuted, cFainter)
		}
		if arg.Type != "" {
			label += " " + badge(arg.Type, cBlue, cFainter)
		}
		b.WriteString(label + "\n")

		if arg.Description != "" {
			b.WriteString("    " + styleDesc.Render(arg.Description) + "\n")
		}

		// Input field, bordered; the border lights up when focused.
		bd := cCardBd
		if focused {
			bd = cBrand
		}
		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(bd).
			Padding(0, 1).
			MarginLeft(2).
			Width(max(10, m.w()-8))
		b.WriteString(box.Render(m.inputs[i].View()) + "\n")

		if arg.Default != "" {
			b.WriteString("    " + styleDim.Render("default: "+arg.Default) + "\n")
		}
		b.WriteString("\n")
	}

	if m.formErr != "" {
		b.WriteString("  " + badge("!", cText, cRed) + " " + styleError.Render(m.formErr) + "\n\n")
	}

	b.WriteString(renderHints([]keyHint{
		{"tab/↑↓", "move"}, {"enter", "next/submit"}, {"esc", "back"},
	}))
	return b.String()
}

// ---------------------------------------------------------------------------
// Result view
// ---------------------------------------------------------------------------

func (m model) updateResult(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.builtCmd = strings.TrimSpace(m.resultInput.Value())
		m.pendingAction = "execute"
		return m, tea.Quit

	case "ctrl+c":
		return m, tea.Quit

	case "esc":
		m.state = viewForm
		m.copyMsg = ""
		m.copyErr = false
		return m, nil
	}

	if m.buildErr != nil {
		// Only esc/quit are valid when the build failed.
		return m, nil
	}

	switch msg.String() {
	case "ctrl+o":
		m.builtCmd = strings.TrimSpace(m.resultInput.Value())
		if err := executor.Copy(m.builtCmd); err != nil {
			m.copyMsg = err.Error()
			m.copyErr = true
		} else {
			m.copyMsg = "copied to clipboard"
			m.copyErr = false
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.resultInput, cmd = m.resultInput.Update(msg)
	return m, cmd
}

func (m model) viewResult() string {
	var b strings.Builder
	b.WriteString("\n" + m.topBar("") + "\n\n")
	b.WriteString(m.logoIcon() + "\n\n")

	if m.buildErr != nil {
		b.WriteString("  " + styleSectionLabel.Render("BUILD FAILED") + "\n")
		b.WriteString(m.card(styleError.Render("✗ "+m.buildErr.Error()), cRed) + "\n\n")
		b.WriteString(renderHints([]keyHint{{"esc", "back"}, {"q", "quit"}}))
		return b.String()
	}

	b.WriteString("  " + styleSectionLabel.Render("BUILT COMMAND") + "\n")
	input := m.resultInput.View()
	b.WriteString(m.card(input, cCardBd) + "\n")

	if m.copyMsg != "" {
		if m.copyErr {
			b.WriteString("\n  " + styleError.Render("✗ "+m.copyMsg) + "\n")
		} else {
			b.WriteString("\n  " + styleOk.Render("✓ "+m.copyMsg) + "\n")
		}
	}
	b.WriteString("\n")

	b.WriteString(renderHints([]keyHint{
		{"enter", "execute"}, {"ctrl+o", "copy"}, {"esc", "back"}, {"ctrl+c", "quit"},
	}))
	return b.String()
}

// card renders content inside a rounded, indented box.
func (m model) card(content string, border lipgloss.Color) string {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1).
		MarginLeft(2).
		Width(max(10, m.w()-8)).
		Render(content)
}

// ---------------------------------------------------------------------------
// Helpers (cont.)
// ---------------------------------------------------------------------------

// forward sends a non-key message to whichever input currently has focus.
func (m model) forward(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.state {
	case viewList:
		m.searchInput, cmd = m.searchInput.Update(msg)
	case viewForm:
		if len(m.inputs) > 0 {
			m.inputs[m.focus], cmd = m.inputs[m.focus].Update(msg)
		}
	}
	return m, cmd
}

// w returns the usable width, falling back to a sane default before the first
// WindowSizeMsg arrives.
func (m model) w() int {
	if m.width > 4 {
		return m.width
	}
	return 80
}

// pad right-pads s with spaces to at least n display columns.
func pad(s string, n int) string {
	if w := lipgloss.Width(s); w < n {
		return s + strings.Repeat(" ", n-w)
	}
	return s
}

// topBar renders the brand line: a palette of dots, the wordmark, and an
// optional right-aligned context string.
func (m model) topBar(right string) string {
	dots := lipgloss.NewStyle().Foreground(cBrand).Render("●") +
		lipgloss.NewStyle().Foreground(cBrand2).Render("●") +
		lipgloss.NewStyle().Foreground(cBlue).Render("●")
	left := "  " + dots + "  " +
		styleSubtitle.Render("command palette")
	if right == "" {
		return left
	}
	right = styleDim.Render(right)
	gap := m.w() - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if gap < 1 {
		return left
	}
	return left + strings.Repeat(" ", gap) + right
}

// logoIcon renders a big, colorful "P" logo for the palette.
func (m *model) logoIcon() string {
	style1 := lipgloss.NewStyle().Foreground(cBrand)
	style2 := lipgloss.NewStyle().Foreground(cBrand2)
	style3 := lipgloss.NewStyle().Foreground(cBlue)
	maxW := m.w() - 2

	row1 := style1.Render(trimLogoRow("  █████████     █████     ██          ██████████  ███████████", maxW))
	row2 := style2.Render(trimLogoRow("  ██      ██   ██   ██    ██          ██          ███████████", maxW))
	row3 := style3.Render(trimLogoRow("  █████████   ██     ██   ██          ████████        ███", maxW))
	row4 := style2.Render(trimLogoRow("  ██         ███████████  ██          ██              ███", maxW))
	row5 := style1.Render(trimLogoRow("  ██         ██       ██  ██████████  ██████████      ███", maxW))

	return lipgloss.JoinVertical(lipgloss.Left, row1, row2, row3, row4, row5)
}

func trimLogoRow(s string, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxW {
		return s
	}
	r := []rune(s)
	if len(r) > maxW {
		r = r[:maxW]
	}
	return string(r)
}

// renderHints draws the footer key/label pairs as little keycaps.
func renderHints(hints []keyHint) string {
	parts := make([]string, len(hints))
	for i, h := range hints {
		parts[i] = styleKeyCap.Render(h.key) + " " + styleKeyLbl.Render(h.label)
	}
	return "  " + strings.Join(parts, "   ")
}

// cwdLabel returns the working directory, abbreviating the home prefix to "~".
func cwdLabel() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(wd, home) {
		return "~" + wd[len(home):]
	}
	return filepath.Clean(wd)
}
