package filearch

import (
	"fmt"
	"go/ast"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"
)

type FileContractRule struct {
	ID       string                  `yaml:"id"`
	Severity string                  `yaml:"severity"`
	Files    FileSet                 `yaml:"files"`
	Require  FileContractRequirement `yaml:"require"`
	Deny     FileContractDenial      `yaml:"deny"`
	Message  string                  `yaml:"message"`
}

type FileContractRequirement struct {
	SiblingFiles []string              `yaml:"siblingFiles"`
	Declarations []DeclarationSelector `yaml:"declarations"`
}

type FileContractDenial struct {
	Declarations []DeclarationSelector `yaml:"declarations"`
}

func checkFileContractRules(pass *analysis.Pass, file *ast.File, filename string, cfg *Config, inventory *repositoryInventory) {
	paths := pathCandidates(filename, cfg)
	candidates := extractDeclarationCandidates(pass.Fset, file)
	for _, rule := range cfg.FileContractRules {
		matches := cfg.matchFileSet(rule.Files, paths)
		if len(matches) == 0 {
			continue
		}
		for _, match := range matches {
			if inventory != nil {
				dir := filepath.ToSlash(filepath.Dir(relativeToWorkdir(filename, cfg)))
				for _, sibling := range rule.Require.SiblingFiles {
					expanded, _ := expandTemplate(sibling, match.Captures)
					if !inventory.hasRelativeFile(dir, expanded) {
						pass.Reportf(file.Package, "[%s]: %s required sibling file not found: %s", rule.ID, rule.Message, expanded)
					}
				}
			}
			for _, selector := range rule.Require.Declarations {
				expanded := expandDeclarationSelector(selector, match.Captures)
				if !anyDeclarationMatches(candidates, expanded) {
					pass.Reportf(file.Package, "[%s]: %s required declaration not found: %s", rule.ID, rule.Message, selectorDescription(expanded))
				}
			}
			seen := make(map[string]bool)
			for _, selector := range rule.Deny.Declarations {
				expanded := expandDeclarationSelector(selector, match.Captures)
				matched := matchingDeclarationCandidates(candidates, expanded)
				if !countMatches(len(matched), expanded.Count, false) {
					continue
				}
				for _, candidate := range matched {
					key := fmt.Sprintf("%d:%s:%s", candidate.Pos, candidate.Kind, candidate.Name)
					if seen[key] {
						continue
					}
					seen[key] = true
					pass.Reportf(candidate.Pos, "[%s]: %s denied declaration matched: %s", rule.ID, rule.Message, candidateDescription(candidate))
				}
			}
		}
	}
}

func expandDeclarationSelector(selector DeclarationSelector, captures map[string]string) DeclarationSelector {
	expanded, _ := expandDeclarationSelectorChecked(selector, captures)
	return expanded
}

func expandDeclarationSelectorChecked(selector DeclarationSelector, captures map[string]string) (DeclarationSelector, error) {
	expanded := selector
	var expansionError error
	expand := func(value string) string {
		if value == "" || expansionError != nil {
			return value
		}
		result, err := expandTemplate(value, captures)
		if err != nil {
			expansionError = err
			return value
		}
		return result
	}
	expanded.Name = expand(selector.Name)
	expanded.NameMatches = expandTemplateValues(selector.NameMatches, expand)
	expanded.NameNotMatches = expandTemplateValues(selector.NameNotMatches, expand)
	expanded.Receiver.Type = expand(selector.Receiver.Type)
	expanded.Receiver.TypeMatches = expandTemplateValues(selector.Receiver.TypeMatches, expand)
	expanded.Parameters = expandParameterCondition(selector.Parameters, expand)
	expanded.Returns = expandReturnCondition(selector.Returns, expand)
	expanded.Embeds = expandEmbedCondition(selector.Embeds, expand)
	expanded.Methods = expandMethodCondition(selector.Methods, expand)
	if selector.Value.Equals != nil {
		value := expand(*selector.Value.Equals)
		expanded.Value.Equals = &value
	}
	expanded.Value.Matches = expandTemplateValues(selector.Value.Matches, expand)
	return expanded, expansionError
}

func expandTemplateValues(values []string, expand func(string) string) []string {
	expanded := make([]string, len(values))
	for i, value := range values {
		expanded[i] = expand(value)
	}
	return expanded
}

func expandTypeCondition(condition TypeCondition, expand func(string) string) TypeCondition {
	condition.Name = expand(condition.Name)
	condition.Type = expand(condition.Type)
	condition.TypeMatches = expandTemplateValues(condition.TypeMatches, expand)
	return condition
}

func expandTypeConditionList(conditions TypeConditionList, expand func(string) string) TypeConditionList {
	expanded := make(TypeConditionList, len(conditions))
	for i, condition := range conditions {
		expanded[i] = expandTypeCondition(condition, expand)
	}
	return expanded
}

func expandOptionalTypeCondition(condition *TypeCondition, expand func(string) string) *TypeCondition {
	if condition == nil {
		return nil
	}
	expanded := expandTypeCondition(*condition, expand)
	return &expanded
}

func expandParameterCondition(condition ParameterCondition, expand func(string) string) ParameterCondition {
	condition.First = expandOptionalTypeCondition(condition.First, expand)
	condition.Contains = expandTypeConditionList(condition.Contains, expand)
	condition.All = expandOptionalTypeCondition(condition.All, expand)
	return condition
}

func expandReturnCondition(condition ReturnCondition, expand func(string) string) ReturnCondition {
	condition.First = expandOptionalTypeCondition(condition.First, expand)
	condition.Contains = expandTypeConditionList(condition.Contains, expand)
	condition.All = expandOptionalTypeCondition(condition.All, expand)
	condition.Matches = expandTemplateValues(condition.Matches, expand)
	return condition
}

func expandEmbedCondition(condition EmbedCondition, expand func(string) string) EmbedCondition {
	condition.Contains = expandTypeConditionList(condition.Contains, expand)
	condition.All = expandOptionalTypeCondition(condition.All, expand)
	return condition
}

func expandFunctionCondition(condition FunctionCondition, expand func(string) string) FunctionCondition {
	condition.Name = expand(condition.Name)
	condition.NameMatches = expandTemplateValues(condition.NameMatches, expand)
	condition.Parameters = expandParameterCondition(condition.Parameters, expand)
	condition.Returns = expandReturnCondition(condition.Returns, expand)
	return condition
}

func expandMethodCondition(condition MethodCondition, expand func(string) string) MethodCondition {
	condition.Contains = append([]FunctionCondition(nil), condition.Contains...)
	for i, method := range condition.Contains {
		condition.Contains[i] = expandFunctionCondition(method, expand)
	}
	if condition.All != nil {
		expanded := expandFunctionCondition(*condition.All, expand)
		condition.All = &expanded
	}
	return condition
}

func fileSetAppliesToPaths(files FileSet, paths []string) bool {
	if MatchesAnyPathGlob(paths, files.Exclude) {
		return false
	}
	for _, path := range paths {
		if MatchesAnyGlob(path, files.Include) {
			return true
		}
	}
	return false
}

func relativeToWorkdir(filename string, cfg *Config) string {
	if rel, err := filepath.Rel(cfg.workdirAbs, filename); err == nil && rel != "." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(filename)
}

func selectorDescription(selector DeclarationSelector) string {
	parts := []string{selector.Kind}
	if selector.Name != "" {
		parts = append(parts, selector.Name)
	}
	if selector.Exported != nil {
		if *selector.Exported {
			parts = append([]string{"exported"}, parts...)
		} else {
			parts = append([]string{"unexported"}, parts...)
		}
	}
	if len(selector.NameMatches) > 0 {
		parts = append(parts, "name matches "+quotedList(selector.NameMatches))
	}
	if len(selector.NameNotMatches) > 0 {
		parts = append(parts, "name does not match "+quotedList(selector.NameNotMatches))
	}
	if len(selector.Returns.Contains) > 0 {
		parts = append(parts, "returns contains "+quotedTypeList(selector.Returns.Contains))
	}
	if len(selector.Returns.Matches) > 0 {
		parts = append(parts, "returns matches "+quotedList(selector.Returns.Matches))
	}
	if selector.Value.Equals != nil {
		parts = append(parts, fmt.Sprintf("value equals %q", *selector.Value.Equals))
	}
	if len(selector.Value.Matches) > 0 {
		parts = append(parts, "value matches "+quotedList(selector.Value.Matches))
	}
	return strings.Join(parts, " ")
}

func quotedList(values []string) string {
	quoted := make([]string, len(values))
	for i, value := range values {
		quoted[i] = fmt.Sprintf("%q", value)
	}
	return strings.Join(quoted, " or ")
}

func quotedTypeList(values []TypeCondition) string {
	types := make([]string, len(values))
	for i, value := range values {
		types[i] = value.Type
	}
	return quotedList(types)
}

func candidateDescription(candidate declarationCandidate) string {
	export := "unexported"
	if candidate.Exported {
		export = "exported"
	}
	return fmt.Sprintf("%s %s %s", export, candidate.Kind, candidate.Name)
}

func (cfg *Config) validateFileContractRules() error {
	for i, rule := range cfg.FileContractRules {
		if rule.ID == "" {
			return fmt.Errorf("fileContractRules[%d].id is required", i)
		}
		if err := validateSeverity(rule.ID, rule.Severity); err != nil {
			return err
		}
		if len(rule.Files.Include) == 0 && len(rule.Files.Templates) == 0 {
			return fmt.Errorf("file contract rule %q must include at least one file pattern", rule.ID)
		}
		for _, name := range rule.Files.Templates {
			compiled, ok := cfg.compiledTemplates[name]
			if !ok {
				return fmt.Errorf("file contract rule %q references undefined template %q", rule.ID, name)
			}
			captures := make(map[string]string, len(compiled.captures))
			for _, capture := range compiled.captures {
				captures[capture] = "example"
			}
			if err := validateFileContractExpansions(rule, captures); err != nil {
				return err
			}
		}
		if len(rule.Files.Include) > 0 {
			if err := validateFileContractExpansions(rule, map[string]string{}); err != nil {
				return err
			}
		}
		if rule.Message == "" {
			return fmt.Errorf("file contract rule %q message is required", rule.ID)
		}
		if len(rule.Require.SiblingFiles) == 0 && len(rule.Require.Declarations) == 0 && len(rule.Deny.Declarations) == 0 {
			return fmt.Errorf("file contract rule %q must configure a requirement or denial", rule.ID)
		}
		for _, pattern := range append(append(append([]string{}, rule.Files.Include...), rule.Files.Exclude...), rule.Require.SiblingFiles...) {
			if err := validateRelativeGlob(pattern); err != nil {
				return fmt.Errorf("file contract rule %q: %w", rule.ID, err)
			}
		}
		for _, selector := range append(append([]DeclarationSelector{}, rule.Require.Declarations...), rule.Deny.Declarations...) {
			if err := validateDeclarationSelector(rule.ID, selector); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateFileContractExpansions(rule FileContractRule, captures map[string]string) error {
	values := append([]string{}, rule.Require.SiblingFiles...)
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, err := expandTemplate(value, captures); err != nil {
			return fmt.Errorf("file contract rule %q has invalid template expansion %q: %w", rule.ID, value, err)
		}
	}
	selectors := append(append([]DeclarationSelector{}, rule.Require.Declarations...), rule.Deny.Declarations...)
	for _, selector := range selectors {
		if _, err := expandDeclarationSelectorChecked(selector, captures); err != nil {
			return fmt.Errorf("file contract rule %q has invalid declaration template expansion: %w", rule.ID, err)
		}
	}
	return nil
}
