# PALET
A terminal command palette. Search commands, fill in args, and run — no more memorising flags.

[![asciicast](https://asciinema.org/a/WjfuteL3rRhP4yaK.svg)](https://asciinema.org/a/WjfuteL3rRhP4yaK)

## Install
build from source:

```bash
git clone https://github.com/AiMlxV/palet
cd palet
go build -o palet .
```

Then make sure `$GOPATH/bin` is in your PATH:

```bash
# Add to ~/.zshrc or ~/.bashrc
export PATH="$PATH:$(go env GOPATH)/bin"

# Then reload
source ~/.zshrc   # or source ~/.bashrc
```

Already done? just run `palet` directly.

## Usage
Run palet to open the command palette.
```bash
palet
```

| Key | Action |
|-----|--------|
| type | search commands |
| `↑` `↓` or `j` `k` | navigate |
| `enter` | select command / next arg field / execute command |
| `tab` | next arg field |
| `shift+tab` | previous arg field |
| `ctrl o` | copy to clipboard |
| `esc` | go back |
| `q` | quit |

## License
[Apache-2.0 license](https://github.com/Thitipong-PP/palet/blob/main/LICENSE)