package command

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"reflect"
	"runtime"

	"github.com/sj14/confible/internal/cache"
	"github.com/sj14/confible/internal/confible"
	"golang.org/x/exp/slices"
)

func Extract(cmds []confible.Command, runAfterCfgs bool) []confible.Command {
	var result []confible.Command

	for _, cmd := range cmds {
		// extract all commands which should run after configs were written
		if runAfterCfgs && cmd.AfterConfigs {
			result = append(result, cmd)
			continue
		}
		// extract all commands which should run before configs were written
		if !runAfterCfgs && !cmd.AfterConfigs {
			result = append(result, cmd)
		}
	}
	return result
}

func Exec(id string, commands []confible.Command, useCache bool, cacheFilepath string) (err error) {
	if len(commands) == 0 {
		return nil
	}

	var cacheInstance *cache.Cache
	if useCache {
		cacheInstance, err = cache.New(cacheFilepath)
		if err != nil {
			log.Fatalln(err)
		}
		cachedCommands := cacheInstance.LoadCommands(id)
		if reflect.DeepEqual(cachedCommands, commands) {
			log.Printf("[%v] commands are cached", id)
			return nil
		}
	}
	for _, commands := range commands {
		// check if we can skip those commands
		if len(commands.OSs) != 0 && !slices.Contains(commands.OSs, runtime.GOOS) {
			log.Printf("skipping as operating system %q is not matching commands filter %q\n", runtime.GOOS, commands.OSs)
			continue
		}
		if len(commands.Archs) != 0 && !slices.Contains(commands.Archs, runtime.GOARCH) {
			log.Printf("skipping as machine arch %q is not matching commands filter %q\n", runtime.GOARCH, commands.Archs)
			continue
		}

		for _, cmd := range commands.Exec {
			if err := ExecNoCache(cmd, os.Stdout); err != nil {
				return err
			}
		}
	}
	if useCache {
		cacheInstance.UpsertCommands(id, commands)
		if err := cacheInstance.Store(cacheFilepath); err != nil {
			return err
		}
	}
	return nil
}

func ExecNoCache(cmd string, stdout io.Writer) error {
	c := exec.Command("sh", "-c", cmd)

	if runtime.GOOS == "windows" {
		c = exec.Command("cmd", "/C", cmd)
	}
	c.Stderr = os.Stderr
	c.Stdout = stdout

	if err := c.Run(); err != nil {
		return fmt.Errorf("failed running command '%v': %v", cmd, err)
	}
	return nil
}
