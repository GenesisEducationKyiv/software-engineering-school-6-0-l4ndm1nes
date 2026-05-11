package platform

import "strings"

func IsTransientMailError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, s := range transientMailMarkers {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}

var transientMailMarkers = []string{
	"dial",
	"timeout",
	"connection",
	"EOF",
	"reset",
	"broken pipe",
	"temporary",
}
