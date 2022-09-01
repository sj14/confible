package command

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/sj14/confible/internal/confible"
)

func Exec(commands []confible.Command) error {
	for _, commands := range commands {
		for _, cmd := range commands.Exec {
			c := exec.Command("sh", "-c", cmd)

			if runtime.GOOS == "windows" {
				c = exec.Command("cmd", "/C", cmd) // TODO: untested
			}
			c.Stderr = os.Stderr
			c.Stdout = os.Stdout

			if err := c.Run(); err != nil {
				return fmt.Errorf("failed running command '%v': %v", cmd, err)
			}
		}
	}
	return nil
}
