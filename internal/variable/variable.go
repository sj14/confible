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
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sj14/confible/internal/confible"
	"github.com/sj14/confible/internal/utils"
)

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

func getCacheFilepath() string {
	switch runtime.GOOS {
	case "darwin":
		return utils.AbsFilepath("~/Library/Preferences/confible.cache")
	case "linux":
		return os.ExpandEnv("$XDG_CONFIG_DIRS/confible.cache")
	case "windows":
		return os.ExpandEnv("$LOCALAPPDATA\\confible.cache")
	default:
		return utils.AbsFilepath(filepath.Join("~", ".confible.cache"))
	}
}

func CleanCache() {
	cacheFile, err := open()
	if err != nil {
		log.Fatalf("failed to open cache file: %v", err)
	}
	defer cacheFile.Close()

	if err := cacheFile.Truncate(0); err != nil {
		log.Fatalf("failed cleaning cache file: %v\n", err)
	}

}

// DON'T FORGET TO CLOSE FILE
func open() (*os.File, error) {
	// open the cache file
	cacheFilepath := getCacheFilepath()
	cacheFile, err := os.OpenFile(cacheFilepath, os.O_RDWR|os.O_CREATE, 0o666)
	if err != nil {
		return nil, fmt.Errorf("failed creating cache file (%v): %v", cacheFilepath, err)
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

func omitID(m variableMap) map[string]string {
	result := make(map[string]string)

	for key, val := range m {
		result[key.VariableName] = val
	}

	return result
}

func Parse(id string, variables []confible.Variable, useCached bool) (map[string]string, error) {
	result := make(variableMap)

	cache, err := loadCache()
	if err != nil {
		log.Fatalln(err)
	}

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

		// variables from input
		for _, input := range variables.Input {
			cachedValue, cacheFound := cache.Variables[idVariable{id, input.VariableName}]
			if cacheFound {
				if useCached {
					if err := addVar(result, idVariable{id, input.VariableName}, cachedValue); err != nil {
						return nil, err
					}
					continue
				}
			}

			reader := bufio.NewReader(os.Stdin)
			fmt.Printf("manual input required: %q\n", input.Prompt)
			if cacheFound {
				fmt.Printf("press enter to use the cached value: %q\n", cachedValue)
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
	return omitID(result), storeCache(id, result)
}
