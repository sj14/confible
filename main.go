package main

import (
	"bufio"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	toml "github.com/pelletier/go-toml"
)

type ConfibleFile struct {
	Configs  []Config
	Commands []Commands
}

type Config struct {
	Target        string
	CommentSymbol string `toml:"comment_symbol"`
	Append        string
}

type Commands struct {
	Commands []string
}

const (
	header = "CONFIBLE START"
	footer = "CONFIBLE END"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalln("need config file")
	}

	var (
		configs []Config
	)
	for _, configPath := range os.Args[1:] {
		log.Printf("parsing config %v\n", configPath)

		configFile, err := os.Open(configPath)
		if err != nil {
			log.Fatalf("failed reading config (%v): %v\n", "TODO confg file", err)
		}

		dec := toml.NewDecoder(configFile)
		dec.Strict(true)

		config := ConfibleFile{}
		if err := dec.Decode(&config); err != nil {
			log.Printf("failed unmarshalling config file: %v\n", err)
		}

		// Aggregate all configs before executing
		configs = append(configs, config.Configs...)

		for _, commands := range config.Commands {
			for _, cmd := range commands.Commands {
				args := strings.Split(cmd, " ")
				c := exec.Command(args[0], args[1:]...)
				c.Stderr = os.Stderr
				c.Stdout = os.Stdout

				log.Printf("running command: %v\n", cmd)

				if err := c.Run(); err != nil {
					log.Fatalf("failed running command '%v': %v\n", cmd, err)
				}
			}
		}
	}

	modifyFiles(configs)
}

func modifyFiles(configs []Config) {
	for _, cfg := range configs {
		if cfg.Append == "" {
			log.Fatalf("missing append\n")
		}
		if cfg.Target == "" {
			log.Fatalf("missing target\n")
		}
		if cfg.CommentSymbol == "" {
			log.Fatalf("missing comment symbol\n")
		}

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

		if _, err := newContent.WriteString(cfg.CommentSymbol + " " + header + "\n"); err != nil {
			log.Fatalln(err)
		}
		if _, err := newContent.WriteString(cfg.Append); err != nil {
			log.Fatalln(err)
		}
		if _, err := newContent.WriteString("\n" + cfg.CommentSymbol + " " + footer + "\n"); err != nil {
			log.Fatalln(err)
		}

		if err := os.WriteFile(cfg.Target, []byte(newContent.String()), os.ModePerm); err != nil {
			log.Fatalf("failed writing target file (%v): %v\n", cfg.Target, err)
		}
	}
}
