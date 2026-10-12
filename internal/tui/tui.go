package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Thitipong-PP/palet/internal/config"
	"github.com/Thitipong-PP/palet/internal/executor"
	"github.com/Thitipong-PP/palet/internal/history"
	"github.com/Thitipong-PP/palet/internal/index"
	"github.com/Thitipong-PP/palet/internal/project"
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

	cHighlight = lipgloss.Color("#ffb86c") // high-contrast gold/amber for search highlights

	styleSelName  = lipgloss.NewStyle().Foreground(cText).Bold(true)
	styleSelBar   = lipgloss.NewStyle().Foreground(cBrand)
	styleMatch    = lipgloss.NewStyle().Foreground(cHighlight).Bold(true)
	styleSelMatch = lipgloss.NewStyle().Foreground(cHighlight).Background(cSelBg).Bold(true)

	styleGroupBar  = lipgloss.NewStyle().Foreground(cBrand)
	styleGroupName = lipgloss.NewStyle().Foreground(cBrand2).Bold(true)
	styleCount     = lipgloss.NewStyle().Foreground(cFaint)

	styleSectionLabel = lipgloss.NewStyle().Foreground(cFaint).Bold(true)

	styleTag = lipgloss.NewStyle().Foreground(cBrand2)

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

// group is one category's bucket of (filtered) entries, preserving the order in
// which the category first appeared in the search results.
type group struct {
	name    string
	entries []index.Entry
}

type model struct {
	all         []index.Entry // immutable full command set
	groups      []group       // current filtered results, grouped by category
	flat        []index.Entry // filtered results flattened in display order
	projectInfo project.Info  // detected project context

	state viewState

	// list view
	searchInput textinput.Model
	cursor      int // index into flat (entries only, never headers)
	listOffset  int // first visible display row
	lastQuery   string
	showHelp    bool
	statusMsg   string

	// form view
	selected       index.Entry
	inputs         []textinput.Model
	focus          int
	formErr        string
	availableFlags []config.Flag
	flagsLoading   bool
	flagFilter     textinput.Model
	selectedFlags  map[string]bool
	flagCursor     int
	formSection    int // 0: args inputs, 1: flags section

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

type flagsLoadedMsg struct {
	cmdKey string
	flags  []config.Flag
}

func loadFlagsCmd(cmdKey string) tea.Cmd {
	return func() tea.Msg {
		flags := index.DiscoverCommandFlags(cmdKey)
		return flagsLoadedMsg{cmdKey: cmdKey, flags: flags}
	}
}

// Options defines startup configuration for the TUI.
type Options struct {
	InitialQuery string
	InitialTag   string
}

// Start opens the Bubble Tea program with default options.
func Start(entries []index.Entry) error {
	return StartWithOptions(entries, Options{})
}

// StartWithOptions opens the Bubble Tea program and, once it closes, runs the built
// command if the user chose to execute it.
func StartWithOptions(entries []index.Entry, opts Options) error {
	si := textinput.New()
	si.Prompt = "⌕  "
	si.Placeholder = "Search commands or #tag…"
	si.PromptStyle = lipgloss.NewStyle().Foreground(cBrand)
	si.PlaceholderStyle = styleDim
	si.TextStyle = lipgloss.NewStyle().Foreground(cText)
	si.Cursor.Style = lipgloss.NewStyle().Foreground(cBrand)
	si.Focus()

	initQ := opts.InitialQuery
	if opts.InitialTag != "" {
		initQ = "#" + opts.InitialTag
	}
	if initQ != "" {
		si.SetValue(initQ)
	}

	projInfo := project.Detect("")
	m := model{
		all:         entries,
		projectInfo: projInfo,
		searchInput: si,
		state:       viewList,
	}
	m.applySearch(initQ)

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
		_ = history.Record(fm.builtCmd)
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
		m.flagFilter.Width = max(10, m.width-14)
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

	case flagsLoadedMsg:
		cmdLookup := m.selected.Command.Template
		if cmdLookup == "" {
			cmdLookup = m.selected.Command.Name
		}
		if m.state == viewForm && msg.cmdKey == cmdLookup {
			m.availableFlags = msg.flags
			m.flagsLoading = false
			if len(m.inputs) == 0 && len(m.availableFlags) > 0 {
				m.formSection = 1
				m.flagFilter.Focus()
			}
		}
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
	if m.showHelp {
		switch msg.String() {
		case "esc", "q", "?", "f1":
			m.showHelp = false
			return m, nil
		}
		return m, nil
	}

	key := msg.String()
	switch key {
	case "ctrl+c":
		return m, tea.Quit

	case "?":
		if m.searchInput.Value() == "" {
			m.showHelp = true
			return m, nil
		}

	case "f1":
		m.showHelp = true
		return m, nil

	case "q":
		if m.searchInput.Value() == "" {
			return m, tea.Quit
		}

	case "esc":
		if m.searchInput.Value() != "" {
			m.searchInput.SetValue("")
			m.applySearch("")
			m.statusMsg = ""
			return m, nil
		}
		return m, tea.Quit

	case "up", "ctrl+p":
		m.moveCursor(-1)
		return m, nil

	case "down", "ctrl+n":
		m.moveCursor(1)
		return m, nil

	case "ctrl+r":
		if len(m.flat) > 0 {
			e := m.flat[m.cursor]
			vals := make(map[string]string)
			for _, a := range e.Command.Args {
				vals[a.Name] = a.Default
			}
			cmd, err := executor.BuildCommand(e.Command.Template, vals)
			if err != nil || cmd == "" {
				cmd = e.Command.Template
				if cmd == "" {
					cmd = e.Command.Name
				}
			}
			m.builtCmd = cmd
			m.pendingAction = "execute"
			return m, tea.Quit
		}
		return m, nil

	case "ctrl+o":
		if len(m.flat) > 0 {
			e := m.flat[m.cursor]
			cmd := e.Command.Template
			if cmd == "" {
				cmd = e.Command.Name
			}
			if err := executor.Copy(cmd); err != nil {
				m.statusMsg = "✗ copy failed: " + err.Error()
			} else {
				m.statusMsg = "✓ copied '" + cmd + "' to clipboard"
			}
			return m, nil
		}
		return m, nil

	case "enter":
		if len(m.flat) > 0 {
			cmd := m.openForm(m.flat[m.cursor])
			return m, cmd
		}
		return m, nil
	}

	// Everything else is search input.
	prev := m.searchInput.Value()
	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	if m.searchInput.Value() != prev {
		m.statusMsg = ""
		m.applySearch(m.searchInput.Value())
	}
	return m, cmd
}

// applySearch refilters the entries and rebuilds the grouped + flat views.
func (m *model) applySearch(query string) {
	changed := query != m.lastQuery
	m.lastQuery = query
	results := search.QueryWithContext(m.all, query, m.projectInfo.Suggestions)
	m.groups = buildGroups(results)

	if query == "" {
		// 1. Context Suggestions (top priority)
		if len(m.projectInfo.Suggestions) > 0 {
			var suggEntries []index.Entry
			seenSugg := make(map[string]bool)
			for _, sugg := range m.projectInfo.Suggestions {
				suggLower := strings.ToLower(sugg)
				for _, e := range m.all {
					fullName := strings.ToLower(e.Group.Name + " " + e.Command.Name)
					cmdName := strings.ToLower(e.Command.Name)
					tmpl := strings.ToLower(e.Command.Template)
					if (fullName == suggLower || cmdName == suggLower || tmpl == suggLower || strings.HasPrefix(fullName, suggLower) || strings.HasPrefix(cmdName, suggLower)) && !seenSugg[fullName] {
						cmdCopy := e.Command
						displayName := e.Command.Name
						if e.Group.Name != "" && e.Group.Name != "system" && !strings.HasPrefix(e.Command.Name, e.Group.Name) {
							displayName = e.Group.Name + " " + e.Command.Name
						} else if e.Command.Template != "" && !strings.Contains(e.Command.Template, "{{") {
							displayName = e.Command.Template
						}
						cmdCopy.Name = displayName

						suggEntries = append(suggEntries, index.Entry{
							Group: config.Group{
								Name:        "suggested",
								Description: "recommended for current project",
							},
							Command: cmdCopy,
						})
						seenSugg[fullName] = true
						break
					}
				}
				if len(suggEntries) >= 6 {
					break
				}
			}
			if len(suggEntries) > 0 {
				m.groups = append([]group{{name: "suggested for project", entries: suggEntries}}, m.groups...)
			}
		}

		// 2. Recent commands
		recentList := history.Recent()
		if len(recentList) > 0 {
			var recentEntries []index.Entry
			seen := make(map[string]bool)
			for _, rcmd := range recentList {
				for _, e := range m.all {
					key := e.Command.Template
					if key == "" {
						key = e.Command.Name
					}
					if (key == rcmd || e.Command.Name == rcmd || strings.HasPrefix(rcmd, key)) && !seen[key] {
						cmdCopy := e.Command
						displayName := e.Command.Name
						if e.Group.Name != "" && e.Group.Name != "system" && !strings.HasPrefix(e.Command.Name, e.Group.Name) {
							displayName = e.Group.Name + " " + e.Command.Name
						} else if e.Command.Template != "" && !strings.Contains(e.Command.Template, "{{") {
							displayName = e.Command.Template
						}
						cmdCopy.Name = displayName

						recentEntries = append(recentEntries, index.Entry{
							Group: config.Group{
								Name:        "recent",
								Description: "recently used commands",
							},
							Command: cmdCopy,
						})
						seen[key] = true
						break
					}
				}
				if len(recentEntries) >= 5 {
					break
				}
			}
			if len(recentEntries) > 0 {
				m.groups = append([]group{{name: "recent", entries: recentEntries}}, m.groups...)
			}
		}
	}

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

// buildGroups buckets entries by category/group, ordering groups by their first
// appearance in the (already ranked) results so grouping survives filtering.
func buildGroups(results []index.Entry) []group {
	var groups []group
	idx := make(map[string]int, len(results))
	for _, e := range results {
		name := e.Group.Name
		gi, ok := idx[name]
		if !ok {
			groups = append(groups, group{name: name})
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
	if m.showHelp {
		var b strings.Builder
		b.WriteString("\n" + m.topBar(m.headerContext()) + "\n\n")
		b.WriteString(m.logoIcon() + "\n\n")
		b.WriteString(m.viewHelpModal() + "\n\n")
		b.WriteString(renderHints([]keyHint{{"esc / ? / q", "close help"}}))
		return b.String()
	}

	var b strings.Builder
	b.WriteString("\n" + m.topBar(m.headerContext()) + "\n\n")
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

	if m.statusMsg != "" {
		b.WriteString("\n  " + styleOk.Render(m.statusMsg) + "\n")
	}

	b.WriteString("\n" + renderHints([]keyHint{
		{"↑/↓", "navigate"}, {"enter", "select"}, {"ctrl+r", "quick run"}, {"ctrl+o", "copy"}, {"?", "help"}, {"q", "quit"},
	}))
	return b.String()
}

func (m model) viewHelpModal() string {
	boxWidth := min(m.w()-6, 68)
	helpStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cBrand).
		Padding(1, 2).
		MarginLeft(2).
		Width(boxWidth)

	var h strings.Builder
	h.WriteString(styleSelName.Render("PALET SHORTCUTS & HELP") + "\n\n")

	type section struct {
		title string
		keys  []keyHint
	}

	sections := []section{
		{
			title: "Search & Navigation",
			keys: []keyHint{
				{"↑/↓, ctrl+p/n", "Navigate commands / flags"},
				{"enter", "Open command or execute"},
				{"ctrl+r", "Quick run (run immediately with defaults)"},
				{"ctrl+o", "Copy command to clipboard"},
				{"#tag", "Filter by tag (e.g. #docker, #service)"},
				{"esc", "Clear search or go back"},
			},
		},
		{
			title: "Arguments & Flags Form",
			keys: []keyHint{
				{"tab", "Next field / switch to flags"},
				{"shift+tab", "Previous field"},
				{"ctrl+n / ctrl+p", "Cycle options for choices / enums"},
				{"space", "Toggle flag [✓] / [ ] or bool"},
				{"enter", "Execute command with selected flags"},
			},
		},
	}

	for _, s := range sections {
		h.WriteString(styleGroupName.Render(s.title) + "\n")
		for _, k := range s.keys {
			h.WriteString("  " + styleKeyCap.Render(k.key) + "  " + styleKeyLbl.Render(k.label) + "\n")
		}
		h.WriteString("\n")
	}

	h.WriteString(styleDim.Render("Press 'esc', '?', or 'q' to close this guide."))
	return helpStyle.Render(h.String())
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
			styleGroupName.Render(strings.ToUpper(g.name)) + "  " +
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
	const nameW = 20
	name := e.Command.Name
	if e.Group.Description == "system command" &&
		e.Command.Name != e.Group.Name &&
		!strings.HasPrefix(e.Command.Name, e.Group.Name) {
		name = e.Group.Name + " " + e.Command.Name
	}
	desc := e.Command.Description

	q := m.searchInput.Value()
	nameIndices := search.MatchIndices(name, q)
	descIndices := search.MatchIndices(desc, q)

	tags := e.AllTags()
	var tagStr string
	if len(tags) > 0 {
		var tagPills []string
		for _, t := range tags {
			tagPills = append(tagPills, "#"+t)
		}
		tagStr = strings.Join(tagPills, " ")
	}
	tagIndices := search.MatchIndices(tagStr, q)

	if selected {
		nameRendered := highlight(name, nameIndices, styleSelName.Background(cSelBg), styleSelMatch)
		nameRendered = pad(" "+nameRendered, nameW+1)
		inner := nameRendered
		if desc != "" {
			descRendered := highlight(desc, descIndices, lipgloss.NewStyle().Foreground(cSubtle).Background(cSelBg), styleSelMatch)
			inner += "  " + descRendered
		}
		if tagStr != "" {
			tagRendered := highlight(tagStr, tagIndices, lipgloss.NewStyle().Foreground(cBrand2).Background(cSelBg), styleSelMatch)
			inner += "  " + tagRendered
		}
		row := lipgloss.NewStyle().Background(cSelBg).Width(max(4, m.w()-5)).Render(inner)
		return styleSelBar.Render("▌") + row
	}

	nameRendered := highlight(name, nameIndices, styleCmd, styleMatch)
	nameRendered = pad(nameRendered, nameW)
	line := "   " + nameRendered
	if desc != "" {
		descRendered := highlight(desc, descIndices, styleDesc, styleMatch)
		line += "  " + descRendered
	}
	if tagStr != "" {
		tagRendered := highlight(tagStr, tagIndices, styleTag, styleMatch)
		line += "  " + tagRendered
	}
	return line
}

func highlight(text string, indices []int, baseStyle, matchStyle lipgloss.Style) string {
	if len(indices) == 0 {
		return baseStyle.Render(text)
	}
	matchMap := make(map[int]bool, len(indices))
	for _, idx := range indices {
		matchMap[idx] = true
	}

	runes := []rune(text)
	var b strings.Builder
	var curRunes []rune
	isMatching := false

	flush := func() {
		if len(curRunes) == 0 {
			return
		}
		str := string(curRunes)
		if isMatching {
			b.WriteString(matchStyle.Render(str))
		} else {
			b.WriteString(baseStyle.Render(str))
		}
		curRunes = curRunes[:0]
	}

	for i, r := range runes {
		match := matchMap[i]
		if match != isMatching {
			flush()
			isMatching = match
		}
		curRunes = append(curRunes, r)
	}
	flush()

	return b.String()
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

func (m *model) openForm(e index.Entry) tea.Cmd {
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

	cmdLookup := e.Command.Template
	if cmdLookup == "" {
		cmdLookup = e.Command.Name
	}

	var cmd tea.Cmd
	m.flagsLoading = false
	m.availableFlags = e.Command.Flags
	if len(m.availableFlags) == 0 {
		if cached, ok := index.GetCachedCommandFlags(cmdLookup); ok {
			m.availableFlags = cached
		} else {
			m.flagsLoading = true
			cmd = loadFlagsCmd(cmdLookup)
		}
	}
	m.selectedFlags = make(map[string]bool)
	m.flagCursor = 0

	fi := textinput.New()
	fi.Prompt = "⚑  "
	fi.Placeholder = "Filter flags (e.g. -d, build)… [Space to toggle]"
	fi.PromptStyle = lipgloss.NewStyle().Foreground(cBrand)
	fi.PlaceholderStyle = styleDim
	fi.TextStyle = lipgloss.NewStyle().Foreground(cText)
	fi.Cursor.Style = lipgloss.NewStyle().Foreground(cBrand)
	fi.Width = max(10, m.w()-14)
	m.flagFilter = fi

	if len(m.inputs) == 0 && len(m.availableFlags) > 0 {
		m.formSection = 1
		m.flagFilter.Focus()
	} else {
		m.formSection = 0
		if len(m.inputs) > 0 {
			m.inputs[0].Focus()
		}
		m.flagFilter.Blur()
	}

	m.state = viewForm
	return cmd
}

func placeholderFor(arg config.Arg) string {
	if len(arg.Choices) > 0 {
		return strings.Join(arg.Choices, " | ")
	}
	if arg.Description != "" {
		return arg.Description
	}
	return arg.Name
}

func (m model) filteredFlags() []config.Flag {
	q := strings.ToLower(strings.TrimSpace(m.flagFilter.Value()))
	if q == "" {
		return m.availableFlags
	}
	var out []config.Flag
	for _, f := range m.availableFlags {
		if strings.Contains(strings.ToLower(f.Name), q) ||
			strings.Contains(strings.ToLower(f.Value()), q) ||
			strings.Contains(strings.ToLower(f.Description), q) {
			out = append(out, f)
		}
	}
	return out
}

func (m model) currentPreviewCmd() string {
	vals := make(map[string]string, len(m.inputs))
	for i, arg := range m.selected.Command.Args {
		vals[arg.Name] = strings.TrimSpace(m.inputs[i].Value())
	}
	cmd, _ := executor.BuildCommand(m.selected.Command.Template, vals)
	if cmd == "" {
		cmd = m.selected.Command.Template
		if cmd == "" {
			cmd = m.selected.Command.Name
		}
	}

	var activeFlags []string
	for _, f := range m.availableFlags {
		v := f.Value()
		if m.selectedFlags[v] {
			activeFlags = append(activeFlags, v)
		}
	}
	if len(activeFlags) > 0 {
		cmd += " " + strings.Join(activeFlags, " ")
	}
	return cmd
}

func (m model) updateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "ctrl+c":
		return m, tea.Quit

	case "esc":
		m.state = viewList
		return m, nil

	case "tab":
		if m.formSection == 0 {
			if m.focus < len(m.inputs)-1 {
				m.focusField(m.focus + 1)
				return m, nil
			}
			if len(m.availableFlags) > 0 {
				m.formSection = 1
				m.blurAllArgs()
				m.flagFilter.Focus()
				return m, nil
			}
		} else {
			if len(m.inputs) > 0 {
				m.formSection = 0
				m.flagFilter.Blur()
				m.focusField(0)
				return m, nil
			}
		}
		return m, nil

	case "shift+tab":
		if m.formSection == 1 {
			if len(m.inputs) > 0 {
				m.formSection = 0
				m.flagFilter.Blur()
				m.focusField(len(m.inputs) - 1)
				return m, nil
			}
		} else {
			if m.focus > 0 {
				m.focusField(m.focus - 1)
				return m, nil
			}
		}
		return m, nil

	case "enter":
		if m.formSection == 0 && m.focus < len(m.inputs)-1 {
			m.focusField(m.focus + 1)
			return m, nil
		}
		return m.submitForm()
	}

	if m.formSection == 1 {
		ff := m.filteredFlags()
		switch key {
		case "up":
			if m.flagCursor > 0 {
				m.flagCursor--
			} else if len(m.inputs) > 0 {
				m.formSection = 0
				m.flagFilter.Blur()
				m.focusField(len(m.inputs) - 1)
			}
			return m, nil

		case "down":
			if m.flagCursor < len(ff)-1 {
				m.flagCursor++
			}
			return m, nil

		case " ":
			if len(ff) > 0 && m.flagCursor >= 0 && m.flagCursor < len(ff) {
				val := ff[m.flagCursor].Value()
				m.selectedFlags[val] = !m.selectedFlags[val]
				return m, nil
			}
		}

		prev := m.flagFilter.Value()
		var cmd tea.Cmd
		m.flagFilter, cmd = m.flagFilter.Update(msg)
		if m.flagFilter.Value() != prev {
			m.flagCursor = 0
		}
		return m, cmd
	}

	// Args section
	switch key {
	case "up":
		m.focusField(m.focus - 1)
		return m, nil

	case "down":
		if m.focus < len(m.inputs)-1 {
			m.focusField(m.focus + 1)
		} else if len(m.availableFlags) > 0 {
			m.formSection = 1
			m.blurAllArgs()
			m.flagFilter.Focus()
		}
		return m, nil

	case "ctrl+n", "ctrl+p", "ctrl+l", "ctrl+h":
		if len(m.inputs) > 0 && m.focus >= 0 && m.focus < len(m.selected.Command.Args) {
			arg := m.selected.Command.Args[m.focus]
			choices := arg.Choices
			if len(choices) == 0 && (arg.Type == "bool" || strings.ToLower(arg.Name) == "bool") {
				choices = []string{"true", "false"}
			}
			if len(choices) > 0 {
				cur := strings.TrimSpace(m.inputs[m.focus].Value())
				idx := -1
				for ci, c := range choices {
					if c == cur {
						idx = ci
						break
					}
				}
				next := 0
				if key == "ctrl+p" || key == "ctrl+h" {
					if idx > 0 {
						next = idx - 1
					} else {
						next = len(choices) - 1
					}
				} else {
					if idx >= 0 {
						next = (idx + 1) % len(choices)
					}
				}
				m.inputs[m.focus].SetValue(choices[next])
				return m, nil
			}
		}

	case " ":
		if len(m.inputs) > 0 && m.focus >= 0 && m.focus < len(m.selected.Command.Args) {
			arg := m.selected.Command.Args[m.focus]
			if arg.Type == "bool" || strings.ToLower(arg.Name) == "bool" {
				cur := strings.TrimSpace(m.inputs[m.focus].Value())
				if cur == "true" {
					m.inputs[m.focus].SetValue("false")
				} else {
					m.inputs[m.focus].SetValue("true")
				}
				return m, nil
			}
		}
	}

	if len(m.inputs) > 0 && m.focus >= 0 && m.focus < len(m.inputs) {
		var cmd tea.Cmd
		m.inputs[m.focus], cmd = m.inputs[m.focus].Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *model) blurAllArgs() {
	for i := range m.inputs {
		m.inputs[i].Blur()
	}
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
	if err == nil {
		if cmd == "" {
			cmd = m.selected.Command.Template
			if cmd == "" {
				cmd = m.selected.Command.Name
			}
		}
		var activeFlags []string
		for _, f := range m.availableFlags {
			v := f.Value()
			if m.selectedFlags[v] {
				activeFlags = append(activeFlags, v)
			}
		}
		if len(activeFlags) > 0 {
			cmd += " " + strings.Join(activeFlags, " ")
		}
	}

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

	b.WriteString("\n" + m.topBar(m.headerContext()) + "\n\n")
	b.WriteString(m.logoIcon() + "\n\n")

	// Breadcrumb: group › command, with description & tags
	b.WriteString("  " + stylePlugin.Render(m.selected.Group.Name) +
		styleDim.Render("  ›  ") + styleCmd.Render(cmd.Name) + "\n")
	if cmd.Description != "" {
		b.WriteString("  " + styleDesc.Render(cmd.Description) + "\n")
	}
	if tags := m.selected.AllTags(); len(tags) > 0 {
		var tagBadges []string
		for _, t := range tags {
			tagBadges = append(tagBadges, badge("#"+t, cBlue, cFainter))
		}
		b.WriteString("  " + strings.Join(tagBadges, " ") + "\n")
	}
	b.WriteString("\n")

	// Arguments section
	for i, arg := range cmd.Args {
		focused := m.formSection == 0 && i == m.focus

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

		choices := arg.Choices
		if len(choices) == 0 && (arg.Type == "bool" || strings.ToLower(arg.Name) == "bool") {
			choices = []string{"true", "false"}
		}
		if len(choices) > 0 {
			var choicePills []string
			cur := strings.TrimSpace(m.inputs[i].Value())
			for _, ch := range choices {
				if ch == cur {
					choicePills = append(choicePills, badge("✓ "+ch, cText, cBrand))
				} else {
					choicePills = append(choicePills, badge(ch, cSubtle, cFainter))
				}
			}
			b.WriteString("    " + styleDim.Render("options (ctrl+n/p): ") + strings.Join(choicePills, " ") + "\n")
		}
		b.WriteString("\n")
	}

	// Flags & Parameters section
	if m.flagsLoading {
		b.WriteString("  " + styleSectionLabel.Render("PARAMETERS & FLAGS") + "\n")
		b.WriteString("    " + styleDim.Render("⚡ discovering flags in background…") + "\n\n")
	} else if len(m.availableFlags) > 0 {
		flagFocused := m.formSection == 1
		headerStyle := styleSectionLabel
		if flagFocused {
			headerStyle = styleSelName
		}
		b.WriteString("  " + headerStyle.Render("PARAMETERS & FLAGS") +
			styleDim.Render(" (Space to toggle, ↑/↓ to navigate)") + "\n")

		fbd := cCardBd
		if flagFocused {
			fbd = cBrand
		}
		fBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(fbd).
			Padding(0, 1).
			MarginLeft(2).
			Width(max(10, m.w()-8)).
			Render(m.flagFilter.View())
		b.WriteString(fBox + "\n")

		ff := m.filteredFlags()
		if len(ff) == 0 {
			b.WriteString("    " + styleDim.Render("no flags match filter") + "\n")
		} else {
			maxVisible := 6
			startIdx := 0
			if m.flagCursor >= maxVisible {
				startIdx = m.flagCursor - maxVisible + 1
			}
			endIdx := min(len(ff), startIdx+maxVisible)

			for fi := startIdx; fi < endIdx; fi++ {
				f := ff[fi]
				isCursor := flagFocused && fi == m.flagCursor
				isChecked := m.selectedFlags[f.Value()]

				checkMark := "[ ]"
				checkStyle := styleDim
				if isChecked {
					checkMark = "[✓]"
					checkStyle = styleOk
				}

				cursorMark := "  "
				rowStyle := styleMuted
				if isCursor {
					cursorMark = styleSelBar.Render("▌ ")
					rowStyle = styleSelName
				}

				flagQ := strings.TrimSpace(m.flagFilter.Value())
				fIndices := search.MatchIndices(f.Name, flagQ)
				flagMatchStyle := styleMatch
				if isCursor {
					flagMatchStyle = styleSelMatch
				}
				flagLabel := highlight(f.Name, fIndices, rowStyle, flagMatchStyle)
				descLabel := ""
				if f.Description != "" {
					descLabel = "  " + styleDesc.Render(f.Description)
				}

				b.WriteString(cursorMark + checkStyle.Render(checkMark) + " " + flagLabel + descLabel + "\n")
			}

			if len(ff) > maxVisible {
				b.WriteString("    " + styleDim.Render(fmt.Sprintf("(showing %d of %d flags)", endIdx-startIdx, len(ff))) + "\n")
			}
		}
		b.WriteString("\n")
	}

	if m.formErr != "" {
		b.WriteString("  " + badge("!", cText, cRed) + " " + styleError.Render(m.formErr) + "\n\n")
	}

	// Live Command Preview
	previewCmd := m.currentPreviewCmd()
	previewBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cCardBd).
		Padding(0, 1).
		MarginLeft(2).
		Width(max(10, m.w()-8)).
		Render(lipgloss.NewStyle().Foreground(cGreen).Render("$ ") + styleCmd.Render(previewCmd))

	b.WriteString("  " + styleSectionLabel.Render("PREVIEW") + "\n")
	b.WriteString(previewBox + "\n\n")

	b.WriteString(renderHints([]keyHint{
		{"tab", "switch section"}, {"ctrl+n/p", "cycle options"}, {"space", "toggle flag/bool"}, {"enter", "execute"}, {"esc", "back"},
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
		if m.formSection == 1 {
			m.flagFilter, cmd = m.flagFilter.Update(msg)
		} else if len(m.inputs) > 0 && m.focus >= 0 && m.focus < len(m.inputs) {
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

// headerContext returns the cwd and detected project summary (e.g. "~/OSS/palet-win  •  Go (palet)").
func (m model) headerContext() string {
	lbl := cwdLabel()
	if m.projectInfo.Summary != "" {
		if lbl != "" {
			return lbl + "  •  " + m.projectInfo.Summary
		}
		return m.projectInfo.Summary
	}
	return lbl
}
