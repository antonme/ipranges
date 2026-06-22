package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunShowsUsageWithoutSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "Usage: irrindex") {
		t.Fatalf("stderr = %q, want usage", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestRunRejectsUnknownSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{"bogus"}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "unknown subcommand") {
		t.Fatalf("stderr = %q, want unknown subcommand", stderr.String())
	}
	if !strings.Contains(stderr.String(), "Usage: irrindex") {
		t.Fatalf("stderr = %q, want usage", stderr.String())
	}
}

func TestRunKnownSubcommandsAreRecognizedButNotImplemented(t *testing.T) {
	for _, subcommand := range []string{"fetch", "build", "verify", "query", "compare", "whois-shim"} {
		t.Run(subcommand, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			code := run([]string{subcommand}, &stdout, &stderr)

			if code != 1 {
				t.Fatalf("exit code = %d, want 1", code)
			}
			if !strings.Contains(stderr.String(), subcommand+" is not implemented yet") {
				t.Fatalf("stderr = %q, want not implemented message", stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}
