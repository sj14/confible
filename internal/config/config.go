package config

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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
		fileFlags := os.O_CREATE
		if cfg.Truncate {
			fileFlags = os.O_CREATE | os.O_TRUNC
		}

		// create folder for the target file if it doesn't exist
		if err := os.MkdirAll(filepath.Dir(cfg.Path), 0o755); err != nil {
			return fmt.Errorf("failed creating target folder (%v): %v", cfg.Path, err)
		}

		// open the target file (doesn't create the folder when it doesn't exit)
		targetFile, err := os.OpenFile(cfg.Path, fileFlags, 0o666)
		if err != nil {
			return fmt.Errorf("failed reading target file (%v): %v", cfg.Path, err)
		}
		defer targetFile.Close()

		// process new file content
		var newContent string
		switch mode {
		case ModeNormal:
			newContent, err = modifyContent(targetFile, confibleFile.Priority, confibleFile.ID, cfg.Comment, cfg.Append, td, time.Now())
			if err != nil {
				return fmt.Errorf("failed appending new content: %w", err)
			}
		case ModeCleanID, ModeCleanAll:
			var existingConfigs []confibleConfig
			newContent, existingConfigs, err = fileContent(targetFile)
			if err != nil {
				return fmt.Errorf("failed cleaning id config: %w", err)
			}

			// ModeCleanAll: we are done, just newContent is enough

			if mode == ModeCleanID {
				for _, existingCfg := range existingConfigs {
					// we want to clean this config
					if existingCfg.id == confibleFile.ID {
						continue
					}
					// but append all other configs
					newContent = newContent + "\n" + existingCfg.content
				}
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

type confibleConfig struct {
	id       string
	priority int64
	content  string
}

func fileContent(reader io.Reader) (string, []confibleConfig, error) {
	content := strings.Builder{}
	existingConfigs := []confibleConfig{}
	existingConfig := confibleConfig{}
	// existingConfigBuilder := strings.Builder{}

	// read the already existing file content
	scanner := bufio.NewScanner(reader)
	processingAnExistingConfig := false
	for scanner.Scan() {
		// we reached our/any own old confible config, do not copy our old config
		if strings.Contains(scanner.Text(), header) {
			existingConfig.id = extractID(scanner.Text())
			existingConfig.priority = extractPriority(scanner.Text())
			existingConfig.content = existingConfig.content + "\n\n"

			// some cleaning when we multiple confible configs write to the same file
			// otherwise each execution would increase the blank lines
			cacheNewContent := strings.TrimSpace(content.String())
			content.Reset()
			content.WriteString(cacheNewContent + "\n\n")
			processingAnExistingConfig = true
		}

		// our/any own old confible config was read, continue copying the other content
		if strings.Contains(scanner.Text(), footer) {
			processingAnExistingConfig = false
			existingConfig.content = existingConfig.content + scanner.Text() + "\n\n\n"
			existingConfigs = append(existingConfigs, existingConfig)
			existingConfig = confibleConfig{}
			continue
		}

		if processingAnExistingConfig {
			existingConfig.content = existingConfig.content + scanner.Text() + "\n"
			continue
		}

		if _, err := content.Write(append(scanner.Bytes(), '\n')); err != nil {
			return "", existingConfigs, err
		}
	}
	if err := scanner.Err(); err != nil {
		return "", existingConfigs, err
	}

	return strings.TrimSpace(content.String()), existingConfigs, nil
}

func generateHeaderWithID(id string) string {
	return fmt.Sprintf(header+" id: %q", id)
}

func generateHeaderWithIDAndPriority(id string, priority int64) string {
	return fmt.Sprintf(generateHeaderWithID(id)+" priority: \"%v\"", priority)
}

func extractX(s, startString string) string {
	idxStart := strings.Index(s, startString)
	if idxStart == -1 {
		return ""
	}
	start := s[idxStart:]
	start = strings.TrimPrefix(start, startString)
	idxEnd := strings.Index(start, "\"")
	if idxEnd == -1 {
		return ""
	}
	return start[:idxEnd]
}

func extractID(s string) string {
	return extractX(s, "id: \"")
}

var DefaultPriority int64 = 1000

func extractPriority(s string) int64 {
	priorityStr := extractX(s, "priority: \"")
	if priorityStr == "" {
		return DefaultPriority
	}

	priority, err := strconv.ParseInt(priorityStr, 10, 64)
	if err != nil {
		log.Fatalf("failed extracting priority from %q\n", s)
	}
	return priority
}

func generateFooterWithID(id string) string {
	return fmt.Sprintf(footer+" id: %q", id)
}

type TemplateData struct {
	Env map[string]string
	Var map[string]string
}

func modifyContent(reader io.Reader, priority int64, id, comment, appendText string, td TemplateData, now time.Time) (string, error) {
	oldC, oldConfigs, err := fileContent(reader)
	if err != nil {
		return "", err
	}

	oldContent := strings.Builder{}
	oldContent.WriteString(oldC)

	var newContent = strings.Builder{}
	// if _, err = newContent.WriteString(oldContent.String()); err != nil {
	// 	return "", err
	// }

	// Add blank line before confible part (only when the target file is not empty)
	if strings.TrimSpace(newContent.String()) != "" {
		if _, err := newContent.WriteString("\n\n"); err != nil {
			return "", err
		}
	}

	// header
	if _, err := newContent.WriteString(comment + " ~~~ " + generateHeaderWithIDAndPriority(id, priority) + " ~~~\n" + comment + " " + now.Format(time.RFC1123) + "\n"); err != nil {
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

	newConfig := confibleConfig{
		id:       id,
		priority: priority,
		content:  newContent.String(),
	}

	newConfigs := []confibleConfig{}

	// delete old config with same id
	for _, cfg := range oldConfigs {
		if cfg.id == id {
			continue
		}
		newConfigs = append(newConfigs, cfg)
	}

	// append new config
	newConfigs = append(newConfigs, newConfig)

	// sort configs by priority
	sort.Slice(newConfigs, func(i, j int) bool {
		return newConfigs[i].priority < newConfigs[j].priority
	})

	// append configs to new content
	oldContent.WriteString("\n\n")
	for _, cfg := range newConfigs {
		oldContent.WriteString(strings.TrimSpace(cfg.content) + "\n\n")
	}

	return strings.TrimSpace(oldContent.String()), nil
}
