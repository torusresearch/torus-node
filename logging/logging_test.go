package logging

import (
	"os"
	"testing"
)

func TestInit(t *testing.T) {
	l := NewDefault()

	// This one should not be printed
	l.Debug("Debug!")
	l.Info("Info!")
	l.Warning("Warning!")
	l.Error("Error!")
}

func TestWithLevelString(t *testing.T) {
	l := (&logger{
		Out: os.Stdout,
	}).WithLevelString("debug")
	l.Debug("this is a debug level message")
}

func TestSetLevelString(t *testing.T) {
	SetLevelString("debug")
	Debug("now you see me!")
	SetLevelString("info")
	Debug("now you don't!")
}
