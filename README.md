# Confible

Confible is a simple configuration tool for your local machine.

When configs are applied, a boundary header and footer are added which allows executing the configs multiple times or adjusting them and the target file will only contain the latest version of your desired configuration without the need of removing old modifications first.

## Usage

```bash
confible [flags] <config.toml>
```

```text
Usage of confible:
  -no-cfg
        do not apply any configs
  -no-cmd
        do not exec any commands
```

Example config

```toml
[[commands]]
name = "test commands"
exec = [
    "echo yo", 
    "echo yoyo",
]

[[config]]
path = "~/.vimrc"
comment_symbol = "#"
append = """
set number
syntax on
set ruler
filetype indent plugin on
"""
```

Beside the `yo` and `yoyo` outputs from the `[[commands]]` section, the `[[config]]` section will result into the below shown `zshrc` file.  
Feel free to adjust the config and rerun `confible` for updating the `zshrc` to the latest version.

```toml
...
content not handled by confible
...

# CONFIBLE START
set number
syntax on
set ruler
filetype indent plugin on

# CONFIBLE END
```