package filearch

import (
	"fmt"
	"go/ast"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/analysis"
)

type componentMatch struct {
	name         string
	pattern      string
	prefixBytes  int
	literalBytes int
	wildcards    int
	ok           bool
}

func (cfg *Config) componentMatches(path string) []componentMatch {
	path = strings.Trim(filepath.ToSlash(path), "/")
	dir := strings.Trim(filepath.ToSlash(filepath.Dir(path)), "/")
	if dir == "." {
		dir = ""
	}

	var matches []componentMatch
	for name, component := range cfg.Components {
		var best componentMatch
		consider := func(pattern string) {
			candidate := componentMatch{
				name:         name,
				pattern:      pattern,
				prefixBytes:  nonWildcardPrefixLen(pattern),
				literalBytes: literalLen(pattern),
				wildcards:    wildcardCount(pattern),
				ok:           true,
			}
			if !best.ok || candidate.moreSpecificThan(best) {
				best = candidate
			}
		}

		if !MatchesAnyGlob(path, component.Files.Exclude) && MatchesAnyGlob(path, component.Files.Include) {
			for _, pattern := range component.Files.Include {
				if matchGlob(path, filepath.ToSlash(pattern)) {
					consider(pattern)
				}
			}
		}

		if len(cfg.componentDirs) > 0 {
			for _, componentDir := range cfg.componentDirs[name] {
				matchedDir := strings.Trim(componentDir.dir, "/")
				if path == matchedDir || dir == matchedDir {
					consider(componentDir.pattern)
				}
			}
		} else {
			for _, pattern := range component.In {
				if matchGlob(path, filepath.ToSlash(pattern)) || (dir != "" && matchGlob(dir, filepath.ToSlash(pattern))) {
					consider(pattern)
				}
			}
		}

		if best.ok {
			matches = append(matches, best)
		}
	}

	sort.Slice(matches, func(i, j int) bool { return matches[i].name < matches[j].name })
	return matches
}

func (cfg *Config) matchAnyComponent(paths []string) (string, bool) {
	for _, path := range paths {
		component, ok := cfg.matchComponent(path)
		if ok {
			return component, true
		}
	}
	return "", false
}

func (cfg *Config) matchComponent(path string) (string, bool) {
	var best componentMatch
	for _, match := range cfg.componentMatches(path) {
		if !best.ok || match.moreSpecificThan(best) {
			best = match
		}
	}
	return best.name, best.ok
}

func (match componentMatch) moreSpecificThan(other componentMatch) bool {
	if match.prefixBytes != other.prefixBytes {
		return match.prefixBytes > other.prefixBytes
	}
	if match.literalBytes != other.literalBytes {
		return match.literalBytes > other.literalBytes
	}
	if match.wildcards != other.wildcards {
		return match.wildcards < other.wildcards
	}
	if len(match.pattern) != len(other.pattern) {
		return len(match.pattern) > len(other.pattern)
	}
	return match.name < other.name
}

func nonWildcardPrefixLen(pattern string) int {
	idx := strings.IndexAny(pattern, "*?[")
	if idx < 0 {
		return len(pattern)
	}
	return idx
}

func literalLen(pattern string) int {
	count := 0
	for _, char := range pattern {
		switch char {
		case '*', '?', '[', ']':
			continue
		default:
			count++
		}
	}
	return count
}

func wildcardCount(pattern string) int {
	count := 0
	for _, char := range pattern {
		switch char {
		case '*', '?', '[':
			count++
		}
	}
	return count
}

func checkComponentOptions(pass *analysis.Pass, file *ast.File, cfg *Config, path string) {
	matches := cfg.componentMatches(path)
	if cfg.ComponentOptions.RequireMatch && len(matches) == 0 {
		pass.Reportf(file.Package, "[componentOptions.requireMatch]: file does not match any component")
	}
	if cfg.ComponentOptions.RequireSingleMatch && len(matches) > 1 {
		names := make([]string, len(matches))
		for i, match := range matches {
			names[i] = match.name
		}
		pass.Reportf(file.Package, "[componentOptions.requireSingleMatch]: file matches multiple components: %s", strings.Join(names, ", "))
	}
}

func (cfg *Config) validateComponentOptions() error {
	if cfg.ComponentOptions.RequireSingleMatch && len(cfg.Components) == 0 {
		return fmt.Errorf("componentOptions.requireSingleMatch requires components")
	}
	return nil
}
