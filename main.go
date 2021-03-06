package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	toml "github.com/pelletier/go-toml"
)

type confibleFile struct {
	Configs  []config  `toml:"config"`
	Commands []command `toml:"commands"`
}

type config struct {
	Name     string `toml:"name"`
	Path     string `toml:"path"`
	Truncate bool   `toml:"truncate"`
	Comment  string `toml:"comment_symbol"`
	Append   string `toml:"append"`
}

type command struct {
	Name string   `toml:"name"`
	Exec []string `toml:"exec"`
}

const (
	header = "CONFIBLE START"
	footer = "CONFIBLE END"
)

func main() {
	var (
		noCommands = flag.Bool("no-cmd", false, "do not exec any commands")
		noConfig   = flag.Bool("no-cfg", false, "do not apply any configs")
	)
	flag.Parse()

	if flag.NArg() < 1 {
		log.Fatalln("need a config file")
	}

	var configs []config
	handledPaths := make(map[string]struct{})

	for _, configPath := range flag.Args() {
		// check if the same config file would be applied multiple times
		if _, ok := handledPaths[configPath]; ok {
			continue
		}
		handledPaths[configPath] = struct{}{}
		log.Printf("parsing config %v\n", configPath)

		configFile, err := os.Open(configPath)
		if err != nil {
			log.Fatalf("failed reading config (%v): %v\n", configPath, err)
		}

		dec := toml.NewDecoder(configFile)
		dec.Strict(true)

		config := confibleFile{}
		if err := dec.Decode(&config); err != nil {
			log.Fatalf("failed unmarshalling config file: %v\n", err)
		}

		// Aggregate all configs before appending
		configs = append(configs, config.Configs...)

		if !*noCommands {
			execCmds(config.Commands)
		}
	}

	if !*noConfig {
		if err := modifyFiles(configs); err != nil {
			log.Fatalln(err)
		}
	}
}

// split on spaces except when inside quotes
var regArgs = regexp.MustCompile(`("[^"]+?"\S*|\S+)`)

func execCmds(commands []command) {
	for _, commands := range commands {
		for _, cmd := range commands.Exec {
			args := regArgs.FindAllString(cmd, -1)

			c := exec.Command(args[0], args[1:]...)
			c.Stderr = os.Stderr
			c.Stdout = os.Stdout

			log.Printf("[%v] running: %v\n", commands.Name, cmd)

			if err := c.Run(); err != nil {
				log.Fatalf("failed running command '%v': %v\n", cmd, err)
			}
		}
	}
}

// validate and aggregate configs which target the same file
func aggregateConfigs(configs []config) []config {
	// the key is the path of the config file
	configsMap := make(map[string]config)

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
			log.Printf("multiple comment styles for %q (%v) and %q (%v) using %v\n", old.Name, old.Comment, cfg.Name, cfg.Comment, old.Comment)
		}
		if old.Truncate != cfg.Truncate {
			log.Fatalf("file should be truncated and not '%v' \n", old.Path)
		}

		old.Append += cfg.Append
		configsMap[cfg.Path] = old
	}

	var aggregated []config
	for _, cfg := range configsMap {
		aggregated = append(aggregated, cfg)
	}

	return aggregated
}

func modifyFiles(configs []config) error {
	configs = aggregateConfigs(configs)

	for _, cfg := range configs {
		flag := os.O_CREATE
		if cfg.Truncate {
			flag = os.O_CREATE | os.O_TRUNC
		}

		targetFile, err := os.OpenFile(cfg.Path, flag, 0o666)
		if err != nil {
			return fmt.Errorf("failed reading target file (%v): %v", cfg.Path, err)
		}
		defer targetFile.Close()

		newContent, err := appendContent(targetFile, cfg.Comment, cfg.Append, time.Now())
		if err != nil {
			return fmt.Errorf("failed appending new content: %w", err)
		}

		if err := os.WriteFile(cfg.Path, []byte(newContent), os.ModePerm); err != nil {
			return fmt.Errorf("failed writing target file (%v): %v", cfg.Path, err)
		}
	}
	return nil
}

func appendContent(reader io.Reader, comment, appendText string, now time.Time) (string, error) {
	newContent := strings.Builder{}

	scanner := bufio.NewScanner(reader)
	skip := false
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), header) {
			skip = true
		}

		if strings.Contains(scanner.Text(), footer) {
			skip = false
			continue
		}

		if skip {
			continue
		}

		newContent.Write(scanner.Bytes())
		newContent.WriteByte('\n')
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	// Add blank line before confible part
	if !strings.HasSuffix(newContent.String(), "\n\n") {
		newContent.WriteByte('\n')
	}

	if _, err := newContent.WriteString(comment + " ~~~ " + header + " ~~~\n" + comment + " " + now.Format(time.RFC1123) + "\n"); err != nil {
		return "", err
	}
	if _, err := newContent.WriteString(strings.TrimSuffix(appendText, "\n")); err != nil {
		return "", err
	}
	if _, err := newContent.WriteString("\n" + comment + " ~~~ " + footer + " ~~~\n"); err != nil {
		return "", err
	}

	return newContent.String(), nil
}
