package variable

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/sj14/confible/internal/utils"
)

type VarVal struct {
	VariableName string `toml:"var"`
	Prompt       string `toml:"prompt"`
}

type VarCmd struct {
	VariableName string `toml:"var"`
	Cmd          string `toml:"cmd"`
}

type Variable struct {
	Exec  []VarCmd `toml:"exec"`
	Input []VarVal `toml:"input"`
}

func addVar(varMap variableMap, key idVariable, value string) error {
	if _, ok := varMap[key]; ok {
		return fmt.Errorf("variable %q already exists", key)
	}

	varMap[key] = strings.TrimSpace(value)
	return nil
}

type idVariable struct {
	ID           string
	VariableName string
}

type variableMap map[idVariable]string

type cache struct {
	Variables variableMap `toml:"variables"`
}

var pathMacOS = utils.AbsFilepath("~/Library/Preferences/confible.cache")

// DON'T FORGET TO CLOSE FILE
func open() (*os.File, error) {
	// open the cache file
	cacheFile, err := os.OpenFile(pathMacOS, os.O_RDWR|os.O_CREATE, 0o666)
	if err != nil {
		return nil, fmt.Errorf("failed creating cache file (%v): %v", pathMacOS, err)
	}

	return cacheFile, nil
}

func loadCache() (cache, error) {
	cacheFile, err := open()
	if err != nil {
		log.Fatalf("failed to open cache file: %v", err)
	}
	defer cacheFile.Close()

	// read the old cache
	decoder := gob.NewDecoder(cacheFile)
	cache := cache{}
	if err := decoder.Decode(&cache); err != nil && err != io.EOF {
		return cache, fmt.Errorf("failed decoding confible cache: %v", err)
	}

	return cache, nil
}

func storeCache(id string, variables variableMap) error {
	cache, err := loadCache()
	if err != nil {
		return err
	}

	if cache.Variables == nil {
		cache.Variables = make(variableMap)
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

	encoder := gob.NewEncoder(cacheFile)
	return encoder.Encode(cache)
}

func OmitId(m variableMap) map[string]string {
	result := make(map[string]string)

	for key, val := range m {
		result[key.VariableName] = val
	}

	return result
}

func Parse(id string, variables []Variable) (variableMap, error) {
	result := make(variableMap)

	for _, variables := range variables {
		// variables from commands
		// TODO: combine with command.Exec()
		for _, cmd := range variables.Exec {
			output := &bytes.Buffer{}

			c := exec.Command("sh", "-c", cmd.Cmd)
			c.Stderr = os.Stderr
			c.Stdout = output

			if err := c.Run(); err != nil {
				return nil, fmt.Errorf("failed running command '%v': %v", cmd, err)
			}

			if err := addVar(result, idVariable{id, cmd.VariableName}, output.String()); err != nil {
				return nil, err
			}
		}

		cache, err := loadCache()
		if err != nil {
			log.Fatalln(err)
		}

		// variables from input
		for _, input := range variables.Input {
			reader := bufio.NewReader(os.Stdin)
			fmt.Printf("manual input required: %q\n", input.VariableName)
			cachedValue, ok := cache.Variables[idVariable{id, input.VariableName}]
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

			if err := addVar(result, idVariable{id, input.VariableName}, text); err != nil {
				return nil, err
			}
		}
	}
	return result, storeCache(id, result)
}
