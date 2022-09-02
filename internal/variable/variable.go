package variable

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/sj14/confible/internal/cache"
	"github.com/sj14/confible/internal/confible"
)

func Parse(id string, variables []confible.Variable, useCached bool) (map[string]string, error) {
	result := make(cache.VariableMap)

	cacheInstance, err := cache.Load()
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

			if err := cache.AddVar(result, id, cmd.VariableName, output.String()); err != nil {
				return nil, err
			}
		}

		// variables from input
		for _, input := range variables.Input {
			cachedValue, cacheFound := cacheInstance.Variables[cache.IdVariable{id, input.VariableName}]
			if cacheFound {
				if useCached {
					if err := cache.AddVar(result, id, input.VariableName, cachedValue); err != nil {
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

			if err := cache.AddVar(result, id, input.VariableName, text); err != nil {
				return nil, err
			}
		}
	}
	return cache.OmitID(result), cache.Store(id, result)
}
