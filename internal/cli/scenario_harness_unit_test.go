//go:build scenario

package cli

import (
	"fmt"
	"strings"
	"testing"
)

func TestCaptureStdoutHandlesOutputLargerThanPipeBuffer(t *testing.T) {
	want := strings.Repeat("x", 128*1024)

	got := captureStdout(t, func() {
		fmt.Print(want)
	})

	if got != want {
		t.Fatalf("captured output length = %d, want %d", len(got), len(want))
	}
}
