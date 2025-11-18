package util

import (
	"fmt"
)

// Progress prints a progress message to the console.
// It uses a carriage return to update the line in place.
// A newline is printed when done == total.
func Progress(msg string, done, total int) {
	if total == 0 { // Avoid division by zero if total is 0
		fmt.Printf("\r%s [ %d / %d ]", msg, done, total)
		if done >= total {
			fmt.Println()
		}
		return
	}
	percent := float64(done) / float64(total) * 100
	fmt.Printf("\r%s [ %d / %d (%.0f%%) ]", msg, done, total, percent)
	if done >= total {
		fmt.Println()
	}
}
