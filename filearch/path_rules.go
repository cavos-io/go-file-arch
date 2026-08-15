package filearch

import (
	"fmt"
	"path/filepath"
)

type PathRule struct {
	ID          string  `yaml:"id"`
	Severity    string  `yaml:"severity"`
	Directories FileSet `yaml:"directories"`
	Depth       string  `yaml:"depth"`
	Require     PathSet `yaml:"require"`
	Allow       PathSet `yaml:"allow"`
	Deny        PathSet `yaml:"deny"`
	Message     string  `yaml:"message"`
}

type PathSet struct {
	Files       []string `yaml:"files"`
	Directories []string `yaml:"directories"`
}

func checkPathRules(cfg *Config, inventory *repositoryInventory) []ruleDiagnostic {
	var diagnostics []ruleDiagnostic
	for _, rule := range cfg.PathRules {
		for _, base := range inventory.matchingDirectories(rule.Directories) {
			files, directories := inventory.relativeEntries(base, rule.Depth)
			diagnostics = append(diagnostics, requiredPathDiagnostics(rule, base, "file", files, rule.Require.Files)...)
			diagnostics = append(diagnostics, requiredPathDiagnostics(rule, base, "directory", directories, rule.Require.Directories)...)
			diagnostics = append(diagnostics, inventoryPathDiagnostics(rule, base, "file", files, rule.Allow.Files, rule.Deny.Files)...)
			diagnostics = append(diagnostics, inventoryPathDiagnostics(rule, base, "directory", directories, rule.Allow.Directories, rule.Deny.Directories)...)
		}
	}
	sortRuleDiagnostics(diagnostics)
	return diagnostics
}

func requiredPathDiagnostics(rule PathRule, base, kind string, entries, patterns []string) []ruleDiagnostic {
	var diagnostics []ruleDiagnostic
	for _, pattern := range patterns {
		if MatchesAnyPathGlob(entries, []string{pattern}) {
			continue
		}
		diagnostics = append(diagnostics, ruleDiagnostic{
			Path: base, Line: 1, Column: 1, RuleID: rule.ID,
			Message: fmt.Sprintf("%s required %s not found: %s", rule.Message, kind, pattern),
		})
	}
	return diagnostics
}

func inventoryPathDiagnostics(rule PathRule, base, kind string, entries, allowed, denied []string) []ruleDiagnostic {
	var diagnostics []ruleDiagnostic
	for _, entry := range entries {
		isDenied := MatchesAnyGlob(entry, denied)
		if isDenied {
			diagnostics = append(diagnostics, ruleDiagnostic{
				Path: joinedInventoryPath(base, entry), Line: 1, Column: 1, RuleID: rule.ID,
				Message: fmt.Sprintf("%s denied %s: %s", rule.Message, kind, entry),
			})
			continue
		}
		if len(allowed) > 0 && !MatchesAnyGlob(entry, allowed) {
			diagnostics = append(diagnostics, ruleDiagnostic{
				Path: joinedInventoryPath(base, entry), Line: 1, Column: 1, RuleID: rule.ID,
				Message: fmt.Sprintf("%s %s is not allowed: %s", rule.Message, kind, entry),
			})
		}
	}
	return diagnostics
}

func joinedInventoryPath(base, relative string) string {
	if base == "." || base == "" {
		return filepath.ToSlash(relative)
	}
	return filepath.ToSlash(filepath.Join(base, relative))
}

func (cfg *Config) validatePathRules() error {
	for i, rule := range cfg.PathRules {
		if rule.ID == "" {
			return fmt.Errorf("pathRules[%d].id is required", i)
		}
		if err := validateSeverity(rule.ID, rule.Severity); err != nil {
			return err
		}
		if len(rule.Directories.Include) == 0 {
			return fmt.Errorf("path rule %q must include at least one directory pattern", rule.ID)
		}
		if rule.Depth != "" && rule.Depth != "direct" && rule.Depth != "recursive" {
			return fmt.Errorf("path rule %q has unsupported depth %q", rule.ID, rule.Depth)
		}
		if rule.Message == "" {
			return fmt.Errorf("path rule %q message is required", rule.ID)
		}
		patterns := append([]string{}, rule.Directories.Include...)
		patterns = append(patterns, rule.Directories.Exclude...)
		for _, set := range []PathSet{rule.Require, rule.Allow, rule.Deny} {
			patterns = append(patterns, set.Files...)
			patterns = append(patterns, set.Directories...)
		}
		if len(patterns) == len(rule.Directories.Include)+len(rule.Directories.Exclude) {
			return fmt.Errorf("path rule %q must configure require, allow, or deny", rule.ID)
		}
		for _, pattern := range patterns {
			if err := validateRelativeGlob(pattern); err != nil {
				return fmt.Errorf("path rule %q: %w", rule.ID, err)
			}
		}
	}
	return nil
}
