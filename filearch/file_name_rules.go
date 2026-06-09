package filearch

import (
	"fmt"
	"go/ast"
	"go/token"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/tools/go/analysis"
)

type FileNameRule struct {
	ID      string              `yaml:"id"`
	Files   FileSet             `yaml:"files"`
	When    FileNameRuleWhen    `yaml:"when"`
	Require FileNameRequirement `yaml:"require"`
	Deny    FileNameRequirement `yaml:"deny"`
	Message string              `yaml:"message"`
}

type FileNameRuleWhen struct {
	Declarations  []string `yaml:"declarations"`
	TypeNameRegex []string `yaml:"typeNameRegex"`
}

type FileNameRequirement struct {
	FileName FileNameCondition `yaml:"fileName"`
}

type FileNameCondition struct {
	Equals    string   `yaml:"equals"`
	EqualsAny []string `yaml:"equalsAny"`
	Matches   []string `yaml:"matches"`
	Prefix    string   `yaml:"prefix"`
	Suffix    string   `yaml:"suffix"`
}

func (cfg *Config) validateFileNameRules() error {
	for i, rule := range cfg.FileNameRules {
		if rule.ID == "" {
			return fmt.Errorf("fileNameRules[%d].id is required", i)
		}
		if len(rule.Files.Include) == 0 {
			return fmt.Errorf("fileName rule %q must include at least one file pattern", rule.ID)
		}
		if rule.Message == "" {
			return fmt.Errorf("fileName rule %q message is required", rule.ID)
		}
		for _, kind := range rule.When.Declarations {
			if !validDeclarationKinds[kind] {
				return fmt.Errorf("fileName rule %q has unsupported declaration kind %q", rule.ID, kind)
			}
		}
		for _, pattern := range append(rule.When.TypeNameRegex, rule.Require.FileName.Matches...) {
			if _, err := regexp.Compile(pattern); err != nil {
				return fmt.Errorf("fileName rule %q has invalid regex %q: %w", rule.ID, pattern, err)
			}
		}
		for _, pattern := range rule.Deny.FileName.Matches {
			if _, err := regexp.Compile(pattern); err != nil {
				return fmt.Errorf("fileName rule %q has invalid regex %q: %w", rule.ID, pattern, err)
			}
		}
	}
	return nil
}

func checkFileNameRule(pass *analysis.Pass, file *ast.File, filename string, rule FileNameRule) {
	baseName := filepath.Base(filename)
	triggers := fileNameRuleTriggers(file, rule)
	if len(triggers) == 0 {
		return
	}

	violation, ok := fileNameViolation(baseName, rule)
	if !ok {
		return
	}

	for _, trigger := range triggers {
		pass.Reportf(trigger.pos, "[%s]: %s file name %q %s; detected declaration kind: %s", rule.ID, rule.Message, baseName, violation, trigger.kind)
	}
}

type fileNameRuleTrigger struct {
	pos  token.Pos
	kind string
}

func fileNameRuleTriggers(file *ast.File, rule FileNameRule) []fileNameRuleTrigger {
	if len(rule.When.Declarations) == 0 && len(rule.When.TypeNameRegex) == 0 {
		return []fileNameRuleTrigger{{pos: file.Package, kind: "file"}}
	}

	var triggers []fileNameRuleTrigger
	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			if matchesWhenDeclaration(rule, "func") && len(rule.When.TypeNameRegex) == 0 {
				triggers = append(triggers, fileNameRuleTrigger{pos: decl.Pos(), kind: "func"})
			}
		case *ast.GenDecl:
			switch decl.Tok {
			case token.CONST:
				if matchesWhenDeclaration(rule, "const") && len(rule.When.TypeNameRegex) == 0 {
					triggers = append(triggers, fileNameRuleTrigger{pos: decl.Pos(), kind: "const"})
				}
				continue
			case token.VAR:
				if matchesWhenDeclaration(rule, "var") && len(rule.When.TypeNameRegex) == 0 {
					triggers = append(triggers, fileNameRuleTrigger{pos: decl.Pos(), kind: "var"})
				}
				continue
			case token.IMPORT:
				continue
			}
			for _, spec := range decl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				kind := declarationKind(typeSpec)
				if !matchesWhenDeclaration(rule, kind) {
					continue
				}
				if !matchesTypeNameRegex(rule, typeSpec.Name.Name) {
					continue
				}
				triggers = append(triggers, fileNameRuleTrigger{pos: typeSpec.Pos(), kind: kind})
			}
		}
	}
	return triggers
}

func matchesWhenDeclaration(rule FileNameRule, kind string) bool {
	return len(rule.When.Declarations) == 0 || contains(rule.When.Declarations, kind)
}

func matchesTypeNameRegex(rule FileNameRule, typeName string) bool {
	if len(rule.When.TypeNameRegex) == 0 {
		return true
	}
	for _, pattern := range rule.When.TypeNameRegex {
		if regexp.MustCompile(pattern).MatchString(typeName) {
			return true
		}
	}
	return false
}

func fileNameAllowed(baseName string, rule FileNameRule) bool {
	_, violated := fileNameViolation(baseName, rule)
	return !violated
}

func fileNameViolation(baseName string, rule FileNameRule) (string, bool) {
	if !fileNameConditionEmpty(rule.Require.FileName) && !matchesFileNameCondition(baseName, rule.Require.FileName) {
		return "does not satisfy required fileName condition", true
	}
	if matchesFileNameCondition(baseName, rule.Deny.FileName) {
		return "satisfies denied fileName condition", true
	}
	return "", false
}

func fileNameConditionEmpty(condition FileNameCondition) bool {
	return condition.Equals == "" &&
		len(condition.EqualsAny) == 0 &&
		len(condition.Matches) == 0 &&
		condition.Prefix == "" &&
		condition.Suffix == ""
}

func matchesFileNameCondition(value string, condition FileNameCondition) bool {
	if condition.Equals != "" && value == condition.Equals {
		return true
	}
	if contains(condition.EqualsAny, value) {
		return true
	}
	if matchesRegexAny(value, condition.Matches) {
		return true
	}
	if condition.Prefix != "" && strings.HasPrefix(value, condition.Prefix) {
		return true
	}
	if condition.Suffix != "" && strings.HasSuffix(value, condition.Suffix) {
		return true
	}
	return false
}

func matchesRegexAny(value string, patterns []string) bool {
	for _, pattern := range patterns {
		if regexp.MustCompile(pattern).MatchString(value) {
			return true
		}
	}
	return false
}

func fileNameRuleAppliesToPath(rule FileNameRule, paths []string) bool {
	if MatchesAnyPathGlob(paths, rule.Files.Exclude) {
		return false
	}
	for _, path := range paths {
		if MatchesAnyGlob(path, rule.Files.Include) {
			return true
		}
	}
	return false
}
