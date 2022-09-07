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

	"github.com/sj14/confible/internal/confible"
	"github.com/sj14/confible/internal/utils"
)

// key == variable name; value == variable value
type keyValueMap map[string]string

// key == id
type variablesMap map[string]keyValueMap

// key == id
type commandsMap map[string][]confible.Command

type Cache struct {
	path      string
	variables variablesMap
	commands  commandsMap
}

// I don't want to export the variables, thus a new struct which won't be returned in any public func.
type cacheGob struct {
	Variables variablesMap
	Commands  commandsMap
}

func gobTocache(gobCache cacheGob, cachePath string) Cache {
	return Cache{
		path:      cachePath,
		variables: gobCache.Variables,
		commands:  gobCache.Commands,
	}
}

func cacheToGob(c Cache) cacheGob {
	return cacheGob{
		Variables: c.variables,
		Commands:  c.commands,
	}
}

// store only when we processed all commands!
// Do not update one after another! All at once!
func (c *Cache) UpsertCommands(id string, commands []confible.Command) {
	c.commands[id] = commands
}

func (c *Cache) UpsertVar(id, name, value string) {
	if c.variables[id] == nil {
		c.variables[id] = make(keyValueMap)
	}
	c.variables[id][name] = strings.TrimSpace(value)
}

func GetCacheFilepath() string {
	switch runtime.GOOS {
	case "darwin":
		return utils.AbsFilepath("~/Library/Preferences/confible.cache")
	case "linux":
		if os.Getenv("XDG_CONFIG_DIRS") != "" {
			return os.ExpandEnv("$XDG_CONFIG_DIRS/confible.cache")
		}
		return os.ExpandEnv("$HOME/.confible.cache")
	case "windows":
		return os.ExpandEnv("$LOCALAPPDATA\\confible.cache")
	default:
		return utils.AbsFilepath(filepath.Join("~", ".confible.cache"))
	}
}

func Clean(fp string) {
	cacheFile, err := open(fp)
	if err != nil {
		log.Fatalf("failed to open cache file: %v", err)
	}
	defer cacheFile.Close()

	if err := cacheFile.Truncate(0); err != nil {
		log.Fatalf("failed cleaning cache file: %v\n", err)
	}

}

// DON'T FORGET TO CLOSE FILE
func open(cacheFilepath string) (*os.File, error) {
	// open the cache file
	cacheFile, err := os.OpenFile(cacheFilepath, os.O_RDWR|os.O_CREATE, 0o666)
	if err != nil {
		return nil, fmt.Errorf("failed creating cache file (%v): %v", cacheFilepath, err)
	}

	return cacheFile, nil
}

func New(path string) (*Cache, error) {
	c := &Cache{
		path: path,
	}
	return c, c.load()
}

func (c *Cache) load() error {
	cacheFile, err := open(c.path)
	if err != nil {
		log.Fatalf("failed to open cache file: %v", err)
	}
	defer cacheFile.Close()

	// read the old cache
	decoder := gob.NewDecoder(cacheFile)
	gobCache := cacheGob{}
	if err := decoder.Decode(&gobCache); err != nil && err != io.EOF {
		return fmt.Errorf("failed decoding confible cache: %v", err)
	}

	*c = gobTocache(gobCache, c.path)
	if c.variables == nil {
		c.variables = make(variablesMap)
	}
	if c.commands == nil {
		c.commands = make(commandsMap)
	}
	return nil
}

func (c *Cache) LoadVar(id, varName string) string {
	return c.variables[id][varName]
}

func (c *Cache) LoadVars(id string) map[string]string {
	return c.variables[id]
}

func (c *Cache) LoadCommands(id string) []confible.Command {
	return c.commands[id]
}

func (c *Cache) Store(cacheFilepath string) error {
	// store the new cache
	cacheFile, err := open(cacheFilepath)
	if err != nil {
		log.Fatalf("failed to open cache file: %v", err)
	}
	defer cacheFile.Close()

	encoder := gob.NewEncoder(cacheFile)
	return encoder.Encode(cacheToGob(*c))
}
