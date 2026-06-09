package filearch

import (
	"path/filepath"
	"strings"
)

func MatchesAnyGlob(path string, patterns []string) bool {
	path = filepath.ToSlash(path)
	for _, pattern := range patterns {
		if matchGlob(path, filepath.ToSlash(pattern)) {
			return true
		}
	}
	return false
}

func matchGlob(path, pattern string) bool {
	path = strings.Trim(path, "/")
	pattern = strings.Trim(pattern, "/")
	if path == "" || pattern == "" {
		return path == pattern
	}
	return matchSegments(strings.Split(path, "/"), strings.Split(pattern, "/"))
}

func matchSegments(path, pattern []string) bool {
	if len(pattern) == 0 {
		return len(path) == 0
	}
	if pattern[0] == "**" {
		if matchSegments(path, pattern[1:]) {
			return true
		}
		return len(path) > 0 && matchSegments(path[1:], pattern)
	}
	if len(path) == 0 {
		return false
	}
	ok, err := filepath.Match(pattern[0], path[0])
	if err != nil || !ok {
		return false
	}
	return matchSegments(path[1:], pattern[1:])
}
