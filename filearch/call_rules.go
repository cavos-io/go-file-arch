package filearch

import (
	"fmt"
	"go/ast"
	"go/types"
	"regexp"
	"strings"

	"golang.org/x/tools/go/analysis"
)

type CallRule struct {
	ID              string        `yaml:"id"`
	Severity        string        `yaml:"severity"`
	Files           FileSet       `yaml:"files"`
	Callee          CallTarget    `yaml:"callee"`
	Require         CallCondition `yaml:"require"`
	Deny            CallCondition `yaml:"deny"`
	DenyNonMatching bool          `yaml:"denyNonMatching"`
	Message         string        `yaml:"message"`
}

type CallTarget struct {
	Package string `yaml:"package"`
	Name    string `yaml:"name"`
}

type CallCondition struct {
	Arguments []CallArgumentSelector `yaml:"arguments"`
	Count     CountCondition         `yaml:"count"`
}

type CallArgumentSelector struct {
	Position int                  `yaml:"position"`
	Type     ResolvedTypeSelector `yaml:"type"`
}

type ResolvedTypeSelector struct {
	Package        string   `yaml:"package"`
	PackageMatches []string `yaml:"packageMatches"`
	Name           string   `yaml:"name"`
	NameMatches    []string `yaml:"nameMatches"`
	Pointer        *bool    `yaml:"pointer"`
	TypeMatches    []string `yaml:"typeMatches"`
}

func (cfg *Config) validateCallRules() error {
	for i, rule := range cfg.CallRules {
		if rule.ID == "" {
			return fmt.Errorf("callRules[%d].id is required", i)
		}
		if err := validateSeverity(rule.ID, rule.Severity); err != nil {
			return err
		}
		if len(rule.Files.Include) == 0 && len(rule.Files.Templates) == 0 {
			return fmt.Errorf("call rule %q must include at least one file pattern or template", rule.ID)
		}
		for _, name := range rule.Files.Templates {
			if _, ok := cfg.Templates[name]; !ok {
				return fmt.Errorf("call rule %q references undefined template %q", rule.ID, name)
			}
		}
		if rule.Message == "" {
			return fmt.Errorf("call rule %q message is required", rule.ID)
		}
		if rule.Callee.Package == "" {
			return fmt.Errorf("call rule %q: callee.package is required", rule.ID)
		}
		if rule.Callee.Name == "" {
			return fmt.Errorf("call rule %q: callee.name is required", rule.ID)
		}
		if !callConditionConfigured(rule.Require) && !callConditionConfigured(rule.Deny) {
			return fmt.Errorf("call rule %q: require or deny is required", rule.ID)
		}
		if rule.DenyNonMatching && len(rule.Require.Arguments) == 0 {
			return fmt.Errorf("call rule %q: denyNonMatching requires require.arguments", rule.ID)
		}
		if err := validateCallCondition(rule.ID, "require", rule.Require); err != nil {
			return err
		}
		if err := validateCallCondition(rule.ID, "deny", rule.Deny); err != nil {
			return err
		}
	}
	return nil
}

func callConditionConfigured(condition CallCondition) bool {
	return len(condition.Arguments) > 0 || countConfigured(condition.Count)
}

func validateCallCondition(ruleID, name string, condition CallCondition) error {
	if err := validateCount(ruleID, name+".count", condition.Count); err != nil {
		return err
	}
	for i, argument := range condition.Arguments {
		if argument.Position < 0 {
			return fmt.Errorf("call rule %q: %s.arguments[%d].position must be non-negative", ruleID, name, i)
		}
		if !resolvedTypeSelectorConfigured(argument.Type) {
			return fmt.Errorf("call rule %q: %s.arguments[%d].type is required", ruleID, name, i)
		}
		if err := validateCallRegexes(ruleID, name, i, argument.Type); err != nil {
			return err
		}
	}
	return nil
}

func resolvedTypeSelectorConfigured(selector ResolvedTypeSelector) bool {
	return selector.Package != "" || len(selector.PackageMatches) > 0 ||
		selector.Name != "" || len(selector.NameMatches) > 0 ||
		selector.Pointer != nil || len(selector.TypeMatches) > 0
}

func validateCallRegexes(ruleID, condition string, index int, selector ResolvedTypeSelector) error {
	for field, patterns := range map[string][]string{
		"packageMatches": selector.PackageMatches,
		"nameMatches":    selector.NameMatches,
		"typeMatches":    selector.TypeMatches,
	} {
		for _, pattern := range patterns {
			if _, err := regexp.Compile(pattern); err != nil {
				return fmt.Errorf("call rule %q has invalid %s.arguments[%d].type.%s regex %q: %w", ruleID, condition, index, field, pattern, err)
			}
		}
	}
	return nil
}

func checkCallRules(pass *analysis.Pass, file *ast.File, cfg *Config, paths []string) {
	for _, rule := range cfg.CallRules {
		if !fileSetAppliesToGenerated(rule.Files, file) || len(cfg.matchFileSet(rule.Files, paths)) == 0 {
			continue
		}
		var calls []*ast.CallExpr
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if ok && callTargetMatches(pass.TypesInfo, call, rule.Callee) {
				calls = append(calls, call)
			}
			return true
		})

		required := matchingCalls(pass.TypesInfo, calls, rule.Require.Arguments)
		if callConditionConfigured(rule.Require) && !countMatches(len(required), rule.Require.Count, false) {
			pass.Reportf(file.Package, "[%s]: %s required call count %s, found %d", rule.ID, rule.Message, describeCount(rule.Require.Count), len(required))
		}
		if rule.DenyNonMatching {
			for _, call := range calls {
				if !callArgumentsMatch(pass.TypesInfo, call, rule.Require.Arguments) {
					pass.Reportf(call.Pos(), "[%s]: %s selected call does not match required arguments", rule.ID, rule.Message)
				}
			}
		}
		if callConditionConfigured(rule.Deny) {
			denied := matchingCalls(pass.TypesInfo, calls, rule.Deny.Arguments)
			if countConfigured(rule.Deny.Count) {
				if countMatches(len(denied), rule.Deny.Count, false) {
					pass.Reportf(file.Package, "[%s]: %s denied call count %s matched %d", rule.ID, rule.Message, describeCount(rule.Deny.Count), len(denied))
				}
				continue
			}
			for _, call := range denied {
				pass.Reportf(call.Pos(), "[%s]: %s denied call matched", rule.ID, rule.Message)
			}
		}
	}
}

func callTargetMatches(info *types.Info, call *ast.CallExpr, target CallTarget) bool {
	function := calledFunction(info, call)
	return function != nil && function.Pkg() != nil && function.Pkg().Path() == target.Package && function.Name() == target.Name
}

func calledFunction(info *types.Info, call *ast.CallExpr) *types.Func {
	switch function := call.Fun.(type) {
	case *ast.Ident:
		resolved, _ := info.Uses[function].(*types.Func)
		return resolved
	case *ast.SelectorExpr:
		resolved, _ := info.Uses[function.Sel].(*types.Func)
		return resolved
	default:
		return nil
	}
}

func matchingCalls(info *types.Info, calls []*ast.CallExpr, arguments []CallArgumentSelector) []*ast.CallExpr {
	matched := make([]*ast.CallExpr, 0, len(calls))
	for _, call := range calls {
		if callArgumentsMatch(info, call, arguments) {
			matched = append(matched, call)
		}
	}
	return matched
}

func callArgumentsMatch(info *types.Info, call *ast.CallExpr, arguments []CallArgumentSelector) bool {
	for _, argument := range arguments {
		if argument.Position >= len(call.Args) || !resolvedTypeMatches(info.TypeOf(call.Args[argument.Position]), argument.Type) {
			return false
		}
	}
	return true
}

func resolvedTypeMatches(value types.Type, selector ResolvedTypeSelector) bool {
	if value == nil {
		return false
	}
	pointer := false
	base := value
	if pointerType, ok := base.(*types.Pointer); ok {
		pointer = true
		base = pointerType.Elem()
	}
	if selector.Pointer != nil && pointer != *selector.Pointer {
		return false
	}

	base = types.Unalias(base)
	named, _ := base.(*types.Named)
	packagePath := ""
	name := ""
	if named != nil && named.Obj() != nil {
		name = named.Obj().Name()
		if named.Obj().Pkg() != nil {
			packagePath = named.Obj().Pkg().Path()
		}
	}
	if selector.Package != "" && packagePath != selector.Package {
		return false
	}
	if len(selector.PackageMatches) > 0 && !matchesRegexAny(packagePath, selector.PackageMatches) {
		return false
	}
	if selector.Name != "" && name != selector.Name {
		return false
	}
	if len(selector.NameMatches) > 0 && !matchesRegexAny(name, selector.NameMatches) {
		return false
	}
	canonical := types.TypeString(value, func(pkg *types.Package) string { return pkg.Path() })
	return len(selector.TypeMatches) == 0 || matchesRegexAny(canonical, selector.TypeMatches)
}

func describeCount(condition CountCondition) string {
	if condition.Equals != nil {
		return fmt.Sprintf("%d", *condition.Equals)
	}
	parts := make([]string, 0, 2)
	if condition.Min != nil {
		parts = append(parts, fmt.Sprintf("min %d", *condition.Min))
	}
	if condition.Max != nil {
		parts = append(parts, fmt.Sprintf("max %d", *condition.Max))
	}
	if len(parts) == 0 {
		return "at least 1"
	}
	return strings.Join(parts, ", ")
}
