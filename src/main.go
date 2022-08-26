package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/pelletier/go-toml/v2"
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
		if err := processConfibleFiles(flag.Args()[1:], *noCommands, *noConfig, modeAppend); err != nil {
			log.Fatalln(err)
		}
	case "clean":
		if err := processConfibleFiles(flag.Args()[1:], *noCommands, *noConfig, modeClean); err != nil {
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

		config := confibleFile{}
		if err := dec.Decode(&config); err != nil {
			return fmt.Errorf("failed unmarshalling config file: %v", err)
		}

		if config.ID == "" {
			return fmt.Errorf("missing ID for %q", configPath)
		}

		if !noCommands && mode == modeAppend {
			execCmds(config.Commands)
		}

		if !noConfig {
			if err := modifyTargetFiles(config.ID, config.Configs, mode); err != nil {
				return err
			}
		}
	}
	return nil
}
