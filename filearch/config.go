package filearch

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Version           int                       `yaml:"version"`
	Workdir           string                    `yaml:"workdir"`
	Components        map[string]Component      `yaml:"components"`
	ComponentOptions  ComponentOptions          `yaml:"componentOptions"`
	CommonComponents  []string                  `yaml:"commonComponents"`
	DependencyRules   map[string]DependencyRule `yaml:"dependencyRules"`
	ContentRules      []ContentRule             `yaml:"contentRules"`
	FileNameRules     []FileNameRule            `yaml:"fileNameRules"`
	DirectoryRules    []DirectoryRule           `yaml:"directoryRules"`
	FileContractRules []FileContractRule        `yaml:"fileContractRules"`
	PathRules         []PathRule                `yaml:"pathRules"`
	ImportRules       []ImportRule              `yaml:"importRules"`
	Templates         map[string]PathTemplate   `yaml:"templates"`

	baseDir string

	modulePath string
	workdir    string
	workdirAbs string

	componentDirs     map[string][]componentDir
	compiledTemplates map[string]compiledPathTemplate
}

type Component struct {
	In    []string `yaml:"in"`
	Files FileSet  `yaml:"files"`
}

type ComponentOptions struct {
	RequireMatch       bool `yaml:"requireMatch"`
	RequireSingleMatch bool `yaml:"requireSingleMatch"`
}

type ImportRule struct {
	ID       string           `yaml:"id"`
	Severity string           `yaml:"severity"`
	Files    FileSet          `yaml:"files"`
	Allow    ImportConditions `yaml:"allow"`
	Deny     ImportConditions `yaml:"deny"`
	Message  string           `yaml:"message"`
}

type ImportConditions struct {
	Components   []string `yaml:"components"`
	Paths        []string `yaml:"paths"`
	PathPrefixes []string `yaml:"pathPrefixes"`
	PathMatches  []string `yaml:"pathMatches"`
	Categories   []string `yaml:"categories"`
}

type FileSet struct {
	Include   []string `yaml:"include"`
	Exclude   []string `yaml:"exclude"`
	Templates []string `yaml:"templates"`
}

type DeclarationSet struct {
	Declarations []string `yaml:"declarations"`
}

var validDeclarationKinds = map[string]bool{
	"package":   true,
	"import":    true,
	"interface": true,
	"struct":    true,
	"func":      true,
	"var":       true,
	"const":     true,
	"type":      true,
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}

	var cfg Config
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse config %q: %w", path, err)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve config path %q: %w", path, err)
	}
	cfg.baseDir = filepath.Dir(absPath)
	cfg.workdir = filepath.ToSlash(strings.Trim(cfg.Workdir, "/"))
	cfg.workdirAbs = cfg.baseDir
	if cfg.workdir != "" && cfg.workdir != "." {
		cfg.workdirAbs = filepath.Join(cfg.baseDir, filepath.FromSlash(cfg.workdir))
	}
	cfg.modulePath = readModulePath(cfg.baseDir)
	if cfg.modulePath == "" {
		cfg.modulePath = readModulePath(cfg.workdirAbs)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config %q: %w", path, err)
	}

	return &cfg, nil
}

func (cfg *Config) validate() error {
	if cfg.Version != 1 {
		return fmt.Errorf("version must be 1")
	}
	if err := cfg.validateComponents(); err != nil {
		return err
	}
	if err := cfg.validateComponentOptions(); err != nil {
		return err
	}
	if err := cfg.validateTemplates(); err != nil {
		return err
	}
	if err := cfg.validateDependencyRules(); err != nil {
		return err
	}
	if err := cfg.validateContentRules(); err != nil {
		return err
	}
	if err := cfg.validateFileNameRules(); err != nil {
		return err
	}
	if err := cfg.validateDirectoryRules(); err != nil {
		return err
	}
	if err := cfg.validateFileContractRules(); err != nil {
		return err
	}
	if err := cfg.validatePathRules(); err != nil {
		return err
	}
	return cfg.validateUniqueRuleIDs()
}

func validateSeverity(ruleID, severity string) error {
	if severity == "" || severity == "error" {
		return nil
	}
	return fmt.Errorf("rule %q has unsupported severity %q; only error is supported", ruleID, severity)
}

func (cfg *Config) validateUniqueRuleIDs() error {
	seen := make(map[string]bool)
	check := func(id string) error {
		if seen[id] {
			return fmt.Errorf("duplicate rule id %q", id)
		}
		seen[id] = true
		return nil
	}
	for _, rule := range cfg.ContentRules {
		if err := check(rule.ID); err != nil {
			return err
		}
	}
	for _, rule := range cfg.FileNameRules {
		if err := check(rule.ID); err != nil {
			return err
		}
	}
	for _, rule := range cfg.DirectoryRules {
		if err := check(rule.ID); err != nil {
			return err
		}
	}
	for _, rule := range cfg.FileContractRules {
		if err := check(rule.ID); err != nil {
			return err
		}
	}
	for _, rule := range cfg.PathRules {
		if err := check(rule.ID); err != nil {
			return err
		}
	}
	for _, rule := range cfg.ImportRules {
		if err := check(rule.ID); err != nil {
			return err
		}
	}
	return nil
}

func readModulePath(baseDir string) string {
	data, err := os.ReadFile(filepath.Join(baseDir, "go.mod"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}

func (cfg *Config) validateComponents() error {
	cfg.componentDirs = make(map[string][]componentDir)
	for name, component := range cfg.Components {
		if name == "" {
			return fmt.Errorf("component name is required")
		}
		if len(component.In) == 0 && len(component.Files.Include) == 0 {
			return fmt.Errorf("component %q must include at least one directory or file pattern", name)
		}
		for _, pattern := range append(append([]string{}, component.Files.Include...), component.Files.Exclude...) {
			if err := validateRelativeGlob(pattern); err != nil {
				return fmt.Errorf("component %q: %w", name, err)
			}
		}
		for _, pattern := range component.In {
			matches := cfg.resolveComponentDirectories(pattern)
			if len(matches) == 0 {
				return fmt.Errorf("not found directories for %q in %q", pattern, filepath.Join(cfg.workdirAbs, filepath.FromSlash(pattern)))
			}
			cfg.componentDirs[name] = append(cfg.componentDirs[name], matches...)
		}
	}
	for _, name := range cfg.CommonComponents {
		if _, ok := cfg.Components[name]; !ok {
			return fmt.Errorf("common component %q is not defined", name)
		}
	}
	return nil
}

func (cfg *Config) resolveComponentDirectories(pattern string) []componentDir {
	var matches []componentDir
	err := filepath.WalkDir(cfg.workdirAbs, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(cfg.workdirAbs, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if matchGlob(rel, filepath.ToSlash(pattern)) {
			matches = append(matches, componentDir{
				dir:     rel,
				pattern: filepath.ToSlash(pattern),
			})
		}
		return nil
	})
	if err != nil {
		return nil
	}
	return matches
}
