package filearch

import (
	"fmt"
	"go/ast"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"
)

type FileContractRule struct {
	ID      string                  `yaml:"id"`
	Files   FileSet                 `yaml:"files"`
	Require FileContractRequirement `yaml:"require"`
	Deny    FileContractDenial      `yaml:"deny"`
	Message string                  `yaml:"message"`
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
		if !fileSetAppliesToPaths(rule.Files, paths) {
			continue
		}
		if inventory != nil {
			dir := filepath.ToSlash(filepath.Dir(relativeToWorkdir(filename, cfg)))
			for _, sibling := range rule.Require.SiblingFiles {
				if !inventory.hasRelativeFile(dir, sibling) {
					pass.Reportf(file.Package, "[%s]: %s required sibling file not found: %s", rule.ID, rule.Message, sibling)
				}
			}
		}
		for _, selector := range rule.Require.Declarations {
			if !anyDeclarationMatches(candidates, selector) {
				pass.Reportf(file.Package, "[%s]: %s required declaration not found: %s", rule.ID, rule.Message, selectorDescription(selector))
			}
		}
		seen := make(map[string]bool)
		for _, selector := range rule.Deny.Declarations {
			for _, candidate := range candidates {
				if !declarationMatches(candidate, selector) {
					continue
				}
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
		parts = append(parts, "returns contains "+quotedList(selector.Returns.Contains))
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
		if len(rule.Files.Include) == 0 {
			return fmt.Errorf("file contract rule %q must include at least one file pattern", rule.ID)
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
