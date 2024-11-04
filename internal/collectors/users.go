package collectors

import (
	"bufio"
	"cartographer-go-agent/configuration"
	"log/slog"
	"os"
	"strings"
	"time"
)

// User struct to represent information about system users
type User struct {
	Username string `json:"username"`
	Password string `json:"-"`
	UID      string `json:"uid"`
	GID      string `json:"gid"`
	Name     string `json:"name"`
	HomeDir  string `json:"home_dir"`
	Shell    string `json:"shell"`
}

// getUsers reads the /etc/passwd file and returns a list of users
func getUsers() ([]User, error) {
	file, err := os.Open("/etc/passwd")
	if err != nil {
		return nil, err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			slog.Error("Error closing file", slog.Any("error", err))
		}
	}(file)

	var users []User

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, ":")

		user := User{
			Username: fields[0],
			Password: fields[1],
			UID:      fields[2],
			GID:      fields[3],
			Name:     fields[4],
			HomeDir:  fields[5],
			Shell:    fields[6],
		}

		users = append(users, user)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

// UsersCollector returns a collector that gathers information about system users
func UsersCollector(ttl time.Duration, config *configuration.Config) *Collector {
	return NewCollector("users", ttl, config, func(cfg *configuration.Config) (interface{}, error) {
		users, err := getUsers()
		if err != nil {
			return nil, err
		}
		return users, nil
	})
}
