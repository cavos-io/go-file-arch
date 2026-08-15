package filearch

import (
	"fmt"
	"strings"
)

type DirectoryRule struct {
	ID          string               `yaml:"id"`
	Severity    string               `yaml:"severity"`
	Directories FileSet              `yaml:"directories"`
	Require     DirectoryRequirement `yaml:"require"`
	Message     string               `yaml:"message"`
}

type DirectoryRequirement struct {
	Files    []string `yaml:"files"`
	AnyFiles []string `yaml:"anyFiles"`
}

type ruleDiagnostic struct {
	Path    string
	Line    int
	Column  int
	RuleID  string
	Message string
}

func checkDirectoryRules(cfg *Config, inventory *repositoryInventory) []ruleDiagnostic {
	var diagnostics []ruleDiagnostic
	for _, rule := range cfg.DirectoryRules {
		for _, dir := range inventory.matchingDirectories(rule.Directories) {
			for _, pattern := range rule.Require.Files {
				if inventory.hasRelativeFile(dir, pattern) {
					continue
				}
				diagnostics = append(diagnostics, ruleDiagnostic{
					Path: dir, Line: 1, Column: 1, RuleID: rule.ID,
					Message: fmt.Sprintf("%s required file not found: %s", rule.Message, pattern),
				})
			}
			if len(rule.Require.AnyFiles) == 0 {
				continue
			}
			matched := false
			for _, pattern := range rule.Require.AnyFiles {
				if inventory.hasRelativeFile(dir, pattern) {
					matched = true
					break
				}
			}
			if !matched {
				diagnostics = append(diagnostics, ruleDiagnostic{
					Path: dir, Line: 1, Column: 1, RuleID: rule.ID,
					Message: fmt.Sprintf("%s required at least one file matching: %s", rule.Message, strings.Join(rule.Require.AnyFiles, ", ")),
				})
			}
		}
	}
	return diagnostics
}

func (cfg *Config) validateDirectoryRules() error {
	for i, rule := range cfg.DirectoryRules {
		if rule.ID == "" {
			return fmt.Errorf("directoryRules[%d].id is required", i)
		}
		if err := validateSeverity(rule.ID, rule.Severity); err != nil {
			return err
		}
		if len(rule.Directories.Include) == 0 {
			return fmt.Errorf("directory rule %q must include at least one directory pattern", rule.ID)
		}
		if rule.Message == "" {
			return fmt.Errorf("directory rule %q message is required", rule.ID)
		}
		if len(rule.Require.Files) == 0 && len(rule.Require.AnyFiles) == 0 {
			return fmt.Errorf("directory rule %q must configure files or anyFiles", rule.ID)
		}
		for _, pattern := range append(append(append([]string{}, rule.Directories.Include...), rule.Directories.Exclude...), append(rule.Require.Files, rule.Require.AnyFiles...)...) {
			if err := validateRelativeGlob(pattern); err != nil {
				return fmt.Errorf("directory rule %q: %w", rule.ID, err)
			}
		}
	}
	return nil
}
