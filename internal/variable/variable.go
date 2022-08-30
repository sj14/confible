package variable

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/sj14/confible/internal/utils"
)

type Variable struct {
	Exec  [][2]string `toml:"exec"`
	Input [][2]string `toml:"input"`
}

func addVar(varMap map[string]string, key, value string) error {
	if _, ok := varMap[key]; ok {
		return fmt.Errorf("variable %q already exists", key)
	}

	varMap[key] = strings.TrimSpace(value)
	return nil
}

type cache struct {
	Variables map[string]string `toml:"variables"`
}

var pathMacOS = utils.AbsFilepath("~/Library/Preferences/confible.toml")

// DON'T FORGET TO CLOSE FILE
func open() (*os.File, error) {
	// open the cache file
	cacheFile, err := os.OpenFile(pathMacOS, os.O_RDWR|os.O_CREATE, 0o666)
	if err != nil {
		return nil, fmt.Errorf("failed creating cache file (%v): %v", pathMacOS, err)
	}

	return cacheFile, nil
}

func LoadCache() (cache, error) {
	cacheFile, err := open()
	if err != nil {
		log.Fatalf("failed to open cache file: %v", err)
	}
	defer cacheFile.Close()

	// read the old cache
	decoder := toml.NewDecoder(cacheFile)
	cache := cache{}
	if err := decoder.Decode(&cache); err != nil {
		return cache, fmt.Errorf("failed decoding confible cache: %v", err)
	}

	return cache, nil
}

// TODO: Store ID together with variables as there might be the same variable names with other IDs
func StoreCache(variables map[string]string) error {
	cache, err := LoadCache()
	if err != nil {
		return err
	}

	if cache.Variables == nil {
		cache.Variables = make(map[string]string)
	}

	// add the new cache values
	for key, value := range variables {
		cache.Variables[key] = value
	}

	// store the new cache
	cacheFile, err := open()
	if err != nil {
		log.Fatalf("failed to open cache file: %v", err)
	}
	defer cacheFile.Close()

	encoder := toml.NewEncoder(cacheFile)
	return encoder.Encode(cache)
}

func Parse(id string, variables []Variable) (map[string]string, error) {
	result := make(map[string]string)

	for _, variables := range variables {
		// variables from commands
		// TODO: combine with command.Exec()
		for _, cmd := range variables.Exec {
			output := &bytes.Buffer{}

			c := exec.Command("sh", "-c", cmd[1])
			c.Stderr = os.Stderr
			c.Stdout = output

			if err := c.Run(); err != nil {
				return nil, fmt.Errorf("failed running command '%v': %v", cmd, err)
			}

			if err := addVar(result, cmd[0], output.String()); err != nil {
				return nil, err
			}
		}

		cache, err := LoadCache()
		if err != nil {
			log.Fatalln(err)
		}

		// variables from input
		for _, input := range variables.Input {
			reader := bufio.NewReader(os.Stdin)
			fmt.Printf("manual input required: %q\n", input[1])
			cachedValue, ok := cache.Variables[input[0]]
			if ok {
				fmt.Printf("press enter to use the cached value %q\n", cachedValue)
			}
			fmt.Print("> ")
			text, err := reader.ReadString('\n')
			if err != nil {
				return nil, fmt.Errorf("failed reading variable input: %v", err)
			}

			if strings.TrimSpace(text) == "" {
				text = cachedValue
			}

			if err := addVar(result, input[0], text); err != nil {
				return nil, err
			}
		}
	}
	return result, StoreCache(result)
}
