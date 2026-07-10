package internal

import (
	"bytes"
	"os"
	"testing"
)

func captureStdout(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	f()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

func TestPrintUsage(t *testing.T) {
	output := captureStdout(func() {
		PrintUsage()
	})
	if len(output) == 0 {
		t.Error("PrintUsage did not print anything")
	}
}
