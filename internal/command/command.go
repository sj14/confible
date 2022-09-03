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
)

func Exec(id string, commands []confible.Command, useCache bool) error {
	if len(commands) == 0 {
		return nil
	}
	cacheInstance, err := cache.Load()
	if err != nil {
		log.Fatalln(err)
	}
	if useCache {
		cmds := cacheInstance.LoadCommands(id)
		if reflect.DeepEqual(cmds, commands) {
			log.Printf("[%v] commands are cached", id)
			return nil
		}
	}
	for _, commands := range commands {
		for _, cmd := range commands.Exec {
			if err := ExecNoCache(cmd, os.Stdout); err != nil {
				return err
			}
		}
	}
	cacheInstance.UpsertCommands(id, commands)
	return cacheInstance.Store()
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
