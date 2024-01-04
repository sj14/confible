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
  -apply-cfgs
        apply configs (default true)
  -apply-cmds
        exec commands (default true)
  -cache-clean
        remove the cache for the given configs
  -cache-file string
        custom path to the cache file
  -cache-list
        list the cached variables
  -cache-prune
        remove the cache file used for all configs
  -cached-cmds
        don't execute commands when they didn't change since last execution (default true)
  -cached-vars
        use the variables from the cache when present (default true)
  -clean
        give a confible file and it will remove the config from configured targets matching the config id
  -version
        print version information
```

## Example

```toml
[settings]
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

```vim
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

Check my personal config [repository](https://github.com/sj14/dotfiles/tree/a752fbc88031bc99b59b5d24fe342dcdafdac750/confible) for more examples.

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
[settings]
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


## Config Reference

```toml
[settings]
# the ID allows to execute different configs to the same path
id = "some unique identifier"
# When deactivated is set to 'true', it won't appended the
# target configs but behave like the '-clean' flag is set,
# removing any configs from the targets with the given id.
deactivated = false
# Filter the operating system. Only when the machines OS matches, the file gets processed.
# When this is not set, the operating system doesn't matter. Default: "[]" (optional)
# Possible values ($GOOS): https://go.dev/doc/install/source#environment
os = ["darwin", "linux"]
# Filter the machine architecture. Only when the architecture matches, the file gets processed.
# When this is not set, the architecture doesn't matter. Default: "[]" (optional)
# Possible values ($GOARCH): https://go.dev/doc/install/source#environment
arch = ["amd64", "arm64"]


[[commands]]
# Same as settings.os but on the command level.
os = ["darwin", "linux"]
# Same as settings.arch but on the command level.
arch = ["amd64", "arm64"]
# Run the commands before writing the configs. Default: "false" (optional).
# Set to "true" to run the commands after the configs were written. 
after_configs = false 
exec = [
    "echo yo", 
    "echo yoyo",
]


[[config]]
# Same as settings.os but on the config level.
os = ["darwin", "linux"]
# Same as settings.arch but on the config level.
arch = ["amd64", "arm64"]
# The position of the config written to the target.
# Lower values are sorted before other confible parts. Default: "1000" (optional)
priority = 1000
path = "path/to/target"
# Enable truncate for erasing target file before writing/updating. 
# If the '-clean' flag is used, the target file will be completely removed.
# Default: "false" (optional).
truncate = false
# When any directories need to be created to store the config at the given path,
# the given permissions will be set for those directories. Default: 0o700 (optional).
# A zero value (no permissions) will be ignored and the default will be used instead.
# It's not possible to set the permissions for already existing directories. For this
# use case, you might want to use [[commands]].
perm_dir = 0o700
# The given permissions will be set for config. Default: 0o644 (optional).
# A zero value (no permissions) will be ignored and the default will be used instead.
perm_file = 0o644
# Symbol which is recognized as a comment by the target file.
comment_symbol = "//" 
append = """
what you want to add
"""


# variables which can be used in the [[config]] parts (see templating)
[[variables]]
# Same as settings.os but on the variables level.
os = ["darwin", "linux"]
# Same as settings.arch but on the variables level.
arch = ["amd64", "arm64"]
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
