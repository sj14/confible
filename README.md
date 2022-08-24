# Confible

Confible is a simple configuration tool for your local machine.

When configs are applied, a boundary header and footer are added which allows executing the configs multiple times or adjusting them and the target file will only contain the latest version of your desired configuration without the need of removing old modifications first.

## Installation

### Precompiled Binaries

See the [releases](https://github.com/sj14/confible/releases) page for precompiled binaries.

### Homebrew

Using the [Homebrew](https://brew.sh/) package manager for macOS:

```bash
brew install sj14/tap/confible
```

### Manually

It's also possible to install the latest release with `go install`:

```bash
go install github.com/sj14/confible
```

## Usage

```bash
confible [flags] <config.toml> [...]
```

```text
Usage of confible:
  -no-cfg
        do not apply any configs
  -no-cmd
        do not exec any commands
```

## Example

```toml
id = "some unique identifier"

[[commands]]
name = "test commands"
exec = [
    "echo hello", 
    "echo world",
]

[[config]]
name = "modify vimrc"
path = "~/.vimrc"
comment_symbol = "\""
append = """
set number
syntax on
set ruler
filetype indent plugin on
"""
```

Beside the `hello` and `world` outputs from the `[[commands]]` section, the `[[config]]` section will result into the below shown `.vimrc` file.  
Feel free to adjust the config and rerun `confible` for updating the `.vimrc` to the latest version.

```text
...
content not handled by confible
...

" ~~~ CONFIBLE START id: "some unique identifier" ~~~
" Wed, 10 Mar 2021 22:10:04 CET
set number
syntax on
set ruler
filetype indent plugin on
" ~~~ CONFIBLE END id: "some unique identifier" ~~~
```

## Config Specification

```toml
id = "some unique identifier" # the ID allows to execute different configs to the same path 

[[commands]]
name = "test commands"
exec = [
    "echo yo", 
    "echo yoyo",
]

[[config]]
name = "adjust my config" # optional
path = "file/to/target"
truncate = false      # enable for erasing target file beforehand (optional)
comment_symbol = "//" # symbol which is recognized as a comment by the target file
append = """
what you want to add
"""
```
