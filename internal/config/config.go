package config

import (
	"bufio"
	"fmt"
	"html/template"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	header = "CONFIBLE START"
	footer = "CONFIBLE END"
)

type Config struct {
	Path     string `toml:"path"`
	Truncate bool   `toml:"truncate"`
	Comment  string `toml:"comment_symbol"`
	Append   string `toml:"append"`
}

// validate and aggregate configs which target the same file
func aggregateConfigs(configs []Config) []Config {
	// the key is the path of the config file
	configsMap := make(map[string]Config)

	for _, cfg := range configs {
		if cfg.Append == "" {
			log.Fatalf("missing append\n")
		}
		if cfg.Path == "" {
			log.Fatalf("missing target\n")
		}
		if cfg.Comment == "" {
			log.Fatalf("missing comment symbol\n")
		}

		if strings.HasPrefix(cfg.Path, "~") {
			home, err := os.UserHomeDir()
			if err != nil {
				log.Fatalf("failed getting home dir: %v\n", err)
			}
			cfg.Path = filepath.Join(home, cfg.Path[1:])
		}

		// add a new config path (no need for aggregating)
		if _, ok := configsMap[cfg.Path]; !ok {
			configsMap[cfg.Path] = cfg
			continue
		}

		// aggregate with existing config path
		old := configsMap[cfg.Path]
		if old.Comment != cfg.Comment {
			log.Printf("multiple comment styles for %q (%q and %q) using %q\n", cfg.Path, old.Comment, cfg.Comment, old.Comment)
		}
		if old.Truncate != cfg.Truncate {
			log.Fatalf("%q should be truncated and also not be truncated\n", cfg.Path)
		}

		old.Append += cfg.Append
		configsMap[cfg.Path] = old
	}

	var aggregated []Config
	for _, cfg := range configsMap {
		aggregated = append(aggregated, cfg)
	}

	return aggregated
}

const (
	ModeAppend uint8 = iota
	ModeClean
)

func ModifyTargetFiles(id string, configs []Config, variables stringMap, mode uint8) error {
	configs = aggregateConfigs(configs)

	for _, cfg := range configs {
		flag := os.O_CREATE
		if cfg.Truncate {
			flag = os.O_CREATE | os.O_TRUNC
		}

		// create folder for the target file if it doesn't exist
		if err := os.MkdirAll(filepath.Dir(cfg.Path), 0o755); err != nil {
			return fmt.Errorf("failed creating target folder (%v): %v", cfg.Path, err)
		}

		// open the target file (doesn't create the folder when it doesn't exit)
		targetFile, err := os.OpenFile(cfg.Path, flag, 0o666)
		if err != nil {
			return fmt.Errorf("failed reading target file (%v): %v", cfg.Path, err)
		}
		defer targetFile.Close()

		// process new file content
		var newContent string
		switch mode {
		case ModeAppend:
			newContent, err = modifyContent(targetFile, id, cfg.Comment, cfg.Append, variables, time.Now())
			if err != nil {
				return fmt.Errorf("failed appending new content: %w", err)
			}
		case ModeClean:
			newContent, err = fileContentWithoutConfiblePartOfID(targetFile, id)
			if err != nil {
				return fmt.Errorf("failed cleaning config: %w", err)
			}
		}

		// write content to the file
		if err := os.WriteFile(cfg.Path, []byte(newContent), os.ModePerm); err != nil {
			return fmt.Errorf("failed writing target file (%v): %v", cfg.Path, err)
		}
	}
	return nil
}

func fileContentWithoutConfiblePartOfID(reader io.Reader, id string) (string, error) {
	content := strings.Builder{}

	// read the already existing file content
	scanner := bufio.NewScanner(reader)
	skip := false
	for scanner.Scan() {
		// we reached our own old config, do not copy our old config
		if strings.Contains(scanner.Text(), generateHeaderWithID(id)) {
			skip = true
		}

		// our config was read, continue copying the other content
		if strings.Contains(scanner.Text(), generateFooterWithID(id)) {
			skip = false
			continue
		}

		if skip {
			continue
		}

		if _, err := content.Write(scanner.Bytes()); err != nil {
			return "", err
		}
		if err := content.WriteByte('\n'); err != nil {
			return "", err
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}

	return strings.TrimSpace(content.String()), nil
}

func generateHeaderWithID(id string) string {
	return fmt.Sprintf(header+" id: %q", id)
}

func generateFooterWithID(id string) string {
	return fmt.Sprintf(footer+" id: %q", id)
}

type stringMap map[string]string

type templateData struct {
	Env stringMap
	Var stringMap
}

func getEnvMap() stringMap {
	envMap := make(stringMap)

	for _, environ := range os.Environ() {
		keyValue := strings.SplitN(environ, "=", 2)
		envMap[keyValue[0]] = keyValue[1]
	}

	return envMap
}

func modifyContent(reader io.Reader, id, comment, appendText string, variables stringMap, now time.Time) (string, error) {
	content, err := fileContentWithoutConfiblePartOfID(reader, id)
	if err != nil {
		return "", err
	}

	var newContent = strings.Builder{}
	if _, err = newContent.WriteString(content); err != nil {
		return "", err
	}

	// Add blank line before confible part (only when the target file is not empty)
	if strings.TrimSpace(newContent.String()) != "" {
		if _, err := newContent.WriteString("\n\n"); err != nil {
			return "", err
		}
	}

	// header
	if _, err := newContent.WriteString(comment + " ~~~ " + generateHeaderWithID(id) + " ~~~\n" + comment + " " + now.Format(time.RFC1123) + "\n"); err != nil {
		return "", err
	}

	// template config
	td := templateData{
		Env: getEnvMap(),
		Var: variables,
	}

	templ, err := template.New("").Parse(strings.TrimSpace(appendText))
	if err != nil {
		return "", fmt.Errorf("failed parsing the template: %w", err)
	}

	err = templ.Execute(&newContent, td)
	if err != nil {
		return "", fmt.Errorf("failed executing the template: %w", err)
	}

	// footer
	if _, err := newContent.WriteString("\n" + comment + " ~~~ " + generateFooterWithID(id) + " ~~~\n"); err != nil {
		return "", err
	}

	return newContent.String(), nil
}
