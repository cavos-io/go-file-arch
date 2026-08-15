package filearch

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"

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

func (cfg *Config) localImportPath(importPath string) (string, bool) {
	target := classifyImport(importPath, cfg)
	if target.Category == importCategoryInternal || target.Category == importCategoryUnmatchedInternal {
		return target.RelativePath, true
	}
	return "", false
}

func (cfg *Config) dependencyAllowed(rule DependencyRule, targetComponent string) bool {
	if contains(rule.MayDependOn, targetComponent) {
		return true
	}
	return contains(cfg.CommonComponents, targetComponent)
}
