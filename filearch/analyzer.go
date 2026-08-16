package filearch

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const doc = "checks Go file content architecture rules configured by YAML"

var configPath string

var Analyzer = NewAnalyzer("")

func NewAnalyzer(defaultConfigPath string) *analysis.Analyzer {
	a := &analysis.Analyzer{
		Name: "go_file_arch",
		Doc:  doc,
		Run:  run,
	}
	a.Flags.Init("go_file_arch", flag.ExitOnError)
	a.Flags.StringVar(&configPath, "config", defaultConfigPath, "path to go-file-arch YAML config")
	return a
}

func run(pass *analysis.Pass) (any, error) {
	if configPath == "" {
		return nil, fmt.Errorf("missing -config path")
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		return nil, err
	}

	return runWithConfig(pass, cfg, nil)
}

func runWithConfig(pass *analysis.Pass, cfg *Config, inventory *repositoryInventory) (any, error) {
	for _, file := range pass.Files {
		filename := pass.Fset.Position(file.Pos()).Filename
		paths := pathCandidates(filename, cfg)
		checkComponentOptions(pass, file, cfg, relativeToWorkdir(filename, cfg))
		for _, rule := range cfg.ContentRules {
			if !fileSetAppliesToGenerated(rule.Files, file) || !ruleAppliesToPath(rule, paths) {
				continue
			}
			checkContentRule(pass, file, rule)
		}
		for _, rule := range cfg.FileNameRules {
			if !fileSetAppliesToGenerated(rule.Files, file) || !fileNameRuleAppliesToPath(rule, paths) {
				continue
			}
			checkFileNameRule(pass, file, filename, rule)
		}
		checkFileContractRules(pass, file, filename, cfg, inventory)
		checkDependencyRules(pass, file, cfg, paths)
		checkImportRules(pass, file, cfg, paths)
		checkDeclarationGroupingRules(pass, file, cfg, paths)
	}

	return nil, nil
}

func pathCandidates(filename string, cfg *Config) []string {
	var candidates []string
	add := func(path string) {
		path = filepath.ToSlash(path)
		if path == "" || path == "." {
			return
		}
		for _, existing := range candidates {
			if existing == path {
				return
			}
		}
		candidates = append(candidates, path)
	}
	addWithDir := func(path string) {
		add(path)
		dir := filepath.ToSlash(filepath.Dir(path))
		if dir != "." && dir != "/" {
			add(dir)
		}
	}

	addWithDir(filename)

	if wd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(wd, filename); err == nil {
			addWithDir(rel)
		}
	}
	if cfg.baseDir != "" {
		if rel, err := filepath.Rel(cfg.baseDir, filename); err == nil {
			addWithDir(rel)
		}
	}
	if cfg.workdirAbs != "" {
		if rel, err := filepath.Rel(cfg.workdirAbs, filename); err == nil {
			addWithDir(rel)
		}
	}

	slash := filepath.ToSlash(filename)
	if idx := strings.LastIndex(slash, "/src/"); idx >= 0 {
		addWithDir(slash[idx+len("/src/"):])
	}

	return candidates
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
