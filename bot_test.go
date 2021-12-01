package papaBot

import (
	"testing"
)

// TestBotNewInstance tests creating a new bot instance.
func TestBotNewInstance(t *testing.T) {
	New("config/file/path", "texts/file/path")
}
