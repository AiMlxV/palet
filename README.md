# PALET
A terminal command palette. Search commands, fill in args, and run — no more memorising flags.

[![asciicast](https://asciinema.org/a/Ucqx87fnBLxOBOLv.svg)](https://asciinema.org/a/Ucqx87fnBLxOBOLv)

## Install
Build from source (Linux / Debian / Ubuntu):

```bash
git clone https://github.com/AiMlxV/palet
cd palet
go build -o palet ./cmd
```

Then make sure `$GOPATH/bin` or your binary directory is in your PATH:

```bash
# Optional: install to your PATH
sudo cp palet /usr/local/bin/
# Or export to PATH in ~/.bashrc or ~/.zshrc
export PATH="$PATH:$(pwd)"
```

## Usage
Run `palet` to open the command palette:

```bash
# Open interactive palette
palet

# Filter directly by tag
palet --tag debian
palet -t service

# Start with an initial query
palet docker
```

### Search & Commands
- Type keywords to search across command names, descriptions, and tags.
- Type `#` to list all tagged commands, or `#<tag>` / `tag:<tag>` (e.g. `#docker`, `#service`) to filter.

### Parameters & Flags Selection
When selecting a command (e.g. `docker compose up`):
- All supported flags and parameters (e.g. `-d, --detach`, `--build`, `--force-recreate`) are shown.
- Filter flags in real time by typing into the flag search box.
- Press **`space`** on any flag to toggle `[✓]` it on or off.
- The **Live Preview** bar updates dynamically as flags are toggled.

| Key | Action |
|-----|--------|
| type | search commands or filter flags |
| `↑` `↓` or `ctrl+p` `ctrl+n` | navigate commands or flags |
| `enter` | select command / execute command |
| `ctrl+r` | quick run (execute immediately with defaults) |
| `ctrl+o` | copy command to clipboard |
| `?` or `F1` | toggle help & shortcuts cheat sheet |
| `space` | toggle flag on / off (`[✓]` / `[ ]`) or boolean arg |
| `ctrl+n` `ctrl+p` | cycle argument options (for choices/enums) |
| `tab` | switch between arguments and flags section |
| `shift+tab` | previous section / field |
| `esc` | clear query / go back |
| `q` | quit (when search is empty) |

### Automated Command Mapping
Palet automatically indexes all system commands, binaries in `$PATH`, subcommands (such as `docker`, `docker compose`, `git`, `systemctl`, `apt`), and their supported flags directly from your system without requiring manual plugin setup.

### Custom Config (Optional)
You can define your own project or personal shortcuts in `~/.config/palet/config.yaml` or `./palet.yaml`:

```yaml
name: my-scripts
commands:
  - name: deploy
    description: Deploy service to staging or prod
    template: "./deploy.sh --env {{.env}}"
    args:
      - name: env
        description: Environment target
        type: string
        default: "staging"
    flags:
      - name: "--force"
        description: "Force deploy without confirmation"
```

## License
[Apache-2.0 license](https://github.com/Thitipong-PP/palet/blob/main/LICENSE)