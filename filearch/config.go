package filearch

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Version          int                       `yaml:"version"`
	Components       map[string]Component      `yaml:"components"`
	CommonComponents []string                  `yaml:"commonComponents"`
	DependencyRules  map[string]DependencyRule `yaml:"dependencyRules"`
	ContentRules     []ContentRule             `yaml:"contentRules"`
	FileNameRules    []FileNameRule            `yaml:"fileNameRules"`

	baseDir string
}

type Component struct {
	In []string `yaml:"in"`
}

type FileSet struct {
	Include []string `yaml:"include"`
	Exclude []string `yaml:"exclude"`
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
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %q: %w", path, err)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve config path %q: %w", path, err)
	}
	cfg.baseDir = filepath.Dir(absPath)

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
	if err := cfg.validateDependencyRules(); err != nil {
		return err
	}
	if err := cfg.validateContentRules(); err != nil {
		return err
	}
	if err := cfg.validateFileNameRules(); err != nil {
		return err
	}
	return nil
}

func (cfg *Config) validateComponents() error {
	for name, component := range cfg.Components {
		if name == "" {
			return fmt.Errorf("component name is required")
		}
		if len(component.In) == 0 {
			return fmt.Errorf("component %q must include at least one path pattern", name)
		}
	}
	for _, name := range cfg.CommonComponents {
		if _, ok := cfg.Components[name]; !ok {
			return fmt.Errorf("common component %q is not defined", name)
		}
	}
	return nil
}
