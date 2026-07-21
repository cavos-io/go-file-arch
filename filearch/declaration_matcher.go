package filearch

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"regexp"
	"strconv"
)

type DeclarationSelector struct {
	Kind           string          `yaml:"kind"`
	Name           string          `yaml:"name"`
	NameMatches    []string        `yaml:"nameMatches"`
	NameNotMatches []string        `yaml:"nameNotMatches"`
	Exported       *bool           `yaml:"exported"`
	Returns        ReturnCondition `yaml:"returns"`
	Value          ValueCondition  `yaml:"value"`
}

type ReturnCondition struct {
	Contains []string `yaml:"contains"`
	Matches  []string `yaml:"matches"`
}

type ValueCondition struct {
	Equals  *string  `yaml:"equals"`
	Matches []string `yaml:"matches"`
}

type declarationCandidate struct {
	Kind     string
	Name     string
	Exported bool
	Pos      token.Pos
	Results  []string
	Value    *string
}

func extractDeclarationCandidates(fset *token.FileSet, file *ast.File) []declarationCandidate {
	var candidates []declarationCandidate
	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			candidates = append(candidates, declarationCandidate{
				Kind: "func", Name: decl.Name.Name, Exported: ast.IsExported(decl.Name.Name),
				Pos: decl.Pos(), Results: functionResults(fset, decl),
			})
		case *ast.GenDecl:
			switch decl.Tok {
			case token.TYPE:
				for _, spec := range decl.Specs {
					typeSpec := spec.(*ast.TypeSpec)
					candidates = append(candidates, declarationCandidate{
						Kind: declarationKind(typeSpec), Name: typeSpec.Name.Name,
						Exported: ast.IsExported(typeSpec.Name.Name), Pos: typeSpec.Pos(),
					})
				}
			case token.CONST, token.VAR:
				kind := "var"
				if decl.Tok == token.CONST {
					kind = "const"
				}
				for _, spec := range decl.Specs {
					valueSpec := spec.(*ast.ValueSpec)
					for i, name := range valueSpec.Names {
						var value *string
						if kind == "const" && i < len(valueSpec.Values) {
							value = literalValue(valueSpec.Values[i])
						}
						candidates = append(candidates, declarationCandidate{
							Kind: kind, Name: name.Name, Exported: ast.IsExported(name.Name),
							Pos: name.Pos(), Value: value,
						})
					}
				}
			}
		}
	}
	return candidates
}

func renderExpr(fset *token.FileSet, expr ast.Expr) string {
	var buffer bytes.Buffer
	if err := format.Node(&buffer, fset, expr); err != nil {
		return ""
	}
	return buffer.String()
}

func functionResults(fset *token.FileSet, decl *ast.FuncDecl) []string {
	if decl.Type.Results == nil {
		return nil
	}
	var results []string
	for _, field := range decl.Type.Results.List {
		rendered := renderExpr(fset, field.Type)
		count := len(field.Names)
		if count == 0 {
			count = 1
		}
		for range count {
			results = append(results, rendered)
		}
	}
	return results
}

func literalValue(expr ast.Expr) *string {
	switch expr := expr.(type) {
	case *ast.BasicLit:
		value := expr.Value
		if expr.Kind == token.STRING || expr.Kind == token.CHAR {
			unquoted, err := strconv.Unquote(value)
			if err != nil {
				return nil
			}
			value = unquoted
		}
		return &value
	case *ast.Ident:
		if expr.Name == "true" || expr.Name == "false" {
			value := expr.Name
			return &value
		}
	}
	return nil
}

func declarationMatches(candidate declarationCandidate, selector DeclarationSelector) bool {
	if candidate.Kind != selector.Kind {
		return false
	}
	if selector.Name != "" && candidate.Name != selector.Name {
		return false
	}
	if selector.Exported != nil && candidate.Exported != *selector.Exported {
		return false
	}
	if len(selector.NameMatches) > 0 && !matchesRegexAny(candidate.Name, selector.NameMatches) {
		return false
	}
	if matchesRegexAny(candidate.Name, selector.NameNotMatches) {
		return false
	}
	for _, required := range selector.Returns.Contains {
		if !contains(candidate.Results, required) {
			return false
		}
	}
	if len(selector.Returns.Matches) > 0 && !anyResultMatches(candidate.Results, selector.Returns.Matches) {
		return false
	}
	if selector.Value.Equals != nil && (candidate.Value == nil || *candidate.Value != *selector.Value.Equals) {
		return false
	}
	if len(selector.Value.Matches) > 0 && (candidate.Value == nil || !matchesRegexAny(*candidate.Value, selector.Value.Matches)) {
		return false
	}
	return true
}

func anyDeclarationMatches(candidates []declarationCandidate, selector DeclarationSelector) bool {
	for _, candidate := range candidates {
		if declarationMatches(candidate, selector) {
			return true
		}
	}
	return false
}

func anyResultMatches(results, patterns []string) bool {
	for _, result := range results {
		if matchesRegexAny(result, patterns) {
			return true
		}
	}
	return false
}

func validateDeclarationSelector(ruleID string, selector DeclarationSelector) error {
	if !validDeclarationKinds[selector.Kind] || selector.Kind == "package" || selector.Kind == "import" {
		return fmt.Errorf("file contract rule %q has unsupported declaration kind %q", ruleID, selector.Kind)
	}
	if selector.Kind != "func" && (len(selector.Returns.Contains) > 0 || len(selector.Returns.Matches) > 0) {
		return fmt.Errorf("file contract rule %q: returns is only valid for func", ruleID)
	}
	if selector.Kind != "const" && (selector.Value.Equals != nil || len(selector.Value.Matches) > 0) {
		return fmt.Errorf("file contract rule %q: value is only valid for const", ruleID)
	}
	groups := []struct {
		name   string
		values []string
	}{
		{"nameMatches", selector.NameMatches},
		{"nameNotMatches", selector.NameNotMatches},
		{"returns.matches", selector.Returns.Matches},
		{"value.matches", selector.Value.Matches},
	}
	for _, group := range groups {
		for _, pattern := range group.values {
			if pattern == "" {
				return fmt.Errorf("file contract rule %q has empty %s regex", ruleID, group.name)
			}
			if _, err := regexp.Compile(pattern); err != nil {
				return fmt.Errorf("file contract rule %q has invalid %s regex %q: %w", ruleID, group.name, pattern, err)
			}
		}
	}
	for _, result := range selector.Returns.Contains {
		if result == "" {
			return fmt.Errorf("file contract rule %q has empty returns.contains value", ruleID)
		}
	}
	return nil
}
