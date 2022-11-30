package config

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/sj14/confible/internal/confible"
	"github.com/sj14/confible/internal/utils"
	"github.com/sj14/confible/internal/variable"
	"golang.org/x/exp/slices"
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
		if cfg.Priority == 0 {
			cfg.Priority = DefaultPriority
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
		if old.PermDir != cfg.PermDir {
			log.Fatalf("%q has perm_dir %v and perm_dir %v\n", cfg.Path, old.PermDir, cfg.PermDir)
		}
		if old.PermFile != cfg.PermFile {
			log.Fatalf("%q has perm_file %v and perm_file %v\n", cfg.Path, old.PermFile, cfg.PermFile)
		}
		if old.Priority != cfg.Priority {
			log.Fatalf("%q has priority %v and priority %v\n", cfg.Path, old.Priority, cfg.Priority)
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

func ModifyTargetFiles(confibleFile confible.File, useCached bool, cacheFilepath string, mode ContentMode) error {
	configs := aggregateConfigs(confibleFile.Configs)

	var td TemplateData

	if mode == ModeNormal {
		// only create template when we are not in a clean mode
		variableMap, err := variable.Parse(confibleFile.Settings.ID, confibleFile.Variables, useCached, cacheFilepath)
		if err != nil {
			return err
		}

		td = TemplateData{
			Env: utils.GetEnvMap(),
			Var: variableMap,
		}
	}

	for _, cfg := range configs {
		// check if we can skip this config
		if len(cfg.OSs) != 0 && !slices.Contains(cfg.OSs, runtime.GOOS) {
			log.Printf("[%v] skipping as operating system %q is not matching config filter %q\n", confibleFile.Settings.ID, runtime.GOOS, cfg.OSs)
			continue
		}
		if len(cfg.Archs) != 0 && !slices.Contains(cfg.Archs, runtime.GOARCH) {
			log.Printf("[%v] skipping as machine arch %q is not matching config filter %q\n", confibleFile.Settings.ID, runtime.GOARCH, cfg.Archs)
			continue
		}

		fileFlags := os.O_CREATE
		if cfg.Truncate {
			fileFlags = fileFlags | os.O_TRUNC
		}

		permDir := os.FileMode(0o700)
		if cfg.PermDir != 0 {
			permDir = cfg.PermDir
		}

		permFile := os.FileMode(0o644)
		if cfg.PermFile != 0 {
			permFile = cfg.PermFile
		}

		// create folder for the target file if it doesn't exist
		if err := os.MkdirAll(filepath.Dir(cfg.Path), permDir); err != nil {
			return fmt.Errorf("failed creating target folder (%v): %v", cfg.Path, err)
		}

		// open the target file (doesn't create the folder when it doesn't exit)
		targetFile, err := os.OpenFile(cfg.Path, fileFlags, permFile)
		if err != nil {
			return fmt.Errorf("failed reading target file (%v): %v", cfg.Path, err)
		}
		defer targetFile.Close()

		// process new file content
		var newContent string
		switch mode {
		case ModeNormal:
			newContent, err = modifyContent(targetFile, cfg.Priority, confibleFile.Settings.ID, cfg.Comment, cfg.Append, td, time.Now())
			if err != nil {
				return fmt.Errorf("failed appending new content: %w", err)
			}
		case ModeCleanID, ModeCleanAll:
			if cfg.Truncate {
				log.Printf("[%v] deleted config %q as truncate was enabled\n", confibleFile.Settings.ID, cfg.Path)
				return os.Remove(cfg.Path)
			}

			var existingConfigs []confibleConfig
			newContent, existingConfigs, err = fileContent(targetFile)
			if err != nil {
				return fmt.Errorf("failed cleaning id config: %w", err)
			}

			// ModeCleanAll: we are done, just newContent is enough

			if mode == ModeCleanID {
				for _, existingCfg := range existingConfigs {
					// we want to clean this config
					if existingCfg.id == confibleFile.Settings.ID {
						continue
					}
					// but append all other configs
					newContent = newContent + "\n\n" + strings.TrimSpace(existingCfg.content)
				}
			}
		default:
			return fmt.Errorf("wrong or no mode specified")
		}

		// write content to the file
		if err := os.WriteFile(cfg.Path, []byte(newContent), permFile); err != nil {
			return fmt.Errorf("failed writing target file (%v): %v", cfg.Path, err)
		}

		// explicitly set permissions as the file might already have existed
		// and previous calls don't adjust it when it exists.
		if err := os.Chmod(cfg.Path, permFile); err != nil {
			return fmt.Errorf("failed setting file permisions %q on %q: %v", permFile, cfg.Path, err)
		}

		log.Printf("[%v] wrote/updated config %q\n", confibleFile.Settings.ID, cfg.Path)
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
	if priority == 0 {
		priority = DefaultPriority
	}

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
