package cache

import (
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sj14/confible/internal/utils"
)

type IdVariable struct {
	ID           string
	VariableName string
}

type VariableMap map[IdVariable]string

type cache struct {
	Variables VariableMap `toml:"variables"`
}

func AddVar(varMap VariableMap, id, name, value string) error {
	key := IdVariable{ID: id, VariableName: name}
	if _, ok := varMap[key]; ok {
		return fmt.Errorf("variable %q already exists for ID %q", name, id)
	}

	varMap[key] = strings.TrimSpace(value)
	return nil
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

func Clean() {
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

func Load() (cache, error) {
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

func Store(id string, variables VariableMap) error {
	cache, err := Load()
	if err != nil {
		return err
	}

	if cache.Variables == nil {
		cache.Variables = make(VariableMap)
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

func OmitID(m VariableMap) map[string]string {
	result := make(map[string]string)

	for key, val := range m {
		result[key.VariableName] = val
	}

	return result
}
