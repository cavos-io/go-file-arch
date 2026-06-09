package filearch

import "testing"

func TestMatchesAnyGlob(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		patterns []string
		want     bool
	}{
		{
			name:     "double star go files",
			path:     "core/user/repository.go",
			patterns: []string{"**/*.go"},
			want:     true,
		},
		{
			name:     "repository under core",
			path:     "core/user/repository.go",
			patterns: []string{"core/**/repository.go"},
			want:     true,
		},
		{
			name:     "model subtree",
			path:     "core/user/model/user.go",
			patterns: []string{"core/**/model/**/*.go"},
			want:     true,
		},
		{
			name:     "test file",
			path:     "core/user/repository_test.go",
			patterns: []string{"**/*_test.go"},
			want:     true,
		},
		{
			name:     "no match",
			path:     "adapter/user/repository.go",
			patterns: []string{"core/**/repository.go"},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchesAnyGlob(tt.path, tt.patterns); got != tt.want {
				t.Fatalf("MatchesAnyGlob(%q, %v) = %v, want %v", tt.path, tt.patterns, got, tt.want)
			}
		})
	}
}
