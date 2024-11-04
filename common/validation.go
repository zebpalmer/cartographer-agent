package common

import (
	"fmt"
	"os"
)

// ValidateFile checks if the given path is a file and not a directory.
func ValidateFile(path string) error {
	s, err := os.Stat(path)
	if err != nil {
		return err
	}
	if s.IsDir() {
		return fmt.Errorf("'%s' is a directory", path)
	}
	return nil
}
