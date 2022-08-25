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

	toml "github.com/pelletier/go-toml/v2"
)

var (
	// will be replaced during the build process
	version = "undefined"
	commit  = "undefined"
	date    = "undefined"
)

type confibleFile struct {
	ID       string    `toml:"id"`
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
		noCommands  = flag.Bool("no-cmd", false, "do not exec any commands")
		noConfig    = flag.Bool("no-cfg", false, "do not apply any configs")
		versionFlag = flag.Bool("version", false, fmt.Sprintf("print version information (%v)", version))
	)
	flag.Parse()

	if *versionFlag {
		fmt.Printf("version: %v\n", version)
		fmt.Printf("commit: %v\n", commit)
		fmt.Printf("date: %v\n", date)
		os.Exit(0)
	}

	if flag.NArg() < 1 {
		log.Fatalln("need a config file")
	}

	if err := processConfigs(flag.Args(), *noCommands, *noConfig); err != nil {
		log.Fatalln(err)
	}
}

func processConfigs(configPaths []string, noCommands, noConfig bool) error {
	for _, configPath := range configPaths {
		log.Printf("parsing config %v\n", configPath)

		configFile, err := os.Open(configPath)
		if err != nil {
			log.Fatalf("failed reading config (%v): %v\n", configPath, err)
		}

		dec := toml.NewDecoder(configFile)
		dec.DisallowUnknownFields()

		config := confibleFile{}
		if err := dec.Decode(&config); err != nil {
			return fmt.Errorf("failed unmarshalling config file: %v", err)
		}

		if config.ID == "" {
			return fmt.Errorf("missing ID for %q", configPath)
		}

		if !noCommands {
			execCmds(config.Commands)
		}

		if !noConfig {
			if err := modifyTargetFiles(config.ID, config.Configs); err != nil {
				return err
			}
		}
	}
	return nil
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

func modifyTargetFiles(id string, configs []config) error {
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
		newContent, err := appendContent(targetFile, id, cfg.Comment, cfg.Append, time.Now())
		if err != nil {
			return fmt.Errorf("failed appending new content: %w", err)
		}

		// write content to the file
		if err := os.WriteFile(cfg.Path, []byte(newContent), os.ModePerm); err != nil {
			return fmt.Errorf("failed writing target file (%v): %v", cfg.Path, err)
		}
	}
	return nil
}

func appendContent(reader io.Reader, id, comment, appendText string, now time.Time) (string, error) {
	var (
		newContent   = strings.Builder{}
		headerWithID = fmt.Sprintf(header+" id: %q", id)
		footerWithID = fmt.Sprintf(footer+" id: %q", id)
	)

	// read the already existing file content
	scanner := bufio.NewScanner(reader)
	skip := false
	for scanner.Scan() {
		// we reached our own old config, do not copy our old config
		if strings.Contains(scanner.Text(), headerWithID) {
			skip = true
		}

		// our config was read, continue copying the other content
		if strings.Contains(scanner.Text(), footerWithID) {
			skip = false
			continue
		}

		if skip {
			continue
		}

		if _, err := newContent.Write(scanner.Bytes()); err != nil {
			return "", err
		}
		if err := newContent.WriteByte('\n'); err != nil {
			return "", err
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}

	// Add blank line before confible part (only when the target file is not empty)
	if strings.TrimSpace(newContent.String()) != "" && !strings.HasSuffix(newContent.String(), "\n\n") {
		newContent.WriteByte('\n')
	}

	// header
	if _, err := newContent.WriteString(comment + " ~~~ " + headerWithID + " ~~~\n" + comment + " " + now.Format(time.RFC1123) + "\n"); err != nil {
		return "", err
	}
	// config
	if _, err := newContent.WriteString(strings.TrimSuffix(appendText, "\n")); err != nil {
		return "", err
	}
	// footer
	if _, err := newContent.WriteString("\n" + comment + " ~~~ " + footerWithID + " ~~~\n"); err != nil {
		return "", err
	}

	return newContent.String(), nil
}
