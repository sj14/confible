package main

import (
	"bufio"
	"flag"
	"log"
	"os"
	"os/exec"
	"path/filepath"
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
		modifyFiles(configs)
	}
}

func execCmds(commands []command) {
	for _, commands := range commands {
		for _, cmd := range commands.Exec {
			args := strings.Split(strings.TrimSpace(cmd), " ")
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

func modifyFiles(configs []config) {
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

		if _, ok := configsMap[cfg.Path]; !ok {
			configsMap[cfg.Path] = cfg
			continue
		}

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

	for _, cfg := range configsMap {
		flag := os.O_CREATE
		if cfg.Truncate {
			flag = os.O_CREATE | os.O_TRUNC
		}

		targetFile, err := os.OpenFile(cfg.Path, flag, 0o666)
		if err != nil {
			log.Fatalf("failed reading target file (%v): %v\n", cfg.Path, err)
		}
		defer targetFile.Close()

		newContent := strings.Builder{}

		scanner := bufio.NewScanner(targetFile)
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
			log.Fatal(err)
		}

		if _, err := newContent.WriteString(cfg.Comment + " ~~~ " + header + " ~~~\n" + cfg.Comment + " " + time.Now().Format(time.RFC1123) + "\n"); err != nil {
			log.Fatalln(err)
		}
		if _, err := newContent.WriteString(cfg.Append); err != nil {
			log.Fatalln(err)
		}
		if _, err := newContent.WriteString("\n" + cfg.Comment + " ~~~ " + footer + " ~~~\n"); err != nil {
			log.Fatalln(err)
		}

		if err := os.WriteFile(cfg.Path, []byte(newContent.String()), os.ModePerm); err != nil {
			log.Fatalf("failed writing target file (%v): %v\n", cfg.Path, err)
		}
	}
}
