package util

import (
	"log"
)

// Check logs a fatal error and exits if err is not nil.
func Check(err error) {
	if err != nil {
		log.Fatalf("%v", err)
	}
}
