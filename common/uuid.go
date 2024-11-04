package common

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

// UUIDFilePath defines the path where the UUID will be stored
const UUIDFilePath = "/var/lib/cartographer-agent/agent-uuid" // Adjust path if needed

// GetOrCreateUUID retrieves a UUID from a file or creates one if it doesn't exist.
func GetOrCreateUUID() (string, error) {
	// Check if the UUID file exists
	if _, err := os.Stat(UUIDFilePath); os.IsNotExist(err) {
		// File does not exist, create a new UUID
		newUUID := uuid.New().String()

		// Ensure the directory exists
		dir := filepath.Dir(UUIDFilePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("failed to create directory: %v", err)
		}

		// Write the UUID to the file
		if err := os.WriteFile(UUIDFilePath, []byte(newUUID), 0644); err != nil {
			return "", fmt.Errorf("failed to write UUID to file: %v", err)
		}

		return newUUID, nil
	}

	// File exists, read the UUID
	contents, err := os.ReadFile(UUIDFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to read UUID from file: %v", err)
	}

	return string(contents), nil
}
