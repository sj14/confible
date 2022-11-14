package variable

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"

	"github.com/sj14/confible/internal/cache"
	"github.com/sj14/confible/internal/command"
	"github.com/sj14/confible/internal/confible"
	"golang.org/x/exp/slices"
)

func Parse(id string, variables []confible.Variable, useCached bool, cacheFilepath string) (map[string]string, error) {
	cacheInstance, err := cache.New(cacheFilepath)
	if err != nil {
		log.Fatalln(err)
	}

	for _, variables := range variables {
		// check if we can skip those variables
		if len(variables.OSs) != 0 && !slices.Contains(variables.OSs, runtime.GOOS) {
			log.Printf("skipping as operating system %q is not matching variables filter %q\n", runtime.GOOS, variables.OSs)
			continue
		}
		if len(variables.Archs) != 0 && !slices.Contains(variables.Archs, runtime.GOARCH) {
			log.Printf("skipping as machine arch %q is not matching variables filter %q\n", runtime.GOARCH, variables.Archs)
			continue
		}

		for _, cmd := range variables.Exec {
			output := &bytes.Buffer{}

			if err := command.ExecNoCache(cmd.Cmd, output); err != nil {
				return nil, err
			}

			cacheInstance.UpsertVar(id, cmd.VariableName, output.String())
		}

		// variables from input
		for _, input := range variables.Input {
			cachedValue := cacheInstance.LoadVar(id, input.VariableName)
			if cachedValue != "" {
				if useCached {
					cacheInstance.UpsertVar(id, input.VariableName, cachedValue)
					log.Printf("[%v] using cached variable %q: %q", id, input.VariableName, cachedValue)
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
			text = strings.TrimSpace(text)
			if text == "" {
				text = cachedValue
			}

			cacheInstance.UpsertVar(id, input.VariableName, text)
		}
	}
	return cacheInstance.LoadVars(id), cacheInstance.Store(cacheFilepath)
}
