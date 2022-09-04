package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/pelletier/go-toml/v2"
	"github.com/sj14/confible/internal/cache"
	"github.com/sj14/confible/internal/command"
	"github.com/sj14/confible/internal/confible"
	"github.com/sj14/confible/internal/config"
)

var (
	// will be replaced during the build process
	version = "undefined"
	commit  = "undefined"
	date    = "undefined"
)

func main() {
	var (
		applyCmds  = flag.Bool("apply-cmds", true, "exec commands")
		applyCfgs  = flag.Bool("apply-cfgs", true, "apply configs")
		cachedVars = flag.Bool("cached-vars", true, "use the variables from the cache when present")
		cachedCmds = flag.Bool("cached-cmds", true, "don't execute commands when they didn't change since last execution")
		cleanID    = flag.Bool("clean-id", false, "give a confible file and it will remove the config from configured targets matching the config id")
		cleanAll   = flag.Bool("clean-all", false, "give a confible file and it will remove all configs from the targets")
		// cleanTarget = flag.Bool("clean-target", false, "give the target file and it will remove all configs (ignores no-cmd and no-cfg flags)")
		cleanCache  = flag.Bool("clean-cache", false, "remove the cache file and exit")
		versionFlag = flag.Bool("version", false, fmt.Sprintf("print version information (%v)", version))
	)
	flag.Parse()

	if *versionFlag {
		fmt.Printf("version: %v\n", version)
		fmt.Printf("commit: %v\n", commit)
		fmt.Printf("date: %v\n", date)
		os.Exit(0)
	}

	if *cleanCache {
		cache.Clean()
		os.Exit(0)
	}

	if flag.NArg() < 1 {
		log.Fatalln("need at least one config file")
	}

	mode := config.ModeNormal
	if *cleanID {
		mode = config.ModeCleanID
	}

	if *cleanAll {
		mode = config.ModeCleanAll
	}

	if err := processConfibleFiles(flag.Args(), *applyCmds, *applyCfgs, *cachedCmds, *cachedVars, mode); err != nil {
		log.Fatalln(err)
	}
}

func processConfibleFiles(configPaths []string, execCmds, applyCfgs, cachedCmds, useCachedVars bool, mode config.ContentMode) error {
	for _, configPath := range configPaths {
		log.Printf("processing config %q\n", configPath)

		configFile, err := os.Open(configPath)
		if err != nil {
			return fmt.Errorf("failed reading config %q: %v", configPath, err)
		}

		dec := toml.NewDecoder(configFile)
		dec.DisallowUnknownFields()

		cfg := confible.File{}
		if err := dec.Decode(&cfg); err != nil {
			return fmt.Errorf("failed unmarshalling config file: %v", err)
		}

		if cfg.ID == "" {
			return fmt.Errorf("missing ID for %q", configPath)
		}

		if execCmds && mode == config.ModeNormal {
			if err := command.Exec(cfg.ID, cfg.Commands, cachedCmds); err != nil {
				return err
			}
		}

		if applyCfgs {
			if err := config.ModifyTargetFiles(cfg, useCachedVars, mode); err != nil {
				return err
			}
		}
	}
	return nil
}
