package filearch

import "testing"

func TestRunArchLintCLIExposesCopiedCommands(t *testing.T) {
	if code := RunArchLintCLI([]string{"version", "--output-type=json"}); code != 0 {
		t.Fatalf("RunArchLintCLI() exit code = %d, want 0", code)
	}
}
