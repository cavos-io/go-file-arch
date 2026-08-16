package filearch

import (
	"fmt"
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

type DeclarationGroupingRule struct {
	ID                   string              `yaml:"id"`
	Severity             string              `yaml:"severity"`
	Files                FileSet             `yaml:"files"`
	Declaration          DeclarationSelector `yaml:"declaration"`
	SeparateWhenCount    CountCondition      `yaml:"separateWhenCount"`
	SingleGroupWhenCount CountCondition      `yaml:"singleGroupWhenCount"`
	Message              string              `yaml:"message"`
}

func (cfg *Config) validateDeclarationGroupingRules() error {
	for i, rule := range cfg.DeclarationGroupingRules {
		if rule.ID == "" {
			return fmt.Errorf("declarationGroupingRules[%d].id is required", i)
		}
		if err := validateSeverity(rule.ID, rule.Severity); err != nil {
			return err
		}
		if len(rule.Files.Include) == 0 && len(rule.Files.Templates) == 0 {
			return fmt.Errorf("declaration grouping rule %q must include at least one file pattern", rule.ID)
		}
		for _, pattern := range append(append([]string{}, rule.Files.Include...), rule.Files.Exclude...) {
			if err := validateRelativeGlob(pattern); err != nil {
				return fmt.Errorf("declaration grouping rule %q: %w", rule.ID, err)
			}
		}
		for _, name := range rule.Files.Templates {
			compiled, ok := cfg.compiledTemplates[name]
			if !ok {
				return fmt.Errorf("declaration grouping rule %q references undefined template %q", rule.ID, name)
			}
			captures := make(map[string]string, len(compiled.captures))
			for _, capture := range compiled.captures {
				captures[capture] = "example"
			}
			if err := validateDeclarationGroupingExpansion(rule, captures); err != nil {
				return err
			}
		}
		if len(rule.Files.Include) > 0 {
			if err := validateDeclarationGroupingExpansion(rule, map[string]string{}); err != nil {
				return err
			}
		}
		if err := validateDeclarationSelector(rule.ID, rule.Declaration); err != nil {
			return err
		}
		if countConfigured(rule.Declaration.Count) {
			return fmt.Errorf("declaration grouping rule %q: declaration count is not allowed", rule.ID)
		}
		if !countConfigured(rule.SeparateWhenCount) && !countConfigured(rule.SingleGroupWhenCount) {
			return fmt.Errorf("declaration grouping rule %q must configure a count threshold", rule.ID)
		}
		if err := validateCount(rule.ID, "separateWhenCount", rule.SeparateWhenCount); err != nil {
			return err
		}
		if err := validateCount(rule.ID, "singleGroupWhenCount", rule.SingleGroupWhenCount); err != nil {
			return err
		}
		if countConfigured(rule.SeparateWhenCount) && countConfigured(rule.SingleGroupWhenCount) && countConditionsOverlap(rule.SeparateWhenCount, rule.SingleGroupWhenCount) {
			return fmt.Errorf("declaration grouping rule %q count thresholds overlap", rule.ID)
		}
		if rule.Message == "" {
			return fmt.Errorf("declaration grouping rule %q message is required", rule.ID)
		}
	}
	return nil
}

func validateDeclarationGroupingExpansion(rule DeclarationGroupingRule, captures map[string]string) error {
	if _, err := expandDeclarationSelectorChecked(rule.Declaration, captures); err != nil {
		return fmt.Errorf("declaration grouping rule %q has invalid declaration template expansion: %w", rule.ID, err)
	}
	return nil
}

func countConditionsOverlap(first, second CountCondition) bool {
	firstMin, firstMax := countBounds(first)
	secondMin, secondMax := countBounds(second)
	return firstMin <= secondMax && secondMin <= firstMax
}

func countBounds(condition CountCondition) (int, int) {
	if condition.Equals != nil {
		return *condition.Equals, *condition.Equals
	}
	minimum := 0
	maximum := int(^uint(0) >> 1)
	if condition.Min != nil {
		minimum = *condition.Min
	}
	if condition.Max != nil {
		maximum = *condition.Max
	}
	return minimum, maximum
}

func checkDeclarationGroupingRules(pass *analysis.Pass, file *ast.File, cfg *Config, paths []string) {
	candidates := extractDeclarationCandidates(pass.Fset, file)
	for _, rule := range cfg.DeclarationGroupingRules {
		if !fileSetAppliesToGenerated(rule.Files, file) {
			continue
		}
		matches := cfg.matchFileSet(rule.Files, paths)
		if len(matches) == 0 {
			continue
		}
		seen := make(map[string]bool)
		for _, match := range matches {
			selector := expandDeclarationSelector(rule.Declaration, match.Captures)
			selected := matchingDeclarationCandidates(candidates, selector)
			if countConfigured(rule.SeparateWhenCount) && countMatches(len(selected), rule.SeparateWhenCount, false) {
				for _, candidate := range selected {
					if candidate.Grouped {
						reportGroupingViolation(pass, rule, candidate, "must use a separate declaration", seen)
					}
				}
			}
			if countConfigured(rule.SingleGroupWhenCount) && countMatches(len(selected), rule.SingleGroupWhenCount, false) {
				checkSingleDeclarationGroup(pass, rule, selected, seen)
			}
		}
	}
}

func checkSingleDeclarationGroup(pass *analysis.Pass, rule DeclarationGroupingRule, selected []declarationCandidate, seen map[string]bool) {
	if len(selected) == 0 {
		return
	}
	groupID := selected[0].GroupID
	valid := groupID.IsValid() && selected[0].Grouped && selected[0].GroupSize == len(selected)
	for _, candidate := range selected[1:] {
		valid = valid && candidate.Grouped && candidate.GroupID == groupID
	}
	if valid {
		return
	}

	reportedGroups := make(map[token.Pos]bool)
	for _, candidate := range selected {
		if candidate.Grouped && candidate.GroupID.IsValid() {
			if reportedGroups[candidate.GroupID] {
				continue
			}
			reportedGroups[candidate.GroupID] = true
		}
		reportGroupingViolation(pass, rule, candidate, "must share one declaration group containing only selected declarations", seen)
	}
}

func reportGroupingViolation(pass *analysis.Pass, rule DeclarationGroupingRule, candidate declarationCandidate, reason string, seen map[string]bool) {
	key := fmt.Sprintf("%d:%s", candidate.Pos, reason)
	if seen[key] {
		return
	}
	seen[key] = true
	pass.Reportf(candidate.Pos, "[%s]: %s declaration %s %s", rule.ID, rule.Message, candidate.Name, reason)
}
