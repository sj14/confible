package utils

import (
	"log"
	"os"
	"path/filepath"
	"strings"
)

func AbsFilepath(path string) string {
	if !strings.HasPrefix(path, "~/") {
		return path
	}

	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("failed getting home dir: %v\n", err)
	}

	return filepath.Join(home, path[2:])
}
