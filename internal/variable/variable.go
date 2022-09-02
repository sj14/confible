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

			cacheInstance.UpsertVar(id, cmd.VariableName, output.String())
		}

		// variables from input
		for _, input := range variables.Input {
			cachedValue := cacheInstance.LoadVar(id, input.VariableName)
			if cachedValue != "" {
				if useCached {
					cacheInstance.UpsertVar(id, input.VariableName, cachedValue)
					continue
				}
			}

			reader := bufio.NewReader(os.Stdin)
			fmt.Printf("manual input required: %q\n", input.Prompt)
			if cachedValue != "" {
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

			cacheInstance.UpsertVar(id, input.VariableName, text)
		}
	}
	return cacheInstance.LoadVars(id), cacheInstance.Store()
}
