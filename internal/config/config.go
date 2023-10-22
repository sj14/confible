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

	if mode == ModeAppend {
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

		existingContent := &strings.Builder{}
		_, err = io.Copy(existingContent, targetFile)
		if err != nil {
			return err
		}

		// process new file content
		var newContent string
		switch mode {
		case ModeAppend:
			newContent, err = appendConfig(existingContent.String(), cfg.Priority, confibleFile.Settings.ID, cfg.Comment, cfg.Append, td, time.Now())
			if err != nil {
				return fmt.Errorf("failed appending new content: %w", err)
			}
		case ModeCleanID:
			if cfg.Truncate {
				log.Printf("[%v] deleted config %q as truncate was enabled\n", confibleFile.Settings.ID, cfg.Path)
				return os.Remove(cfg.Path)
			}

			configs, err := extractConfigs(existingContent.String())
			if err != nil {
				return fmt.Errorf("failed cleaning id config: %w", err)
			}

			// we want to clean this config
			configs = removeConfig(configs, confibleFile.Settings.ID)

			// write new content without config
			newContent = removeConfigs(existingContent.String())

			for _, cfg := range configs {
				newContent = newContent + "\n\n" + strings.TrimSpace(cfg.content)
			}
			newContent = newContent + "\n"
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

		log.Printf("[%v] wrote config %q\n", confibleFile.Settings.ID, cfg.Path)
	}
	return nil
}

type ContentMode uint8

const (
	ModeAppend ContentMode = iota
	ModeCleanID
)

type confibleConfig struct {
	id       string
	priority int64
	content  string
}

func extractConfigs(existing string) ([]confibleConfig, error) {
	configs := []confibleConfig{}
	configProcessing := confibleConfig{}

	// read the already existing file content
	scanner := bufio.NewScanner(strings.NewReader(existing))
	processingAnExistingConfig := false
	for scanner.Scan() {
		// we reached a confible config
		if strings.Contains(scanner.Text(), header) {
			configProcessing.id = extractID(scanner.Text())
			configProcessing.priority = extractPriority(scanner.Text())
			configProcessing.content = configProcessing.content + "\n\n"
			processingAnExistingConfig = true
		}

		// old confible config was entirely read, append it to the slice
		if strings.Contains(scanner.Text(), footer) {
			processingAnExistingConfig = false
			configProcessing.content = configProcessing.content + scanner.Text() + "\n\n\n"
			configs = append(configs, configProcessing)
			configProcessing = confibleConfig{}
			continue
		}

		if processingAnExistingConfig {
			configProcessing.content = configProcessing.content + scanner.Text() + "\n"
			continue
		}
	}
	if err := scanner.Err(); err != nil {
		return configs, err
	}

	return configs, nil
}

func removeConfig(existingConfigs []confibleConfig, idToRemove string) []confibleConfig {
	var result []confibleConfig

	for _, existingCfg := range existingConfigs {
		// we want to clean this config
		if existingCfg.id == idToRemove {
			continue
		}
		// but append all other configs
		result = append(result, existingCfg)
	}

	return result
}

func generateHeaderWithID(id string) string {
	return fmt.Sprintf(header+" id: %q", id)
}

func generateHeaderWithIDAndPriority(id string, priority int64) string {
	return fmt.Sprintf(generateHeaderWithID(id)+" priority: \"%v\"", priority)
}

func extractConfigMeta(s, startString string) string {
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
	return extractConfigMeta(s, "id: \"")
}

var DefaultPriority int64 = 1000

func extractPriority(s string) int64 {
	priorityStr := extractConfigMeta(s, "priority: \"")
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

func removeConfigs(existing string) string {
	result := strings.Builder{}

	scanner := bufio.NewScanner(strings.NewReader(existing))
	processingAnExistingConfig := false
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), header) {
			processingAnExistingConfig = true
			continue
		}
		if strings.Contains(scanner.Text(), footer) {
			processingAnExistingConfig = false
			continue
		}
		if processingAnExistingConfig {
			continue
		}
		result.WriteString(scanner.Text() + "\n")

	}
	return strings.TrimSpace(result.String())
}

func newConfig(comment, id, appendText string, priority int64, td TemplateData, now time.Time) confibleConfig {
	content := strings.Builder{}
	// header
	content.WriteString(comment + " ~~~ " + generateHeaderWithIDAndPriority(id, priority) + " ~~~\n" + comment + " " + now.Format(time.RFC1123) + "\n")

	templ, err := template.New("").Parse(strings.TrimSpace(appendText))
	if err != nil {
		panic(err)
	}

	err = templ.Execute(&content, td)
	if err != nil {
		panic(err)
	}

	// footer
	content.WriteString("\n" + comment + " ~~~ " + generateFooterWithID(id) + " ~~~\n")

	return confibleConfig{
		id:       id,
		priority: priority,
		content:  content.String(),
	}
}

func appendConfig(existing string, priority int64, id, comment, appendText string, td TemplateData, now time.Time) (string, error) {
	if priority == 0 {
		priority = DefaultPriority
	}

	// get existing configs
	newConfigs, err := extractConfigs(existing)
	if err != nil {
		return "", err
	}

	// remove old configs with same id
	newConfigs = removeConfig(newConfigs, id)

	// add new or updated config
	newConfigs = append(newConfigs, newConfig(comment, id, appendText, priority, td, now))

	// start new content
	newContent := strings.Builder{}

	// add old content but without any configs
	newContent.WriteString(removeConfigs(existing))

	// sort configs by priority
	sort.Slice(newConfigs, func(i, j int) bool {
		return newConfigs[i].priority < newConfigs[j].priority
	})

	// oldContent := &strings.Builder{}

	// dmp := diffmatchpatch.New()
	// diffs := dmp.DiffMain(existing, newContent.String(), false)
	// fmt.Println("new:" + dmp.DiffPrettyText(diffs))

	// append configs to new content
	newContent.WriteString("\n\n")

	for _, cfg := range newConfigs {
		newContent.WriteString(strings.TrimSpace(cfg.content) + "\n\n")
	}

	return strings.TrimSpace(newContent.String()), nil
}
