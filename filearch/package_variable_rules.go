package filearch

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

type PackageVariableRule struct {
	ID          string              `yaml:"id"`
	Severity    string              `yaml:"severity"`
	Files       FileSet             `yaml:"files"`
	WriteFiles  FileSet             `yaml:"writeFiles"`
	Declaration DeclarationSelector `yaml:"declaration"`
	DenyWrites  bool                `yaml:"denyWrites"`
	Message     string              `yaml:"message"`
}

func (cfg *Config) validatePackageVariableRules() error {
	for i, rule := range cfg.PackageVariableRules {
		if rule.ID == "" {
			return fmt.Errorf("packageVariableRules[%d].id is required", i)
		}
		if err := validateSeverity(rule.ID, rule.Severity); err != nil {
			return err
		}
		if len(rule.Files.Include) == 0 && len(rule.Files.Templates) == 0 {
			return fmt.Errorf("package variable rule %q must include at least one declaration file pattern", rule.ID)
		}
		if err := cfg.validatePackageVariableFileSet(rule.ID, "files", rule.Files); err != nil {
			return err
		}
		if err := cfg.validatePackageVariableFileSet(rule.ID, "writeFiles", effectiveWriteFiles(rule.WriteFiles)); err != nil {
			return err
		}
		for _, name := range rule.Files.Templates {
			compiled := cfg.compiledTemplates[name]
			captures := make(map[string]string, len(compiled.captures))
			for _, capture := range compiled.captures {
				captures[capture] = "example"
			}
			if err := validatePackageVariableExpansion(rule, captures); err != nil {
				return err
			}
		}
		if len(rule.Files.Include) > 0 {
			if err := validatePackageVariableExpansion(rule, map[string]string{}); err != nil {
				return err
			}
		}
		if err := validateDeclarationSelector(rule.ID, rule.Declaration); err != nil {
			return err
		}
		if rule.Declaration.Kind != "var" {
			return fmt.Errorf("package variable rule %q declaration kind must be var", rule.ID)
		}
		if countConfigured(rule.Declaration.Count) {
			return fmt.Errorf("package variable rule %q declaration count is not allowed", rule.ID)
		}
		if !rule.DenyWrites {
			return fmt.Errorf("package variable rule %q denyWrites must be true", rule.ID)
		}
		if rule.Message == "" {
			return fmt.Errorf("package variable rule %q message is required", rule.ID)
		}
	}
	return nil
}

func validatePackageVariableExpansion(rule PackageVariableRule, captures map[string]string) error {
	if _, err := expandDeclarationSelectorChecked(rule.Declaration, captures); err != nil {
		return fmt.Errorf("package variable rule %q has invalid declaration template expansion: %w", rule.ID, err)
	}
	return nil
}

func (cfg *Config) validatePackageVariableFileSet(ruleID, name string, files FileSet) error {
	for _, pattern := range append(append([]string{}, files.Include...), files.Exclude...) {
		if err := validateRelativeGlob(pattern); err != nil {
			return fmt.Errorf("package variable rule %q %s: %w", ruleID, name, err)
		}
	}
	for _, template := range files.Templates {
		if _, ok := cfg.compiledTemplates[template]; !ok {
			return fmt.Errorf("package variable rule %q %s references undefined template %q", ruleID, name, template)
		}
	}
	return nil
}

func effectiveWriteFiles(files FileSet) FileSet {
	if len(files.Include) == 0 && len(files.Templates) == 0 {
		files.Include = []string{"**/*.go"}
	}
	return files
}

func checkPackageVariableRules(pass *analysis.Pass, declarationFile *ast.File, cfg *Config, paths []string) {
	if pass.TypesInfo == nil || pass.Pkg == nil {
		return
	}
	for _, rule := range cfg.PackageVariableRules {
		if !fileSetAppliesToGenerated(rule.Files, declarationFile) {
			continue
		}
		selected := make(map[*types.Var]bool)
		for _, match := range cfg.matchFileSet(rule.Files, paths) {
			for variable := range selectedPackageVariables(pass, declarationFile, expandDeclarationSelector(rule.Declaration, match.Captures)) {
				selected[variable] = true
			}
		}
		if len(selected) == 0 {
			continue
		}
		checkSelectedVariableWrites(pass, cfg, rule, selected)
	}
}

func selectedPackageVariables(pass *analysis.Pass, file *ast.File, selector DeclarationSelector) map[*types.Var]bool {
	candidates := matchingDeclarationCandidates(extractDeclarationCandidates(pass.Fset, file), selector)
	positions := make(map[token.Pos]bool, len(candidates))
	for _, candidate := range candidates {
		positions[candidate.Pos] = true
	}
	selected := make(map[*types.Var]bool, len(candidates))
	ast.Inspect(file, func(node ast.Node) bool {
		name, ok := node.(*ast.Ident)
		if !ok || !positions[name.Pos()] {
			return true
		}
		variable, ok := pass.TypesInfo.Defs[name].(*types.Var)
		if ok && variable.Parent() == pass.Pkg.Scope() {
			selected[variable] = true
		}
		return true
	})
	return selected
}

func checkSelectedVariableWrites(pass *analysis.Pass, cfg *Config, rule PackageVariableRule, selected map[*types.Var]bool) {
	writeFiles := effectiveWriteFiles(rule.WriteFiles)
	seen := make(map[token.Pos]bool)
	for _, file := range pass.Files {
		filename := pass.Fset.Position(file.Pos()).Filename
		if !fileSetAppliesToGenerated(writeFiles, file) || len(cfg.matchFileSet(writeFiles, pathCandidates(filename, cfg))) == 0 {
			continue
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.AssignStmt:
				for _, expression := range node.Lhs {
					reportSelectedVariableWrite(pass, rule, expression, selected, seen)
				}
			case *ast.IncDecStmt:
				reportSelectedVariableWrite(pass, rule, node.X, selected, seen)
			case *ast.RangeStmt:
				if node.Tok == token.ASSIGN {
					reportSelectedVariableWrite(pass, rule, node.Key, selected, seen)
					reportSelectedVariableWrite(pass, rule, node.Value, selected, seen)
				}
			}
			return true
		})
	}
}

func reportSelectedVariableWrite(pass *analysis.Pass, rule PackageVariableRule, expression ast.Expr, selected map[*types.Var]bool, seen map[token.Pos]bool) {
	root := writtenRootIdentifier(expression)
	if root == nil || seen[expression.Pos()] {
		return
	}
	variable, ok := pass.TypesInfo.ObjectOf(root).(*types.Var)
	if !ok || !selected[variable] {
		return
	}
	seen[expression.Pos()] = true
	pass.Reportf(expression.Pos(), "[%s]: %s direct write to package variable %s", rule.ID, rule.Message, variable.Name())
}

func writtenRootIdentifier(expression ast.Expr) *ast.Ident {
	switch expression := expression.(type) {
	case nil:
		return nil
	case *ast.Ident:
		return expression
	case *ast.ParenExpr:
		return writtenRootIdentifier(expression.X)
	case *ast.StarExpr:
		return writtenRootIdentifier(expression.X)
	case *ast.SelectorExpr:
		return writtenRootIdentifier(expression.X)
	case *ast.IndexExpr:
		return writtenRootIdentifier(expression.X)
	case *ast.IndexListExpr:
		return writtenRootIdentifier(expression.X)
	default:
		return nil
	}
}
