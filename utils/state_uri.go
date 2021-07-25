package utils

import (
	"strings"
)

func IsLocalStateURI(stateURI string) bool {
	stateURIParts := strings.Split(stateURI, "/")
	if len(stateURIParts) == 0 {
		return false
	}
	hostnameParts := strings.Split(stateURIParts[0], ".")
	if len(hostnameParts) == 0 {
		return false
	}
	return hostnameParts[len(hostnameParts)-1] == "local"
}
