package portutil

import (
	"fmt"
	"os"
	"strconv"
)

const DefaultPort = 3847

// FromEnv reads SWITCHBOARD_PORT and returns the parsed port along with an
// optional warning. When the env var is unset the default port is returned
// with an empty warning. An invalid value also returns the default port but
// includes a human-readable warning string the caller can choose to log.
func FromEnv() (port int, warn string) {
	envPort := os.Getenv("SWITCHBOARD_PORT")
	if envPort == "" {
		return DefaultPort, ""
	}
	p, err := strconv.Atoi(envPort)
	if err != nil || p <= 0 || p >= 65536 {
		return DefaultPort, fmt.Sprintf("invalid SWITCHBOARD_PORT=%q, using default %d", envPort, DefaultPort)
	}
	return p, ""
}
