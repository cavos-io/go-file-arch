package filearch

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

type PathTemplate struct {
	Pattern  string            `yaml:"pattern"`
	Captures map[string]string `yaml:"captures"`
}

type TemplateMatch struct {
	Path     string
	Captures map[string]string
}

type compiledPathTemplate struct {
	expression *regexp.Regexp
	captures   []string
}

var capturePattern = regexp.MustCompile(`\{([A-Za-z][A-Za-z0-9_]*)\}`)
var expansionPattern = regexp.MustCompile(`\{([A-Za-z][A-Za-z0-9_]*)(?:\|([a-z]+))?\}`)

func (cfg *Config) validateTemplates() error {
	cfg.compiledTemplates = make(map[string]compiledPathTemplate, len(cfg.Templates))
	names := make([]string, 0, len(cfg.Templates))
	for name := range cfg.Templates {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		compiled, err := compilePathTemplate(name, cfg.Templates[name])
		if err != nil {
			return err
		}
		cfg.compiledTemplates[name] = compiled
	}
	return nil
}

func compilePathTemplate(name string, template PathTemplate) (compiledPathTemplate, error) {
	if name == "" {
		return compiledPathTemplate{}, fmt.Errorf("template name is required")
	}
	if template.Pattern == "" {
		return compiledPathTemplate{}, fmt.Errorf("template %q pattern is required", name)
	}
	pattern := filepath.ToSlash(template.Pattern)
	indices := capturePattern.FindAllStringSubmatchIndex(pattern, -1)
	var expression strings.Builder
	expression.WriteString("^")
	last := 0
	seen := make(map[string]bool)
	var captures []string
	for _, index := range indices {
		expression.WriteString(regexp.QuoteMeta(pattern[last:index[0]]))
		capture := pattern[index[2]:index[3]]
		if seen[capture] {
			return compiledPathTemplate{}, fmt.Errorf("template %q has duplicate capture %q", name, capture)
		}
		constraint, ok := template.Captures[capture]
		if !ok || constraint == "" {
			return compiledPathTemplate{}, fmt.Errorf("template %q capture %q has no constraint", name, capture)
		}
		constraint = strings.TrimPrefix(strings.TrimSuffix(constraint, "$"), "^")
		if _, err := regexp.Compile(constraint); err != nil {
			return compiledPathTemplate{}, fmt.Errorf("template %q capture %q has invalid constraint: %w", name, capture, err)
		}
		expression.WriteString("(?P<")
		expression.WriteString(capture)
		expression.WriteString(">")
		expression.WriteString(constraint)
		expression.WriteString(")")
		seen[capture] = true
		captures = append(captures, capture)
		last = index[1]
	}
	expression.WriteString(regexp.QuoteMeta(pattern[last:]))
	expression.WriteString("$")
	compiled, err := regexp.Compile(expression.String())
	if err != nil {
		return compiledPathTemplate{}, fmt.Errorf("template %q pattern is invalid: %w", name, err)
	}
	return compiledPathTemplate{expression: compiled, captures: captures}, nil
}

func (cfg *Config) matchFileSet(files FileSet, paths []string) []TemplateMatch {
	var matches []TemplateMatch
	seen := make(map[string]bool)
	for _, path := range paths {
		path = filepath.ToSlash(path)
		if MatchesAnyGlob(path, files.Exclude) {
			continue
		}
		if MatchesAnyGlob(path, files.Include) {
			key := "literal:" + path
			if !seen[key] {
				seen[key] = true
				matches = append(matches, TemplateMatch{Path: path, Captures: map[string]string{}})
			}
		}
		for _, name := range files.Templates {
			compiled, ok := cfg.compiledTemplates[name]
			if !ok {
				continue
			}
			parts := compiled.expression.FindStringSubmatch(path)
			if parts == nil {
				continue
			}
			key := name + ":" + path
			if seen[key] {
				continue
			}
			captures := make(map[string]string, len(compiled.captures))
			for _, capture := range compiled.captures {
				captures[capture] = parts[compiled.expression.SubexpIndex(capture)]
			}
			seen[key] = true
			matches = append(matches, TemplateMatch{Path: path, Captures: captures})
		}
	}
	return matches
}

func expandTemplate(value string, captures map[string]string) (string, error) {
	var expansionErr error
	expanded := expansionPattern.ReplaceAllStringFunc(value, func(token string) string {
		parts := expansionPattern.FindStringSubmatch(token)
		captured, ok := captures[parts[1]]
		if !ok {
			expansionErr = fmt.Errorf("undefined capture %q", parts[1])
			return token
		}
		switch parts[2] {
		case "":
			return captured
		case "snake":
			return strings.Join(templateWords(captured), "_")
		case "camel":
			words := templateWords(captured)
			if len(words) == 0 {
				return ""
			}
			return words[0] + pascalWords(words[1:])
		case "pascal":
			return pascalWords(templateWords(captured))
		default:
			expansionErr = fmt.Errorf("unknown transform %q", parts[2])
			return token
		}
	})
	if expansionErr != nil {
		return "", expansionErr
	}
	return expanded, nil
}

func templateWords(value string) []string {
	var normalized strings.Builder
	var previous rune
	for _, current := range value {
		if current == '_' || current == '-' || unicode.IsSpace(current) {
			normalized.WriteRune(' ')
			previous = current
			continue
		}
		if unicode.IsUpper(current) && (unicode.IsLower(previous) || unicode.IsDigit(previous)) {
			normalized.WriteRune(' ')
		}
		normalized.WriteRune(unicode.ToLower(current))
		previous = current
	}
	return strings.Fields(normalized.String())
}

func pascalWords(words []string) string {
	var result strings.Builder
	for _, word := range words {
		if word == "" {
			continue
		}
		runes := []rune(word)
		result.WriteRune(unicode.ToUpper(runes[0]))
		result.WriteString(string(runes[1:]))
	}
	return result.String()
}
