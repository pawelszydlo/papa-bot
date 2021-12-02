package papaBot

import (
	"testing"
)

// TestBotNewWrongFiles tests creation failing if config files are not found.
func TestBotNewWrongFiles(t *testing.T) {
	err, _ := New("config/file/path", "texts/file/path")
	if err == nil {
		t.Fatal("Bot creation should have failed.")
	}
}
