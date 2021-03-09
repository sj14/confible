package main

import (
	"bufio"
	"log"
	"os"
	"path/filepath"
	"strings"

	toml "github.com/pelletier/go-toml"
)

type ConfibleFile struct {
	Configs  []Config
	Commands []Command
}

type Config struct {
	Target  string
	Comment string
	Append  string
}

type Command struct {
	Command string
}

const (
	header = "CONFIBLE START"
	footer = "CONFIBLE END"
)

func main() {

	if len(os.Args) < 2 {
		log.Fatalln("need config file")
	}

	var configs []Config
	for _, configPath := range os.Args[1:] {
		log.Printf("parsing config %v\n", configPath)

		configFile, err := os.ReadFile(configPath)
		if err != nil {
			log.Fatalf("failed reading config (%v): %v\n", "TODO confg file", err)
		}

		config := ConfibleFile{}
		if err := toml.Unmarshal(configFile, &config); err != nil {
			log.Printf("failed unmarshalling config file: %v\n", err)
		}

		// Aggregate all configs before executing
		configs = append(configs, config.Configs...)

		// TODO: run commands
		// config.Commands
	}

	modifyFiles(configs)
}

func modifyFiles(configs []Config) {
	for _, cfg := range configs {
		if strings.HasPrefix(cfg.Target, "~") {
			home, err := os.UserHomeDir()
			if err != nil {
				log.Fatalf("failed getting home dir: %v\n", err)
			}
			cfg.Target = filepath.Join(home, cfg.Target[1:])
		}

		targetFile, err := os.OpenFile(cfg.Target, os.O_CREATE, 0o666)
		if err != nil {
			log.Fatalf("failed reading target file (%v): %v\n", cfg.Target, err)
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

		if _, err := newContent.WriteString(cfg.Comment + " " + header + "\n"); err != nil {
			log.Fatalln(err)
		}
		if _, err := newContent.WriteString(cfg.Append); err != nil {
			log.Fatalln(err)
		}
		if _, err := newContent.WriteString("\n" + cfg.Comment + " " + footer + "\n"); err != nil {
			log.Fatalln(err)
		}

		if err := os.WriteFile(cfg.Target, []byte(newContent.String()), os.ModePerm); err != nil {
			log.Fatalf("failed writing target file (%v): %v\n", cfg.Target, err)
		}
	}
}
