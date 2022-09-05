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

```console
confible [flags] <config.toml> [...]
```

```text
 Usage of confible:
  -apply-cfgs
        apply configs (default true)
  -apply-cmds
        exec commands (default true)
  -cached-cmds
        don't execute commands when they didn't change since last execution (default true)
  -cached-vars
        use the variables from the cache when present (default true)
  -clean-all
        give a confible file and it will remove all configs from the targets
  -clean-cache
        remove the cache file
  -clean-id
        give a confible file and it will remove the config from configured targets matching the config id
  -version
        print version information
```

## Example

```toml
id = "vimrc"

[[commands]]
exec = [
    "echo hello", 
    "echo world",
]

[[config]]
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

" ~~~ CONFIBLE START id: "vimrc" ~~~
" Wed, 10 Mar 2021 22:10:04 CET
set number
syntax on
set ruler
filetype indent plugin on
" ~~~ CONFIBLE END id: "vimrc" ~~~
```

Check my personal config [repository](https://github.com/sj14/dotfiles) for more examples.

## Templates

You can include environment variables in configs using `{{ .Env.VARIABLE_NAME }}`.

```toml
[[config]]
path = "~/test.conf"
comment_symbol = "#"
append = """
My home dir is {{ .Env.HOME }}.
"""
```

## Variables

You can add variables to your configs.
Variables can be assigned by executing commands or based on manual inputs.
Executing the following example will wait for you to input your name and age.

```toml
id = "variables"

[[variables]]
input = [
    { var = "nick", prompt = "your nick name" },
    { var = "age", prompt = "your age in years" },
]
exec = [
    { var = "curDate", cmd = "date" },
    { var = "say", cmd = "echo 'Hello World!'" },
]

[[config]]
path = "~/test.conf"
comment_symbol = "#"
append = """
My Nick is {{ .Var.nick}}
I am {{ .Var.age}} years old
Today is {{ .Var.curDate }}
I want to say {{ .Var.say }}
"""
```


## Config Specification

```toml
id = "some unique identifier" # the ID allows to execute different configs to the same path 

priority = 1000 # the position of the config written to the target. Lower values are sorted before other confible parts. Default: 1000 (optional)

[[commands]]
exec = [
    "echo yo", 
    "echo yoyo",
]

[[config]]
path = "path/to/target"
truncate = false      # enable for erasing target file beforehand (optional)
comment_symbol = "//" # symbol which is recognized as a comment by the target file
append = """
what you want to add
"""

# variables which can be used in the [[config]] parts (see templating)
[[variables]]
# Variables which will create an input prompt.
# The first value is the variable name, the second value is the prompt message.
input = [ 
    { var = "nick", prompt = "your nick name" },
    { var = "age", prompt = "your age in years" },
]
# Variables where the command output is assigned.
# The first value is the variable name, the second value is the command to execute.
exec = [
    { var = "curDate", cmd = "date" },
    { var = "say", cmd = "echo 'Hello World!'" },
]
```
