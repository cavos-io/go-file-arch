package filearch

import (
	"fmt"
	"go/ast"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const (
	importCategoryStandard          = "standard"
	importCategoryInternal          = "internal"
	importCategoryUnmatchedInternal = "unmatched-internal"
	importCategoryExternal          = "external"
)

var validImportCategories = map[string]bool{
	importCategoryStandard:          true,
	importCategoryInternal:          true,
	importCategoryUnmatchedInternal: true,
	importCategoryExternal:          true,
}

type importTarget struct {
	Path         string
	Category     string
	Component    string
	RelativePath string
}

func classifyImport(importPath string, cfg *Config) importTarget {
	target := importTarget{Path: importPath}
	trimmed := strings.Trim(importPath, "/")
	if cfg.modulePath != "" && (trimmed == cfg.modulePath || strings.HasPrefix(trimmed, cfg.modulePath+"/")) {
		target.RelativePath = strings.TrimPrefix(strings.TrimPrefix(trimmed, cfg.modulePath), "/")
		if component, ok := cfg.matchPackageComponent(target.RelativePath); ok {
			target.Category = importCategoryInternal
			target.Component = component
		} else {
			target.Category = importCategoryUnmatchedInternal
		}
		return target
	}
	if component, ok := cfg.matchPackageComponent(trimmed); ok {
		target.Category = importCategoryInternal
		target.Component = component
		target.RelativePath = trimmed
		return target
	}
	first := trimmed
	if slash := strings.IndexByte(first, '/'); slash >= 0 {
		first = first[:slash]
	}
	if !strings.Contains(first, ".") {
		target.Category = importCategoryStandard
	} else {
		target.Category = importCategoryExternal
	}
	return target
}

func importConditionsMatch(conditions ImportConditions, target importTarget) bool {
	if target.Component != "" && contains(conditions.Components, target.Component) {
		return true
	}
	if contains(conditions.Paths, target.Path) || contains(conditions.Categories, target.Category) {
		return true
	}
	for _, prefix := range conditions.PathPrefixes {
		if strings.HasPrefix(target.Path, prefix) {
			return true
		}
	}
	for _, pattern := range conditions.PathMatches {
		if regexp.MustCompile(pattern).MatchString(target.Path) {
			return true
		}
	}
	return false
}

func importRuleViolated(rule ImportRule, target importTarget) bool {
	if importConditionsMatch(rule.Deny, target) {
		return true
	}
	return importConditionsConfigured(rule.Allow) && !importConditionsMatch(rule.Allow, target)
}

func checkImportRules(pass *analysis.Pass, file *ast.File, cfg *Config, paths []string) {
	if len(cfg.ImportRules) == 0 {
		return
	}
	for _, rule := range cfg.ImportRules {
		if len(cfg.matchFileSet(rule.Files, paths)) == 0 {
			continue
		}
		for _, spec := range file.Imports {
			importPath, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				continue
			}
			target := classifyImport(importPath, cfg)
			if !importRuleViolated(rule, target) {
				continue
			}
			component := target.Component
			if component == "" {
				component = "<none>"
			}
			pass.Reportf(spec.Pos(), "[%s]: %s import %q is not allowed; category: %s; target component: %s", rule.ID, rule.Message, target.Path, target.Category, component)
		}
	}
}

func (cfg *Config) validateImportRules() error {
	for i, rule := range cfg.ImportRules {
		if rule.ID == "" {
			return fmt.Errorf("importRules[%d].id is required", i)
		}
		if err := validateSeverity(rule.ID, rule.Severity); err != nil {
			return err
		}
		if len(rule.Files.Include) == 0 && len(rule.Files.Templates) == 0 {
			return fmt.Errorf("import rule %q must include at least one file pattern", rule.ID)
		}
		for _, pattern := range append(append([]string{}, rule.Files.Include...), rule.Files.Exclude...) {
			if err := validateRelativeGlob(pattern); err != nil {
				return fmt.Errorf("import rule %q: %w", rule.ID, err)
			}
		}
		for _, name := range rule.Files.Templates {
			if _, ok := cfg.compiledTemplates[name]; !ok {
				return fmt.Errorf("import rule %q references undefined template %q", rule.ID, name)
			}
		}
		if !importConditionsConfigured(rule.Allow) && !importConditionsConfigured(rule.Deny) {
			return fmt.Errorf("import rule %q must configure allow or deny conditions", rule.ID)
		}
		if rule.Message == "" {
			return fmt.Errorf("import rule %q message is required", rule.ID)
		}
		if err := cfg.validateImportConditions(rule.ID, "allow", rule.Allow); err != nil {
			return err
		}
		if err := cfg.validateImportConditions(rule.ID, "deny", rule.Deny); err != nil {
			return err
		}
	}
	return nil
}

func (cfg *Config) validateImportConditions(ruleID, name string, conditions ImportConditions) error {
	for _, component := range conditions.Components {
		if _, ok := cfg.Components[component]; !ok {
			return fmt.Errorf("import rule %q %s references undefined component %q", ruleID, name, component)
		}
	}
	for _, category := range conditions.Categories {
		if !validImportCategories[category] {
			return fmt.Errorf("import rule %q %s has unsupported category %q", ruleID, name, category)
		}
	}
	for _, path := range append(append([]string{}, conditions.Paths...), conditions.PathPrefixes...) {
		if path == "" || path != strings.TrimSpace(path) || strings.HasPrefix(path, "/") {
			return fmt.Errorf("import rule %q %s has invalid import path or prefix %q", ruleID, name, path)
		}
	}
	for _, pattern := range conditions.PathMatches {
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("import rule %q %s has invalid pathMatches regex %q: %w", ruleID, name, pattern, err)
		}
	}
	return nil
}

func importConditionsConfigured(conditions ImportConditions) bool {
	return len(conditions.Components) > 0 || len(conditions.Paths) > 0 || len(conditions.PathPrefixes) > 0 || len(conditions.PathMatches) > 0 || len(conditions.Categories) > 0
}
