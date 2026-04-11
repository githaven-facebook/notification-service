package template

import (
	"fmt"
	"strings"
	"text/template"
)

// ValidationError represents a template validation error.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("template validation error: field=%s message=%s", e.Field, e.Message)
}

// ValidateSyntax checks that a template string is valid Go text/template syntax.
func ValidateSyntax(name, tmplStr string) error {
	if tmplStr == "" {
		return &ValidationError{Field: name, Message: "template body cannot be empty"}
	}

	_, err := template.New(name).Parse(tmplStr)
	if err != nil {
		return &ValidationError{Field: name, Message: fmt.Sprintf("invalid template syntax: %s", err.Error())}
	}
	return nil
}

// ValidateRequiredVariables checks that all required variables are present in the params map.
func ValidateRequiredVariables(tmplStr string, params map[string]string, required []string) error {
	missing := make([]string, 0)
	for _, key := range required {
		if _, ok := params[key]; !ok {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return &ValidationError{
			Field:   "params",
			Message: fmt.Sprintf("missing required template variables: %s", strings.Join(missing, ", ")),
		}
	}
	return nil
}

// ExtractVariables parses a template string and returns all variable names referenced.
func ExtractVariables(tmplStr string) ([]string, error) {
	tmpl, err := template.New("extract").Parse(tmplStr)
	if err != nil {
		return nil, fmt.Errorf("parse template for variable extraction: %w", err)
	}

	vars := make(map[string]struct{})
	extractVarsFromTree(tmpl, vars)

	result := make([]string, 0, len(vars))
	for v := range vars {
		result = append(result, v)
	}
	return result, nil
}

// extractVarsFromTree walks the template parse tree to find variable references.
// This is a simplified implementation using string parsing.
func extractVarsFromTree(tmpl *template.Template, vars map[string]struct{}) {
	// Walk through the template tree to extract .VarName references
	if tmpl.Tree == nil {
		return
	}

	// Parse the raw template string to find {{.VarName}} patterns
	raw := tmpl.Tree.Root.String()
	// Simple extraction: look for .VarName patterns
	i := 0
	for i < len(raw) {
		dot := strings.Index(raw[i:], ".")
		if dot == -1 {
			break
		}
		start := i + dot + 1
		end := start
		for end < len(raw) && (raw[end] == '_' || (raw[end] >= 'A' && raw[end] <= 'Z') || (raw[end] >= 'a' && raw[end] <= 'z') || (raw[end] >= '0' && raw[end] <= '9')) {
			end++
		}
		if end > start {
			vars[raw[start:end]] = struct{}{}
		}
		i = start
	}
}
