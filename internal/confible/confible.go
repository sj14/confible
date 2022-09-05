package confible

type File struct {
	ID        string     `toml:"id"`
	Priority  int64      `toml:"priority"`
	Configs   []Config   `toml:"config"`
	Commands  []Command  `toml:"commands"`
	Variables []Variable `toml:"variables"`
}

type Config struct {
	Path     string `toml:"path"`
	Truncate bool   `toml:"truncate"`
	Comment  string `toml:"comment_symbol"`
	Append   string `toml:"append"`
}

type Command struct {
	Exec []string `toml:"exec"`
}

type Variable struct {
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
