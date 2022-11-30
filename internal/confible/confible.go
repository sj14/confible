package confible

import "os"

type File struct {
	Settings  Settings   `toml:"settings"`
	Configs   []Config   `toml:"config"`
	Commands  []Command  `toml:"commands"`
	Variables []Variable `toml:"variables"`
}

type Settings struct {
	Deactivated bool     `toml:"deactivated"`
	ID          string   `toml:"id"`
	OSs         []string `toml:"os"`
	Archs       []string `toml:"arch"`
}

type Config struct {
	OSs      []string    `toml:"os"`
	Archs    []string    `toml:"arch"`
	Priority int64       `toml:"priority"`
	Path     string      `toml:"path"`
	Truncate bool        `toml:"truncate"`
	PermDir  os.FileMode `toml:"perm_dir"`
	PermFile os.FileMode `toml:"perm_file"`
	Comment  string      `toml:"comment_symbol"`
	Append   string      `toml:"append"`
}

type Command struct {
	OSs          []string `toml:"os"`
	Archs        []string `toml:"arch"`
	AfterConfigs bool     `toml:"after_configs"`
	Exec         []string `toml:"exec"`
}

type Variable struct {
	OSs   []string `toml:"os"`
	Archs []string `toml:"arch"`
	Exec  []VarCmd `toml:"exec"`
	Input []VarVal `toml:"input"`
}

type VarVal struct {
	VariableName string `toml:"var"`
	Prompt       string `toml:"prompt"`
}

type VarCmd struct {
	VariableName string `toml:"var"`
	Cmd          string `toml:"cmd"`
}
