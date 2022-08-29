package variable

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type Variable struct {
	Exec  [][2]string `toml:"exec"`
	Input [][2]string `toml:"input"`
}

func addVar(varMap map[string]string, key, value string) error {
	if _, ok := varMap[key]; ok {
		return fmt.Errorf("variable %q already exists", key)
	}

	varMap[key] = strings.TrimSpace(value)
	return nil
}

func Parse(variables []Variable) (map[string]string, error) {
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
				return nil, fmt.Errorf("failed running command '%v': %v", cmd, err)
			}

			if err := addVar(result, cmd[0], output.String()); err != nil {
				return nil, err
			}
		}

		// variables from input
		for _, input := range variables.Input {
			reader := bufio.NewReader(os.Stdin)
			fmt.Printf("manual input required: %q\n", input[1])
			fmt.Print("> ")
			text, err := reader.ReadString('\n')
			if err != nil {
				return nil, fmt.Errorf("failed reading input: %v", err)
			}

			if err := addVar(result, input[0], text); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}
