package variable

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

type Variable struct {
	Exec  [][2]string `toml:"exec"`
	Input [][2]string `toml:"input"`
}

func Parse(variables []Variable) map[string]string {
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
				log.Fatalf("failed running command '%v': %v\n", cmd, err)
			}

			result[cmd[0]] = strings.TrimSpace(output.String())
		}

		// variables from input
		for _, input := range variables.Input {
			reader := bufio.NewReader(os.Stdin)
			fmt.Printf("manual input required: %q\n", input[1])
			fmt.Print("> ")
			text, err := reader.ReadString('\n')
			if err != nil {
				log.Fatalf("failed reading input: %v\n", err)
			}

			result[input[0]] = strings.TrimSpace(text)
		}
	}
	return result
}
