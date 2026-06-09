package filearch

import (
	"fmt"
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

type ContentRule struct {
	ID      string         `yaml:"id"`
	Files   FileSet        `yaml:"files"`
	Allow   DeclarationSet `yaml:"allow"`
	Deny    DeclarationSet `yaml:"deny"`
	Message string         `yaml:"message"`
}

func (cfg *Config) validateContentRules() error {
	for i, rule := range cfg.ContentRules {
		if rule.ID == "" {
			return fmt.Errorf("contentRules[%d].id is required", i)
		}
		if len(rule.Files.Include) == 0 {
			return fmt.Errorf("content rule %q must include at least one file pattern", rule.ID)
		}
		if rule.Message == "" {
			return fmt.Errorf("content rule %q message is required", rule.ID)
		}
		for _, kind := range append(rule.Allow.Declarations, rule.Deny.Declarations...) {
			if !validDeclarationKinds[kind] {
				return fmt.Errorf("content rule %q has unsupported declaration kind %q", rule.ID, kind)
			}
		}
	}
	return nil
}

func checkContentRule(pass *analysis.Pass, file *ast.File, rule ContentRule) {
	for _, spec := range file.Imports {
		reportIfContentViolation(pass, rule, "import", spec.Pos())
	}

	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			reportIfContentViolation(pass, rule, "func", decl.Pos())
		case *ast.GenDecl:
			checkGenDecl(pass, rule, decl)
		}
	}
}

func checkGenDecl(pass *analysis.Pass, rule ContentRule, decl *ast.GenDecl) {
	switch decl.Tok {
	case token.IMPORT:
		return
	case token.CONST:
		reportIfContentViolation(pass, rule, "const", decl.Pos())
	case token.VAR:
		reportIfContentViolation(pass, rule, "var", decl.Pos())
	case token.TYPE:
		for _, spec := range decl.Specs {
			typeSpec := spec.(*ast.TypeSpec)
			reportIfContentViolation(pass, rule, declarationKind(typeSpec), typeSpec.Pos())
		}
	}
}

func declarationKind(spec *ast.TypeSpec) string {
	switch spec.Type.(type) {
	case *ast.InterfaceType:
		return "interface"
	case *ast.StructType:
		return "struct"
	default:
		return "type"
	}
}

func reportIfContentViolation(pass *analysis.Pass, rule ContentRule, kind string, pos token.Pos) {
	if kind == "package" || kind == "import" {
		if !contains(rule.Deny.Declarations, kind) {
			return
		}
	}

	denied := contains(rule.Deny.Declarations, kind)
	allowed := contains(rule.Allow.Declarations, kind)

	if !denied && (len(rule.Allow.Declarations) == 0 || allowed) {
		return
	}

	pass.Reportf(pos, "[%s]: %s detected declaration kind: %s", rule.ID, rule.Message, kind)
}

func ruleAppliesToPath(rule ContentRule, paths []string) bool {
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
