package filearch

import (
	"fmt"
	"go/ast"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
)

type DependencyRule struct {
	MayDependOn []string `yaml:"mayDependOn"`
}

func (cfg *Config) validateDependencyRules() error {
	for component := range cfg.DependencyRules {
		if _, ok := cfg.Components[component]; !ok {
			return fmt.Errorf("dependency rule references undefined component %q", component)
		}
		for _, dependency := range cfg.DependencyRules[component].MayDependOn {
			if _, ok := cfg.Components[dependency]; !ok {
				return fmt.Errorf("dependency rule for %q references undefined component %q", component, dependency)
			}
		}
	}
	return nil
}

func checkDependencyRules(pass *analysis.Pass, file *ast.File, cfg *Config, paths []string) {
	sourceComponent, ok := cfg.matchAnyComponent(paths)
	if !ok {
		return
	}

	rule, ok := cfg.DependencyRules[sourceComponent]
	if !ok {
		return
	}

	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		targetPath, ok := cfg.localImportPath(importPath)
		if !ok {
			continue
		}
		targetComponent, ok := cfg.matchComponent(targetPath)
		if !ok {
			continue
		}
		if cfg.dependencyAllowed(rule, targetComponent) {
			continue
		}
		pass.Reportf(
			spec.Pos(),
			"[dependencyRules.%s] dependency rule for component %s: %s packages may not import component %s. Move dependency behind %s interface or %s implementation. detected import component: %s",
			sourceComponent,
			sourceComponent,
			sourceComponent,
			targetComponent,
			sourceComponent,
			targetComponent,
			targetComponent,
		)
	}
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
	for name, component := range cfg.Components {
		for _, pattern := range component.In {
			if !MatchesAnyGlob(path, []string{pattern}) {
				continue
			}
			candidate := componentMatch{
				name:         name,
				pattern:      pattern,
				prefixBytes:  nonWildcardPrefixLen(pattern),
				literalBytes: literalLen(pattern),
				wildcards:    wildcardCount(pattern),
			}
			if !best.ok || candidate.moreSpecificThan(best) {
				best = candidate
				best.ok = true
			}
		}
	}
	return best.name, best.ok
}

type componentMatch struct {
	name         string
	pattern      string
	prefixBytes  int
	literalBytes int
	wildcards    int
	ok           bool
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

func (cfg *Config) localImportPath(importPath string) (string, bool) {
	importPath = strings.Trim(importPath, "/")
	if cfg.modulePath != "" {
		if importPath == cfg.modulePath {
			return "", true
		}
		if strings.HasPrefix(importPath, cfg.modulePath+"/") {
			return strings.TrimPrefix(importPath, cfg.modulePath+"/"), true
		}
	}
	if _, ok := cfg.matchComponent(importPath); ok {
		return importPath, true
	}
	return "", false
}

func (cfg *Config) dependencyAllowed(rule DependencyRule, targetComponent string) bool {
	if contains(rule.MayDependOn, targetComponent) {
		return true
	}
	return contains(cfg.CommonComponents, targetComponent)
}
