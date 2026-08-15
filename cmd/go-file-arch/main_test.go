package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRunCLIExitCodes(t *testing.T) {
	tests := []struct {
		name       string
		config     string
		source     string
		wantExit   int
		wantStderr string
	}{
		{"pass", "version: 1\n", "package fixture\n", 0, ""},
		{"violation", "version: 1\ncontentRules:\n- id: no-functions\n  files: {include: ['**/*.go']}\n  deny: {declarations: [func]}\n  message: functions denied\n", "package fixture\nfunc Feature() {}\n", 1, "architecture violation"},
		{"invalid config", "version: 2\n", "package fixture\n", 2, "version must be 1"},
		{"malformed Go", "version: 1\n", "package fixture\nfunc {\n", 2, "parse "},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			writeCLIFile(t, dir, "go.mod", "module example.com/fixture\n\ngo 1.25\n")
			writeCLIFile(t, dir, "policy.yml", test.config)
			writeCLIFile(t, dir, "fixture.go", test.source)
			var stdout, stderr bytes.Buffer
			got := runCLI([]string{"--config", filepath.Join(dir, "policy.yml"), "--workdir", dir, "./..."}, &stdout, &stderr)
			if got != test.wantExit {
				t.Fatalf("runCLI() = %d, want %d; stderr=%s", got, test.wantExit, stderr.String())
			}
			if test.wantStderr != "" && !bytes.Contains(stderr.Bytes(), []byte(test.wantStderr)) {
				t.Fatalf("stderr = %q, want %q", stderr.String(), test.wantStderr)
			}
		})
	}
}

func TestRunCLIShortWorkdirFlag(t *testing.T) {
	dir := t.TempDir()
	writeCLIFile(t, dir, "go.mod", "module example.com/fixture\n\ngo 1.25\n")
	writeCLIFile(t, dir, "policy.yml", "version: 1\n")
	writeCLIFile(t, dir, "fixture.go", "package fixture\n")
	var stdout, stderr bytes.Buffer
	if got := runCLI([]string{"-c", filepath.Join(dir, "policy.yml"), "-C", dir, "./..."}, &stdout, &stderr); got != 0 {
		t.Fatalf("runCLI() = %d, stderr=%s", got, stderr.String())
	}
}

func TestRunCLIPackageFailureExitCode(t *testing.T) {
	dir := t.TempDir()
	writeCLIFile(t, dir, "go.mod", "module example.com/fixture\n\ngo 1.25\n")
	writeCLIFile(t, dir, "policy.yml", "version: 1\n")
	var stdout, stderr bytes.Buffer
	got := runCLI([]string{"-c", filepath.Join(dir, "policy.yml"), "-C", dir, "./missing/..."}, &stdout, &stderr)
	if got != 2 || !bytes.Contains(stderr.Bytes(), []byte("package loading failed")) {
		t.Fatalf("runCLI() = %d, stderr=%q; want execution failure", got, stderr.String())
	}
}

func TestRunCLIHelpSucceedsOnStdout(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if got := runCLI([]string{"--help"}, &stdout, &stderr); got != 0 || !bytes.Contains(stdout.Bytes(), []byte("Exit codes:")) {
		t.Fatalf("runCLI(--help) = %d, stdout=%q, stderr=%q", got, stdout.String(), stderr.String())
	}
}

func writeCLIFile(t *testing.T, dir, name, contents string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}
