package filearch

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
)

type ContentRule struct {
	ID       string         `yaml:"id"`
	Severity string         `yaml:"severity"`
	Files    FileSet        `yaml:"files"`
	Allow    DeclarationSet `yaml:"allow"`
	Deny     DeclarationSet `yaml:"deny"`
	Message  string         `yaml:"message"`
}

func (cfg *Config) validateContentRules() error {
	for i, rule := range cfg.ContentRules {
		if rule.ID == "" {
			return fmt.Errorf("contentRules[%d].id is required", i)
		}
		if err := validateSeverity(rule.ID, rule.Severity); err != nil {
			return err
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
		path, _ := strconv.Unquote(spec.Path.Value)
		reportIfContentViolation(pass, rule, "import", path, spec.Pos())
	}

	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			reportIfContentViolation(pass, rule, "func", funcDeclName(decl), decl.Pos())
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
		reportIfContentViolation(pass, rule, "const", valueSpecNames(decl), decl.Pos())
	case token.VAR:
		reportIfContentViolation(pass, rule, "var", valueSpecNames(decl), decl.Pos())
	case token.TYPE:
		for _, spec := range decl.Specs {
			typeSpec := spec.(*ast.TypeSpec)
			reportIfContentViolation(pass, rule, declarationKind(typeSpec), typeSpec.Name.Name, typeSpec.Pos())
		}
	}
}

// funcDeclName renders a method as "Receiver.Method" and a plain func as its name.
func funcDeclName(decl *ast.FuncDecl) string {
	if decl.Recv != nil && len(decl.Recv.List) > 0 {
		return receiverTypeName(decl.Recv.List[0].Type) + "." + decl.Name.Name
	}
	return decl.Name.Name
}

func receiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return receiverTypeName(t.X)
	case *ast.Ident:
		return t.Name
	default:
		return ""
	}
}

func valueSpecNames(decl *ast.GenDecl) string {
	var names []string
	for _, spec := range decl.Specs {
		if valueSpec, ok := spec.(*ast.ValueSpec); ok {
			for _, name := range valueSpec.Names {
				names = append(names, name.Name)
			}
		}
	}
	return strings.Join(names, ", ")
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

func reportIfContentViolation(pass *analysis.Pass, rule ContentRule, kind, name string, pos token.Pos) {
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

	reason := contentViolationReason(rule, kind, denied)
	pass.Reportf(pos, "[%s]: %s Found disallowed %s%s%s.", rule.ID, rule.Message, kind, quotedName(name), reason)
}

// contentViolationReason explains why the declaration is rejected: an explicit
// deny, or absence from a non-empty allow list.
func contentViolationReason(rule ContentRule, kind string, denied bool) string {
	if denied {
		return fmt.Sprintf(" (%s declarations are denied here)", kind)
	}
	if len(rule.Allow.Declarations) > 0 {
		return fmt.Sprintf(" (only %s allowed here)", strings.Join(rule.Allow.Declarations, ", "))
	}
	return ""
}

func quotedName(name string) string {
	if name == "" {
		return ""
	}
	return " " + strconv.Quote(name)
}

func ruleAppliesToPath(rule ContentRule, paths []string) bool {
	return fileSetAppliesToPaths(rule.Files, paths)
}
