package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/pelletier/go-toml/v2"
	"github.com/sj14/confible/internal/command"
	"github.com/sj14/confible/internal/config"
	"github.com/sj14/confible/internal/utils"
	"github.com/sj14/confible/internal/variable"
)

var (
	// will be replaced during the build process
	version = "undefined"
	commit  = "undefined"
	date    = "undefined"
)

type confibleFile struct {
	ID        string              `toml:"id"`
	Configs   []config.Config     `toml:"config"`
	Commands  []command.Command   `toml:"commands"`
	Variables []variable.Variable `toml:"variables"`
}

func main() {
	var (
		noCommands = flag.Bool("no-cmd", false, "do not exec any commands")
		noConfig   = flag.Bool("no-cfg", false, "do not apply any configs")
		cleanID    = flag.Bool("clean-id", false, "give a confible file and it will remove the config from configured targets matching the config id")
		cleanAll   = flag.Bool("clean-all", false, "give a confible file and it will remove all configs from the targets")
		// cleanTarget = flag.Bool("clean-target", false, "give the target file and it will remove all configs (ignores no-cmd and no-cfg flags)")
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
		log.Fatalln("need at least one config file")
	}

	if *cleanID {
		if err := processConfibleFiles(flag.Args(), *noCommands, *noConfig, config.ModeCleanWithoutID); err != nil {
			log.Fatalln(err)
		}
		return
	}

	if *cleanAll {
		if err := processConfibleFiles(flag.Args(), *noCommands, *noConfig, config.ModeCleanWithoutAll); err != nil {
			log.Fatalln(err)
		}
		return
	}

	if err := processConfibleFiles(flag.Args(), *noCommands, *noConfig, config.ModeNormal); err != nil {
		log.Fatalln(err)
	}
}

func processConfibleFiles(configPaths []string, noCommands, noConfig bool, mode config.ContentMode) error {
	for _, configPath := range configPaths {
		log.Printf("processing config %q\n", configPath)

		configFile, err := os.Open(configPath)
		if err != nil {
			return fmt.Errorf("failed reading config %q: %v", configPath, err)
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

		if !noCommands && mode == config.ModeNormal {
			command.Exec(cfg.Commands)
		}

		if !noConfig {
			var td config.TemplateData

			if mode == config.ModeNormal {
				// only create template when we are not in a clean mode
				variableMap, err := variable.Parse(cfg.ID, cfg.Variables)
				if err != nil {
					return err
				}

				td = config.TemplateData{
					Env: utils.GetEnvMap(),
					Var: variableMap,
				}
			}
			if err := config.ModifyTargetFiles(cfg.ID, cfg.Configs, td, mode); err != nil {
				return err
			}
		}
	}
	return nil
}
