package config

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/sj14/confible/internal/confible"
	"github.com/sj14/confible/internal/utils"
	"github.com/sj14/confible/internal/variable"
)

const (
	header = "CONFIBLE START"
	footer = "CONFIBLE END"
)

// validate and aggregate configs which target the same file
func aggregateConfigs(configs []confible.Config) []confible.Config {
	// the key is the path of the config file
	configsMap := make(map[string]confible.Config)

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

		cfg.Path = utils.AbsFilepath(cfg.Path)

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

	var aggregated []confible.Config
	for _, cfg := range configsMap {
		aggregated = append(aggregated, cfg)
	}

	return aggregated
}

func ModifyTargetFiles(confibleFile confible.File, useCached bool, mode ContentMode) error {
	configs := aggregateConfigs(confibleFile.Configs)

	var td TemplateData

	if mode == ModeNormal {
		// only create template when we are not in a clean mode
		variableMap, err := variable.Parse(confibleFile.ID, confibleFile.Variables, useCached)
		if err != nil {
			return err
		}

		td = TemplateData{
			Env: utils.GetEnvMap(),
			Var: variableMap,
		}
	}

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
		case ModeNormal:
			newContent, err = modifyContent(targetFile, confibleFile.ID, cfg.Comment, cfg.Append, td, time.Now())
			if err != nil {
				return fmt.Errorf("failed appending new content: %w", err)
			}
		case ModeCleanID:
			newContent, err = fileContent(targetFile, confibleFile.ID)
			if err != nil {
				return fmt.Errorf("failed cleaning id config: %w", err)
			}
		case ModeCleanAll:
			newContent, err = fileContent(targetFile, "")
			if err != nil {
				return fmt.Errorf("failed cleaning all config: %w", err)
			}
		default:
			return fmt.Errorf("wrong or no mode specified")
		}

		// write content to the file
		if err := os.WriteFile(cfg.Path, []byte(newContent), os.ModePerm); err != nil {
			return fmt.Errorf("failed writing target file (%v): %v", cfg.Path, err)
		}
		log.Printf("[%v] wrote config to %q\n", confibleFile.ID, cfg.Path)
	}
	return nil
}

type ContentMode uint8

const (
	ModeNormal ContentMode = iota
	ModeCleanID
	ModeCleanAll
)

func fileContent(reader io.Reader, id string) (string, error) {
	content := strings.Builder{}

	// read the already existing file content
	scanner := bufio.NewScanner(reader)
	skip := false
	for scanner.Scan() {
		lookForStart := header
		if id != "" {
			lookForStart = generateHeaderWithID(id)
		}
		// we reached our/any own old confible config, do not copy our old config
		if strings.Contains(scanner.Text(), lookForStart) {
			// some cleaning when we multiple confible configs write to the same file
			// otherwise each execution would increase the blank lines
			cacheNewContent := strings.TrimSpace(content.String())
			content.Reset()
			content.WriteString(cacheNewContent + "\n\n")
			skip = true
		}

		lookForEnd := footer
		if id != "" {
			lookForEnd = generateFooterWithID(id)
		}
		// our/any own old confible config was read, continue copying the other content
		if strings.Contains(scanner.Text(), lookForEnd) {
			skip = false
			continue
		}

		if skip {
			continue
		}

		if _, err := content.Write(append(scanner.Bytes(), '\n')); err != nil {
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

type TemplateData struct {
	Env map[string]string
	Var map[string]string
}

func modifyContent(reader io.Reader, id, comment, appendText string, td TemplateData, now time.Time) (string, error) {
	oldContent, err := fileContent(reader, id)
	if err != nil {
		return "", err
	}

	var newContent = strings.Builder{}
	if _, err = newContent.WriteString(oldContent); err != nil {
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
