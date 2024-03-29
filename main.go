package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/pelletier/go-toml/v2"
	"github.com/sj14/confible/internal/cache"
	"github.com/sj14/confible/internal/command"
	"github.com/sj14/confible/internal/confible"
	"github.com/sj14/confible/internal/config"
	"golang.org/x/exp/slices"
)

var (
	// will be replaced during the build process
	version = "undefined"
	commit  = "undefined"
	date    = "undefined"
)

func main() {
	var (
		applyCmds     = flag.Bool("apply-cmds", true, "exec commands")
		applyCfgs     = flag.Bool("apply-cfgs", true, "apply configs")
		cachedVars    = flag.Bool("cached-vars", true, "use the variables from the cache when present")
		cachedCmds    = flag.Bool("cached-cmds", true, "don't execute commands when they didn't change since last execution")
		cleanID       = flag.Bool("clean", false, "give a confible file and it will remove the config from configured targets matching the config id")
		cacheList     = flag.Bool("cache-list", false, "list the cached variables")
		cachePrune    = flag.Bool("cache-prune", false, "remove the cache file used for all configs")
		cacheClean    = flag.Bool("cache-clean", false, "remove the cache for the given configs")
		cacheFilepath = flag.String("cache-file", cache.GetCacheFilepath(), "custom path to the cache file")
		// verbosity     = flag.Uint("verbosity", 1, "verbosity of the output (0-3)")
		versionFlag = flag.Bool("version", false, fmt.Sprintf("print version information (%v)", version))
	)
	flag.Parse()

	if *versionFlag {
		fmt.Printf("version: %v\n", version)
		fmt.Printf("commit: %v\n", commit)
		fmt.Printf("date: %v\n", date)
	}

	if *cachePrune {
		cache.Prune(*cacheFilepath)
	}

	if *cacheList {
		fmt.Printf("cache path: %s\n\n", *cacheFilepath)
		c, err := cache.New(*cacheFilepath)
		if err != nil {
			log.Fatalf("failed opening cache: %v\n", err)
		}
		c.ListVars()
	}

	mode := config.ModeAppend
	if *cleanID {
		mode = config.ModeCleanID
	}

	if err := processConfibleFiles(flag.Args(), *applyCmds, *applyCfgs, *cachedCmds, *cachedVars, *cacheClean, *cacheFilepath, mode); err != nil {
		log.Fatalln(err)
	}
}

func processConfibleFiles(configPaths []string, execCmds, applyCfgs, cachedCmds, useCachedVars, cleanCache bool, cacheFilepath string, mode config.ContentMode) error {
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

		// check if we can skip this file
		if len(cfg.Settings.OSs) != 0 && !slices.Contains(cfg.Settings.OSs, runtime.GOOS) {
			log.Printf("[%v] skipping as operating system %q is not matching settings filter %q\n", cfg.Settings.ID, runtime.GOOS, cfg.Settings.OSs)
			continue
		}
		if len(cfg.Settings.Archs) != 0 && !slices.Contains(cfg.Settings.Archs, runtime.GOARCH) {
			log.Printf("[%v] skipping as machine arch %q is not matching settings filter %q\n", cfg.Settings.ID, runtime.GOARCH, cfg.Settings.Archs)
			continue
		}

		if cfg.Settings.ID == "" {
			return fmt.Errorf("missing ID for %q", configPath)
		}

		cfgmode := mode
		if cfg.Settings.Deactivated {
			log.Printf("[%v] cleaning configs as 'deactivated' is set\n", cfg.Settings.ID)
			cfgmode = config.ModeCleanID
		}

		if cleanCache {
			log.Printf("[%v] cleaning cache\n", cfg.Settings.ID)
			if err := cache.Clean(cacheFilepath, cfg.Settings.ID); err != nil {
				log.Printf("failed to clean cache for %s\n", cfg.Settings.ID)
			}
		}

		// commands which should run before the configs were written
		if execCmds && cfgmode == config.ModeAppend {
			if err := command.Exec(cfg.Settings.ID, command.Extract(cfg.Commands, false), cachedCmds, cacheFilepath); err != nil {
				return err
			}
		}

		if applyCfgs {
			if err := config.ModifyTargetFiles(cfg, useCachedVars, cacheFilepath, cfgmode); err != nil {
				return err
			}
		}

		// commands which should run after the configs were written
		if execCmds && cfgmode == config.ModeAppend {
			if err := command.Exec(cfg.Settings.ID, command.Extract(cfg.Commands, true), cachedCmds, cacheFilepath); err != nil {
				return err
			}
		}
	}
	return nil
}
