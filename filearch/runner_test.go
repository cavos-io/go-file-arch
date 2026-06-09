package filearch

import (
	"context"
	"testing"
)

func TestRunRequiresConfigPath(t *testing.T) {
	err := Run(context.Background(), Options{
		Patterns: []string{"./..."},
	})
	if err == nil {
		t.Fatal("Run() error = nil, want error")
	}
}
