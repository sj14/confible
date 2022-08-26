package main

import (
	"log"
	"os"
	"os/exec"
)

type command struct {
	Exec []string `toml:"exec"`
}

func execCmds(commands []command) {
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
