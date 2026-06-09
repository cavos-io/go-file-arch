package filearch

import (
	"os"
	"sync"

	"github.com/cavos-io/go-file-arch/internal/archlint/app"
)

var archLintCLIArgsMu sync.Mutex

func RunArchLintCLI(args []string) int {
	archLintCLIArgsMu.Lock()
	defer archLintCLIArgsMu.Unlock()

	previousArgs := os.Args
	defer func() {
		os.Args = previousArgs
	}()

	os.Args = append([]string{"go-arch-lint"}, args...)
	return app.Execute()
}
