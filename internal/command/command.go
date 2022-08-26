package command

import (
	"log"
	"os"
	"os/exec"
)

type Command struct {
	Exec []string `toml:"exec"`
}

func Exec(commands []Command) {
	for _, commands := range commands {
		for _, cmd := range commands.Exec {
			c := exec.Command("sh", "-c", cmd)
			c.Stderr = os.Stderr
			c.Stdout = os.Stdout

			if err := c.Run(); err != nil {
				log.Fatalf("failed running command '%v': %v\n", cmd, err)
			}
		}
	}
}