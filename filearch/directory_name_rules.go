package filearch

import (
	"fmt"
	"path/filepath"
	"regexp"
)

type DirectoryNameRule struct {
	ID          string                   `yaml:"id"`
	Severity    string                   `yaml:"severity"`
	Directories FileSet                  `yaml:"directories"`
	Require     DirectoryNameRequirement `yaml:"require"`
	Deny        DirectoryNameRequirement `yaml:"deny"`
	Message     string                   `yaml:"message"`
}

type DirectoryNameRequirement struct {
	DirectoryName FileNameCondition `yaml:"directoryName"`
}

func checkDirectoryNameRules(cfg *Config, inventory *repositoryInventory) []ruleDiagnostic {
	var diagnostics []ruleDiagnostic
	for _, rule := range cfg.DirectoryNameRules {
		for _, directory := range inventory.matchingDirectories(rule.Directories) {
			name := filepath.Base(directory)
			message := ""
			if matchesFileNameCondition(name, rule.Deny.DirectoryName) {
				message = fmt.Sprintf("%s directory %q satisfies denied directoryName condition", rule.Message, name)
			} else if !fileNameConditionEmpty(rule.Require.DirectoryName) && !matchesFileNameCondition(name, rule.Require.DirectoryName) {
				message = fmt.Sprintf("%s directory %q does not satisfy required directoryName condition", rule.Message, name)
			}
			if message == "" {
				continue
			}
			diagnostics = append(diagnostics, ruleDiagnostic{
				Path: directory, Line: 1, Column: 1, RuleID: rule.ID, Message: message,
			})
		}
	}
	sortRuleDiagnostics(diagnostics)
	return diagnostics
}

func (cfg *Config) validateDirectoryNameRules() error {
	for i, rule := range cfg.DirectoryNameRules {
		if rule.ID == "" {
			return fmt.Errorf("directoryNameRules[%d].id is required", i)
		}
		if err := validateSeverity(rule.ID, rule.Severity); err != nil {
			return err
		}
		if len(rule.Directories.Include) == 0 {
			return fmt.Errorf("directoryName rule %q must include at least one directory pattern", rule.ID)
		}
		for _, pattern := range append(append([]string{}, rule.Directories.Include...), rule.Directories.Exclude...) {
			if err := validateRelativeGlob(pattern); err != nil {
				return fmt.Errorf("directoryName rule %q: %w", rule.ID, err)
			}
		}
		if fileNameConditionEmpty(rule.Require.DirectoryName) && fileNameConditionEmpty(rule.Deny.DirectoryName) {
			return fmt.Errorf("directoryName rule %q must configure require or deny", rule.ID)
		}
		for _, pattern := range append(append([]string{}, rule.Require.DirectoryName.Matches...), rule.Deny.DirectoryName.Matches...) {
			if _, err := regexp.Compile(pattern); err != nil {
				return fmt.Errorf("directoryName rule %q has invalid regex %q: %w", rule.ID, pattern, err)
			}
		}
		if rule.Message == "" {
			return fmt.Errorf("directoryName rule %q message is required", rule.ID)
		}
	}
	return nil
}
