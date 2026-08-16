package filearch

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"regexp"
	"strconv"

	"gopkg.in/yaml.v3"
)

type DeclarationSelector struct {
	Kind             string             `yaml:"kind"`
	Name             string             `yaml:"name"`
	NameMatches      []string           `yaml:"nameMatches"`
	NameNotMatches   []string           `yaml:"nameNotMatches"`
	Exported         *bool              `yaml:"exported"`
	Receiver         ReceiverCondition  `yaml:"receiver"`
	Parameters       ParameterCondition `yaml:"parameters"`
	Returns          ReturnCondition    `yaml:"returns"`
	Embeds           EmbedCondition     `yaml:"embeds"`
	Fields           FieldCondition     `yaml:"fields"`
	Methods          MethodCondition    `yaml:"methods"`
	Count            CountCondition     `yaml:"count"`
	Value            ValueCondition     `yaml:"value"`
	Underlying       TypeCondition      `yaml:"underlying"`
	Initialized      *bool              `yaml:"initialized"`
	InitializerCalls []string           `yaml:"initializerCalls"`
}

type CountCondition struct {
	Equals *int `yaml:"equals"`
	Min    *int `yaml:"min"`
	Max    *int `yaml:"max"`
}

type TypeCondition struct {
	Name           string   `yaml:"name"`
	Type           string   `yaml:"type"`
	TypeMatches    []string `yaml:"typeMatches"`
	TypeNotMatches []string `yaml:"typeNotMatches"`
	ExportedType   *bool    `yaml:"exportedType"`
}

type TypeConditionList []TypeCondition

func (conditions *TypeConditionList) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.SequenceNode {
		return fmt.Errorf("type condition list must be a sequence")
	}
	values := make(TypeConditionList, 0, len(node.Content))
	for _, item := range node.Content {
		var condition TypeCondition
		if err := item.Decode(&condition); err != nil {
			return err
		}
		values = append(values, condition)
	}
	*conditions = values
	return nil
}

func (condition *TypeCondition) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		if node.Tag != "!!str" {
			return fmt.Errorf("type condition scalar must be a string")
		}
		condition.Type = node.Value
		return nil
	}
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("type condition must be a string or mapping")
	}
	allowed := map[string]bool{"name": true, "type": true, "typeMatches": true, "typeNotMatches": true, "exportedType": true}
	for i := 0; i < len(node.Content); i += 2 {
		if key := node.Content[i].Value; !allowed[key] {
			return fmt.Errorf("field %s not found in type filearch.TypeCondition", key)
		}
	}
	type plain TypeCondition
	return node.Decode((*plain)(condition))
}

type ReceiverCondition struct {
	Present        *bool    `yaml:"present"`
	Pointer        *bool    `yaml:"pointer"`
	Type           string   `yaml:"type"`
	TypeMatches    []string `yaml:"typeMatches"`
	TypeNotMatches []string `yaml:"typeNotMatches"`
}

type ParameterCondition struct {
	Count    CountCondition    `yaml:"count"`
	First    *TypeCondition    `yaml:"first"`
	Contains TypeConditionList `yaml:"contains"`
	All      *TypeCondition    `yaml:"all"`
}

type ReturnCondition struct {
	Count    CountCondition    `yaml:"count"`
	First    *TypeCondition    `yaml:"first"`
	Contains TypeConditionList `yaml:"contains"`
	All      *TypeCondition    `yaml:"all"`
	Matches  []string          `yaml:"matches"`
}

type EmbedCondition struct {
	Count    CountCondition    `yaml:"count"`
	Contains TypeConditionList `yaml:"contains"`
	All      *TypeCondition    `yaml:"all"`
}

type FieldCondition struct {
	Count    CountCondition  `yaml:"count"`
	Contains []FieldSelector `yaml:"contains"`
	All      *FieldSelector  `yaml:"all"`
}

type FieldSelector struct {
	Name         string   `yaml:"name"`
	NameMatches  []string `yaml:"nameMatches"`
	Type         string   `yaml:"type"`
	TypeMatches  []string `yaml:"typeMatches"`
	Exported     *bool    `yaml:"exported"`
	ExportedType *bool    `yaml:"exportedType"`
	TagMatches   []string `yaml:"tagMatches"`
}

type MethodCondition struct {
	Count    CountCondition      `yaml:"count"`
	Contains []FunctionCondition `yaml:"contains"`
	All      *FunctionCondition  `yaml:"all"`
}

type FunctionCondition struct {
	Name        string             `yaml:"name"`
	NameMatches []string           `yaml:"nameMatches"`
	Parameters  ParameterCondition `yaml:"parameters"`
	Returns     ReturnCondition    `yaml:"returns"`
}

type ValueCondition struct {
	Equals  *string  `yaml:"equals"`
	Matches []string `yaml:"matches"`
}

type typedCandidate struct {
	Name string
	Type string
}

type fieldCandidate struct {
	Name     string
	Type     string
	Exported bool
	Tag      string
}

type receiverCandidate struct {
	Type    string
	Pointer bool
}

type functionCandidate struct {
	Name       string
	Parameters []typedCandidate
	Results    []typedCandidate
}

type declarationCandidate struct {
	Kind             string
	Name             string
	Exported         bool
	Pos              token.Pos
	Receiver         *receiverCandidate
	Parameters       []typedCandidate
	Results          []typedCandidate
	Embeds           []typedCandidate
	Fields           []fieldCandidate
	Methods          []functionCandidate
	Value            *string
	Underlying       string
	Initialized      bool
	InitializerCalls []string
	GroupID          token.Pos
	Grouped          bool
	GroupSize        int
}

func extractDeclarationCandidates(fset *token.FileSet, file *ast.File) []declarationCandidate {
	var candidates []declarationCandidate
	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			signature := extractFunction(fset, decl.Name.Name, decl.Type)
			candidate := declarationCandidate{
				Kind: "func", Name: decl.Name.Name, Exported: ast.IsExported(decl.Name.Name),
				Pos: decl.Pos(), Parameters: signature.Parameters, Results: signature.Results,
			}
			if decl.Recv != nil && len(decl.Recv.List) > 0 {
				receiverType := decl.Recv.List[0].Type
				candidate.Receiver = &receiverCandidate{Type: renderExpr(fset, receiverType)}
				if star, ok := receiverType.(*ast.StarExpr); ok {
					candidate.Receiver.Pointer = true
					candidate.Receiver.Type = renderExpr(fset, star.X)
				}
			}
			candidates = append(candidates, candidate)
		case *ast.GenDecl:
			groupID := decl.Pos()
			grouped := decl.Lparen.IsValid()
			groupSize := declarationGroupSize(decl)
			switch decl.Tok {
			case token.TYPE:
				for _, spec := range decl.Specs {
					typeSpec := spec.(*ast.TypeSpec)
					kind := declarationKind(typeSpec)
					candidate := declarationCandidate{
						Kind: kind, Name: typeSpec.Name.Name,
						Exported: ast.IsExported(typeSpec.Name.Name), Pos: typeSpec.Pos(),
						GroupID: groupID, Grouped: grouped, GroupSize: groupSize,
					}
					if kind != "alias" {
						candidate.Underlying = renderExpr(fset, typeSpec.Type)
					}
					switch value := typeSpec.Type.(type) {
					case *ast.StructType:
						candidate.Embeds = embeddedTypes(fset, value.Fields)
						candidate.Fields = structFields(fset, value.Fields)
					case *ast.InterfaceType:
						candidate.Embeds, candidate.Methods = interfaceMembers(fset, value)
					}
					candidates = append(candidates, candidate)
				}
			case token.CONST, token.VAR:
				kind := "var"
				if decl.Tok == token.CONST {
					kind = "const"
				}
				for _, spec := range decl.Specs {
					valueSpec := spec.(*ast.ValueSpec)
					initialized := len(valueSpec.Values) > 0
					for i, name := range valueSpec.Names {
						var value *string
						if kind == "const" && i < len(valueSpec.Values) {
							value = literalValue(valueSpec.Values[i])
						}
						var initializerCalls []string
						if len(valueSpec.Values) == 1 {
							initializerCalls = calledFunctions(fset, valueSpec.Values[0])
						} else if i < len(valueSpec.Values) {
							initializerCalls = calledFunctions(fset, valueSpec.Values[i])
						}
						candidates = append(candidates, declarationCandidate{
							Kind: kind, Name: name.Name, Exported: ast.IsExported(name.Name), Pos: name.Pos(), Value: value, Initialized: initialized,
							InitializerCalls: initializerCalls,
							GroupID:          groupID, Grouped: grouped, GroupSize: groupSize,
						})
					}
				}
			}
		}
	}
	return candidates
}

func calledFunctions(fset *token.FileSet, expression ast.Expr) []string {
	var calls []string
	ast.Inspect(expression, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if ok {
			calls = append(calls, renderExpr(fset, call.Fun))
		}
		return true
	})
	return calls
}

func declarationGroupSize(decl *ast.GenDecl) int {
	size := 0
	for _, spec := range decl.Specs {
		switch spec := spec.(type) {
		case *ast.TypeSpec:
			size++
		case *ast.ValueSpec:
			size += len(spec.Names)
		}
	}
	return size
}

func extractFunction(fset *token.FileSet, name string, function *ast.FuncType) functionCandidate {
	return functionCandidate{Name: name, Parameters: typedFields(fset, function.Params), Results: typedFields(fset, function.Results)}
}

func typedFields(fset *token.FileSet, fields *ast.FieldList) []typedCandidate {
	if fields == nil {
		return nil
	}
	var values []typedCandidate
	for _, field := range fields.List {
		typeName := renderExpr(fset, field.Type)
		if len(field.Names) == 0 {
			values = append(values, typedCandidate{Type: typeName})
			continue
		}
		for _, name := range field.Names {
			values = append(values, typedCandidate{Name: name.Name, Type: typeName})
		}
	}
	return values
}

func embeddedTypes(fset *token.FileSet, fields *ast.FieldList) []typedCandidate {
	if fields == nil {
		return nil
	}
	var embeds []typedCandidate
	for _, field := range fields.List {
		if len(field.Names) == 0 {
			embeds = append(embeds, typedCandidate{Type: renderExpr(fset, field.Type)})
		}
	}
	return embeds
}

func structFields(fset *token.FileSet, fields *ast.FieldList) []fieldCandidate {
	if fields == nil {
		return nil
	}
	var values []fieldCandidate
	for _, field := range fields.List {
		tag := ""
		if field.Tag != nil {
			tag, _ = strconv.Unquote(field.Tag.Value)
		}
		typeName := renderExpr(fset, field.Type)
		if len(field.Names) == 0 {
			name := embeddedFieldName(field.Type)
			values = append(values, fieldCandidate{
				Name: name, Type: typeName, Exported: ast.IsExported(name), Tag: tag,
			})
			continue
		}
		for _, name := range field.Names {
			values = append(values, fieldCandidate{
				Name: name.Name, Type: typeName, Exported: ast.IsExported(name.Name), Tag: tag,
			})
		}
	}
	return values
}

func embeddedFieldName(expr ast.Expr) string {
	switch value := expr.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.StarExpr:
		return embeddedFieldName(value.X)
	case *ast.SelectorExpr:
		return value.Sel.Name
	case *ast.IndexExpr:
		return embeddedFieldName(value.X)
	case *ast.IndexListExpr:
		return embeddedFieldName(value.X)
	default:
		return ""
	}
}

func interfaceMembers(fset *token.FileSet, value *ast.InterfaceType) ([]typedCandidate, []functionCandidate) {
	var embeds []typedCandidate
	var methods []functionCandidate
	for _, field := range value.Methods.List {
		if len(field.Names) == 0 {
			embeds = append(embeds, typedCandidate{Type: renderExpr(fset, field.Type)})
			continue
		}
		function, ok := field.Type.(*ast.FuncType)
		if !ok {
			continue
		}
		for _, name := range field.Names {
			methods = append(methods, extractFunction(fset, name.Name, function))
		}
	}
	return embeds, methods
}

func renderExpr(fset *token.FileSet, expr ast.Expr) string {
	var buffer bytes.Buffer
	if err := format.Node(&buffer, fset, expr); err != nil {
		return ""
	}
	return buffer.String()
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
	if candidate.Kind != selector.Kind || (selector.Name != "" && candidate.Name != selector.Name) {
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
	if underlyingConfigured(selector.Underlying) && !typeMatches(typedCandidate{Type: candidate.Underlying}, selector.Underlying) {
		return false
	}
	if selector.Initialized != nil && candidate.Initialized != *selector.Initialized {
		return false
	}
	if len(selector.InitializerCalls) > 0 && !matchesRegexAnyOf(candidate.InitializerCalls, selector.InitializerCalls) {
		return false
	}
	if !receiverMatches(candidate.Receiver, selector.Receiver) {
		return false
	}
	if !typedSequenceMatches(candidate.Parameters, selector.Parameters.Count, selector.Parameters.First, selector.Parameters.Contains, selector.Parameters.All) {
		return false
	}
	if !returnSequenceMatches(candidate.Results, selector.Returns) {
		return false
	}
	if !typedSequenceMatches(candidate.Embeds, selector.Embeds.Count, nil, selector.Embeds.Contains, selector.Embeds.All) {
		return false
	}
	if !fieldsMatch(candidate.Fields, selector.Fields) {
		return false
	}
	if !methodsMatch(candidate.Methods, selector.Methods) {
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

func fieldsMatch(fields []fieldCandidate, condition FieldCondition) bool {
	if !countMatches(len(fields), condition.Count, true) {
		return false
	}
	for _, required := range condition.Contains {
		matched := false
		for _, field := range fields {
			if fieldMatches(field, required) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if condition.All != nil {
		for _, field := range fields {
			if !fieldMatches(field, *condition.All) {
				return false
			}
		}
	}
	return true
}

func fieldMatches(field fieldCandidate, selector FieldSelector) bool {
	if selector.Name != "" && field.Name != selector.Name {
		return false
	}
	if len(selector.NameMatches) > 0 && !matchesRegexAny(field.Name, selector.NameMatches) {
		return false
	}
	if selector.Type != "" && field.Type != selector.Type {
		return false
	}
	if len(selector.TypeMatches) > 0 && !matchesRegexAny(field.Type, selector.TypeMatches) {
		return false
	}
	if selector.Exported != nil && field.Exported != *selector.Exported {
		return false
	}
	if selector.ExportedType != nil && exportedType(field.Type) != *selector.ExportedType {
		return false
	}
	return len(selector.TagMatches) == 0 || matchesRegexAny(field.Tag, selector.TagMatches)
}

func receiverMatches(candidate *receiverCandidate, condition ReceiverCondition) bool {
	configured := receiverConfigured(condition)
	if !configured {
		return true
	}
	if condition.Present != nil && (*condition.Present != (candidate != nil)) {
		return false
	}
	if candidate == nil {
		return condition.Present != nil && !*condition.Present
	}
	if condition.Pointer != nil && candidate.Pointer != *condition.Pointer {
		return false
	}
	if condition.Type != "" && candidate.Type != condition.Type {
		return false
	}
	if len(condition.TypeMatches) > 0 && !matchesRegexAny(candidate.Type, condition.TypeMatches) {
		return false
	}
	return !matchesRegexAny(candidate.Type, condition.TypeNotMatches)
}

func typedSequenceMatches(values []typedCandidate, count CountCondition, first *TypeCondition, contains []TypeCondition, all *TypeCondition) bool {
	if !countMatches(len(values), count, true) {
		return false
	}
	if first != nil && (len(values) == 0 || !typeMatches(values[0], *first)) {
		return false
	}
	for _, condition := range contains {
		matched := false
		for _, value := range values {
			if typeMatches(value, condition) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if all != nil {
		for _, value := range values {
			if !typeMatches(value, *all) {
				return false
			}
		}
	}
	return true
}

func typeMatches(value typedCandidate, condition TypeCondition) bool {
	if condition.Name != "" && value.Name != condition.Name {
		return false
	}
	if condition.Type != "" && value.Type != condition.Type {
		return false
	}
	if len(condition.TypeMatches) > 0 && !matchesRegexAny(value.Type, condition.TypeMatches) {
		return false
	}
	if matchesRegexAny(value.Type, condition.TypeNotMatches) {
		return false
	}
	if condition.ExportedType != nil && exportedType(value.Type) != *condition.ExportedType {
		return false
	}
	return true
}

func exportedType(value string) bool {
	identifier := regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*$`).FindString(value)
	return identifier != "" && ast.IsExported(identifier)
}

func methodsMatch(methods []functionCandidate, condition MethodCondition) bool {
	if !countMatches(len(methods), condition.Count, true) {
		return false
	}
	for _, required := range condition.Contains {
		matched := false
		for _, method := range methods {
			if functionMatches(method, required) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if condition.All != nil {
		for _, method := range methods {
			if !functionMatches(method, *condition.All) {
				return false
			}
		}
	}
	return true
}

func functionMatches(function functionCandidate, condition FunctionCondition) bool {
	if condition.Name != "" && function.Name != condition.Name {
		return false
	}
	if len(condition.NameMatches) > 0 && !matchesRegexAny(function.Name, condition.NameMatches) {
		return false
	}
	return typedSequenceMatches(function.Parameters, condition.Parameters.Count, condition.Parameters.First, condition.Parameters.Contains, condition.Parameters.All) &&
		returnSequenceMatches(function.Results, condition.Returns)
}

func returnSequenceMatches(results []typedCandidate, condition ReturnCondition) bool {
	if !typedSequenceMatches(results, condition.Count, condition.First, condition.Contains, condition.All) {
		return false
	}
	if len(condition.Matches) > 0 {
		for _, result := range results {
			if matchesRegexAny(result.Type, condition.Matches) {
				return true
			}
		}
		return false
	}
	return true
}

func anyDeclarationMatches(candidates []declarationCandidate, selector DeclarationSelector) bool {
	return countMatches(len(matchingDeclarationCandidates(candidates, selector)), selector.Count, false)
}

func matchingDeclarationCandidates(candidates []declarationCandidate, selector DeclarationSelector) []declarationCandidate {
	var matched []declarationCandidate
	for _, candidate := range candidates {
		if declarationMatches(candidate, selector) {
			matched = append(matched, candidate)
		}
	}
	return matched
}

func countMatches(actual int, condition CountCondition, unconstrained bool) bool {
	if condition.Equals == nil && condition.Min == nil && condition.Max == nil {
		return unconstrained || actual >= 1
	}
	if condition.Equals != nil && actual != *condition.Equals {
		return false
	}
	if condition.Min != nil && actual < *condition.Min {
		return false
	}
	if condition.Max != nil && actual > *condition.Max {
		return false
	}
	return true
}

func validateDeclarationSelector(ruleID string, selector DeclarationSelector) error {
	if !validDeclarationKinds[selector.Kind] || selector.Kind == "package" || selector.Kind == "import" {
		return fmt.Errorf("file contract rule %q has unsupported declaration kind %q", ruleID, selector.Kind)
	}
	if selector.Kind != "func" && receiverConfigured(selector.Receiver) {
		return fmt.Errorf("file contract rule %q: receiver is only valid for func", ruleID)
	}
	if selector.Receiver.Present != nil && !*selector.Receiver.Present &&
		(selector.Receiver.Pointer != nil || selector.Receiver.Type != "" || len(selector.Receiver.TypeMatches) > 0 || len(selector.Receiver.TypeNotMatches) > 0) {
		return fmt.Errorf("file contract rule %q: receiver.present false cannot have receiver shape constraints", ruleID)
	}
	if selector.Kind != "func" && parameterConfigured(selector.Parameters) {
		return fmt.Errorf("file contract rule %q: parameters is only valid for func", ruleID)
	}
	if selector.Kind != "func" && returnConfigured(selector.Returns) {
		return fmt.Errorf("file contract rule %q: returns is only valid for func", ruleID)
	}
	if selector.Kind != "struct" && selector.Kind != "interface" && embedConfigured(selector.Embeds) {
		return fmt.Errorf("file contract rule %q: embeds is only valid for struct or interface", ruleID)
	}
	if selector.Kind != "struct" && fieldConfigured(selector.Fields) {
		return fmt.Errorf("file contract rule %q: fields is only valid for struct", ruleID)
	}
	if selector.Kind != "interface" && methodConfigured(selector.Methods) {
		return fmt.Errorf("file contract rule %q: methods is only valid for interface", ruleID)
	}
	if selector.Kind != "const" && (selector.Value.Equals != nil || len(selector.Value.Matches) > 0) {
		return fmt.Errorf("file contract rule %q: value is only valid for const", ruleID)
	}
	if selector.Kind != "type" && underlyingConfigured(selector.Underlying) {
		return fmt.Errorf("file contract rule %q: underlying is only valid for type", ruleID)
	}
	if selector.Initialized != nil && selector.Kind != "var" && selector.Kind != "const" {
		return fmt.Errorf("file contract rule %q: initialized is only valid for var or const", ruleID)
	}
	if len(selector.InitializerCalls) > 0 && selector.Kind != "var" && selector.Kind != "const" {
		return fmt.Errorf("file contract rule %q: initializerCalls is only valid for var or const", ruleID)
	}
	if err := validateCount(ruleID, "count", selector.Count); err != nil {
		return err
	}
	if err := validateRegexes(ruleID, "nameMatches", selector.NameMatches); err != nil {
		return err
	}
	if err := validateRegexes(ruleID, "nameNotMatches", selector.NameNotMatches); err != nil {
		return err
	}
	if err := validateRegexes(ruleID, "initializerCalls", selector.InitializerCalls); err != nil {
		return err
	}
	if err := validateRegexes(ruleID, "receiver.typeMatches", selector.Receiver.TypeMatches); err != nil {
		return err
	}
	if err := validateRegexes(ruleID, "receiver.typeNotMatches", selector.Receiver.TypeNotMatches); err != nil {
		return err
	}
	if err := validateTypeCondition(ruleID, "underlying", selector.Underlying); err != nil {
		return err
	}
	if err := validateParameterCondition(ruleID, "parameters", selector.Parameters); err != nil {
		return err
	}
	if err := validateReturnCondition(ruleID, "returns", selector.Returns); err != nil {
		return err
	}
	if err := validateEmbedCondition(ruleID, selector.Embeds); err != nil {
		return err
	}
	if err := validateFieldCondition(ruleID, selector.Fields); err != nil {
		return err
	}
	if err := validateMethodCondition(ruleID, selector.Methods); err != nil {
		return err
	}
	if err := validateRegexes(ruleID, "value.matches", selector.Value.Matches); err != nil {
		return err
	}
	return nil
}

func matchesRegexAnyOf(values, patterns []string) bool {
	for _, value := range values {
		if matchesRegexAny(value, patterns) {
			return true
		}
	}
	return false
}

func validateFieldCondition(ruleID string, condition FieldCondition) error {
	if err := validateCount(ruleID, "fields.count", condition.Count); err != nil {
		return err
	}
	for i, selector := range condition.Contains {
		if err := validateFieldSelector(ruleID, fmt.Sprintf("fields.contains[%d]", i), selector); err != nil {
			return err
		}
	}
	if condition.All != nil {
		return validateFieldSelector(ruleID, "fields.all", *condition.All)
	}
	return nil
}

func validateFieldSelector(ruleID, name string, selector FieldSelector) error {
	for field, patterns := range map[string][]string{
		"nameMatches": selector.NameMatches,
		"typeMatches": selector.TypeMatches,
		"tagMatches":  selector.TagMatches,
	} {
		if err := validateRegexes(ruleID, name+"."+field, patterns); err != nil {
			return err
		}
	}
	return nil
}

func validateRegexes(ruleID, name string, patterns []string) error {
	for _, pattern := range patterns {
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("file contract rule %q has invalid %s regex %q: %w", ruleID, name, pattern, err)
		}
	}
	return nil
}

func validateParameterCondition(ruleID, name string, condition ParameterCondition) error {
	if err := validateCount(ruleID, name+".count", condition.Count); err != nil {
		return err
	}
	return validateTypeConditions(ruleID, name, condition.First, condition.Contains, condition.All)
}

func validateReturnCondition(ruleID, name string, condition ReturnCondition) error {
	if err := validateCount(ruleID, name+".count", condition.Count); err != nil {
		return err
	}
	if err := validateTypeConditions(ruleID, name, condition.First, condition.Contains, condition.All); err != nil {
		return err
	}
	return validateRegexes(ruleID, name+".matches", condition.Matches)
}

func validateEmbedCondition(ruleID string, condition EmbedCondition) error {
	if err := validateCount(ruleID, "embeds.count", condition.Count); err != nil {
		return err
	}
	return validateTypeConditions(ruleID, "embeds", nil, condition.Contains, condition.All)
}

func validateMethodCondition(ruleID string, condition MethodCondition) error {
	if err := validateCount(ruleID, "methods.count", condition.Count); err != nil {
		return err
	}
	for i, method := range condition.Contains {
		if err := validateFunctionCondition(ruleID, fmt.Sprintf("methods.contains[%d]", i), method); err != nil {
			return err
		}
	}
	if condition.All != nil {
		return validateFunctionCondition(ruleID, "methods.all", *condition.All)
	}
	return nil
}

func validateFunctionCondition(ruleID, name string, condition FunctionCondition) error {
	if err := validateRegexes(ruleID, name+".nameMatches", condition.NameMatches); err != nil {
		return err
	}
	if err := validateParameterCondition(ruleID, name+".parameters", condition.Parameters); err != nil {
		return err
	}
	return validateReturnCondition(ruleID, name+".returns", condition.Returns)
}

func validateTypeConditions(ruleID, name string, first *TypeCondition, contains TypeConditionList, all *TypeCondition) error {
	conditions := append(TypeConditionList{}, contains...)
	if first != nil {
		conditions = append(conditions, *first)
	}
	if all != nil {
		conditions = append(conditions, *all)
	}
	for i, condition := range conditions {
		if err := validateTypeCondition(ruleID, fmt.Sprintf("%s.typeConditions[%d]", name, i), condition); err != nil {
			return err
		}
	}
	return nil
}

func validateTypeCondition(ruleID, name string, condition TypeCondition) error {
	if err := validateRegexes(ruleID, name+".typeMatches", condition.TypeMatches); err != nil {
		return err
	}
	return validateRegexes(ruleID, name+".typeNotMatches", condition.TypeNotMatches)
}

func validateCount(ruleID, name string, condition CountCondition) error {
	for _, value := range []*int{condition.Equals, condition.Min, condition.Max} {
		if value != nil && *value < 0 {
			return fmt.Errorf("file contract rule %q: %s values must be non-negative", ruleID, name)
		}
	}
	if condition.Equals != nil && (condition.Min != nil || condition.Max != nil) {
		return fmt.Errorf("file contract rule %q: %s.equals cannot be combined with min or max", ruleID, name)
	}
	if condition.Min != nil && condition.Max != nil && *condition.Min > *condition.Max {
		return fmt.Errorf("file contract rule %q: %s.min exceeds max", ruleID, name)
	}
	return nil
}

func receiverConfigured(value ReceiverCondition) bool {
	return value.Present != nil || value.Pointer != nil || value.Type != "" || len(value.TypeMatches) > 0 || len(value.TypeNotMatches) > 0
}
func underlyingConfigured(value TypeCondition) bool {
	return value.Name != "" || value.Type != "" || len(value.TypeMatches) > 0 || len(value.TypeNotMatches) > 0 || value.ExportedType != nil
}
func parameterConfigured(value ParameterCondition) bool {
	return value.First != nil || len(value.Contains) > 0 || value.All != nil || countConfigured(value.Count)
}
func returnConfigured(value ReturnCondition) bool {
	return value.First != nil || len(value.Contains) > 0 || value.All != nil || len(value.Matches) > 0 || countConfigured(value.Count)
}
func embedConfigured(value EmbedCondition) bool {
	return len(value.Contains) > 0 || value.All != nil || countConfigured(value.Count)
}
func fieldConfigured(value FieldCondition) bool {
	return len(value.Contains) > 0 || value.All != nil || countConfigured(value.Count)
}
func methodConfigured(value MethodCondition) bool {
	return len(value.Contains) > 0 || value.All != nil || countConfigured(value.Count)
}
func countConfigured(value CountCondition) bool {
	return value.Equals != nil || value.Min != nil || value.Max != nil
}
