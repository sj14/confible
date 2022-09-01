package utils

import (
	"log"
	"os"
	"path/filepath"
	"strings"
)

func AbsFilepath(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}

	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("failed getting home dir: %v\n", err)
	}

	return filepath.Join(home, path[1:])
}

func GetEnvMap() map[string]string {
	envMap := make(map[string]string)

	for _, environ := range os.Environ() {
		keyValue := strings.SplitN(environ, "=", 2)
		envMap[keyValue[0]] = keyValue[1]
	}

	return envMap
}
