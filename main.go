package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/pelletier/go-toml/v2"
	"github.com/sj14/confible/internal/command"
	"github.com/sj14/confible/internal/config"
)

var (
	// will be replaced during the build process
	version = "undefined"
	commit  = "undefined"
	date    = "undefined"
)

type confibleFile struct {
	ID       string            `toml:"id"`
	Configs  []config.Config   `toml:"config"`
	Commands []command.Command `toml:"commands"`
}

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

	if flag.NArg() < 2 {
		log.Fatalln("need a config file")
	}

	switch flag.Arg(0) {
	case "apply":
		if err := processConfibleFiles(flag.Args()[1:], *noCommands, *noConfig, config.ModeAppend); err != nil {
			log.Fatalln(err)
		}
	case "clean":
		if err := processConfibleFiles(flag.Args()[1:], *noCommands, *noConfig, config.ModeClean); err != nil {
			log.Fatalln(err)
		}
	default:
		log.Fatalln("missing 'apply' or 'clean' command")
	}
}

func processConfibleFiles(configPaths []string, noCommands, noConfig bool, mode uint8) error {
	for _, configPath := range configPaths {
		log.Printf("processing config %v\n", configPath)

		configFile, err := os.Open(configPath)
		if err != nil {
			log.Fatalf("failed reading config (%v): %v\n", configPath, err)
		}

		dec := toml.NewDecoder(configFile)
		dec.DisallowUnknownFields()

		cfg := confibleFile{}
		if err := dec.Decode(&cfg); err != nil {
			return fmt.Errorf("failed unmarshalling config file: %v", err)
		}

		if cfg.ID == "" {
			return fmt.Errorf("missing ID for %q", configPath)
		}

		if !noCommands && mode == config.ModeAppend {
			command.Exec(cfg.Commands)
		}

		if !noConfig {
			if err := config.ModifyTargetFiles(cfg.ID, cfg.Configs, mode); err != nil {
				return err
			}
		}
	}
	return nil
}
