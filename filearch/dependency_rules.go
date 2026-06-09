package filearch

import (
	"fmt"
	"go/ast"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
)

type DependencyRule struct {
	MayDependOn []string `yaml:"mayDependOn"`
}

type componentDir struct {
	dir     string
	pattern string
}

type dependencyGraph struct {
	Edges []dependencyEdge
}

type dependencyEdge struct {
	SourceComponent string
	TargetComponent string
	ImportPath      string
	Position        token.Pos
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
	graph := buildDependencyGraph(file, cfg, paths)
	for _, edge := range graph.Edges {
		rule, ok := cfg.DependencyRules[edge.SourceComponent]
		if !ok {
			continue
		}
		if cfg.dependencyAllowed(rule, edge.TargetComponent) {
			continue
		}
		pass.Reportf(
			edge.Position,
			"[dependencyRules.%s] dependency rule for component %s: %s packages may not import component %s via %q. Move dependency behind %s interface or %s implementation. detected import component: %s",
			edge.SourceComponent,
			edge.SourceComponent,
			edge.SourceComponent,
			edge.TargetComponent,
			edge.ImportPath,
			edge.SourceComponent,
			edge.TargetComponent,
			edge.TargetComponent,
		)
	}
}

func buildDependencyGraph(file *ast.File, cfg *Config, paths []string) dependencyGraph {
	sourceComponent, ok := cfg.matchAnyComponent(paths)
	if !ok {
		return dependencyGraph{}
	}

	var graph dependencyGraph
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
		graph.Edges = append(graph.Edges, dependencyEdge{
			SourceComponent: sourceComponent,
			TargetComponent: targetComponent,
			ImportPath:      importPath,
			Position:        spec.Pos(),
		})
	}
	return graph
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
	path = strings.Trim(filepath.ToSlash(path), "/")

	var best componentMatch
	if len(cfg.componentDirs) > 0 {
		for name, dirs := range cfg.componentDirs {
			for _, dir := range dirs {
				if path != strings.Trim(dir.dir, "/") {
					continue
				}
				candidate := componentMatch{
					name:         name,
					pattern:      dir.pattern,
					prefixBytes:  nonWildcardPrefixLen(dir.pattern),
					literalBytes: literalLen(dir.pattern),
					wildcards:    wildcardCount(dir.pattern),
				}
				if !best.ok || candidate.moreSpecificThan(best) {
					best = candidate
					best.ok = true
				}
			}
		}
		return best.name, best.ok
	}

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
