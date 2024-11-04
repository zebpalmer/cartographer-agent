package common

import (
	"math/rand"
	"time"
)

// RandomInt returns a random integer between min and max (inclusive).
func RandomInt(min, max int) int {
	// Generate a random number between min and max (inclusive):
	// rand.Intn(max-min+1) gives a number between 0 and (max-min),
	// adding min shifts the range to [min, max].
	return rand.Intn(max-min+1) + min
}

// RandomSleep pauses execution for a random duration between min and max seconds.
func RandomSleep(minSeconds, maxSeconds int) {
	// Get a random number of seconds between minSeconds and maxSeconds
	randomSeconds := RandomInt(minSeconds, maxSeconds)

	// Sleep for the random duration
	time.Sleep(time.Duration(randomSeconds) * time.Second)
}
