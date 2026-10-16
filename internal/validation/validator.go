package validation

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

type Schema struct {
	Required   []string          `json:"required"`
	Properties map[string]string `json:"properties"`
	Types      map[string]string `json:"types"`
}

type Validator struct {
	schemas map[string]Schema
}

func NewValidator() *Validator {
	return &Validator{
		schemas: make(map[string]Schema),
	}
}

func (v *Validator) RegisterSchema(name string, schema Schema) {
	v.schemas[name] = schema
}

func (v *Validator) Validate(schemaName string, data []byte) error {
	schema, ok := v.schemas[schemaName]
	if !ok {
		return fmt.Errorf("schema not found: %s", schemaName)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	for _, field := range schema.Required {
		if _, ok := parsed[field]; !ok {
			return fmt.Errorf("missing required field: %s", field)
		}
	}

	for field, fieldType := range schema.Types {
		val, ok := parsed[field]
		if !ok {
			continue
		}
		if !checkType(val, fieldType) {
			return fmt.Errorf("field %s: expected %s, got %T", field, fieldType, val)
		}
	}

	return nil
}

func checkType(val any, expected string) bool {
	switch expected {
	case "string":
		_, ok := val.(string)
		return ok
	case "number":
		_, ok := val.(float64)
		return ok
	case "boolean":
		_, ok := val.(bool)
		return ok
	case "array":
		_, ok := val.([]any)
		return ok
	case "object":
		_, ok := val.(map[string]any)
		return ok
	case "null":
		return val == nil
	}
	return true
}

type OutputSanitizer struct {
	maxFieldLength  int
	maxStringLength int
	maxDepth        int
}

func NewOutputSanitizer() *OutputSanitizer {
	return &OutputSanitizer{
		maxFieldLength:  256,
		maxStringLength: 10000,
		maxDepth:        5,
	}
}

func (s *OutputSanitizer) SanitizeString(input string) string {
	input = strings.TrimSpace(input)

	if utf8.RuneCountInString(input) > s.maxStringLength {
		input = string([]rune(input)[:s.maxStringLength])
	}

	input = strings.ReplaceAll(input, "\x00", "")
	input = strings.ReplaceAll(input, "$", "")
	input = strings.ReplaceAll(input, "{", "\\{")
	input = strings.ReplaceAll(input, "}", "\\}")

	return input
}

func (s *OutputSanitizer) SanitizeFieldName(input string) string {
	if utf8.RuneCountInString(input) > s.maxFieldLength {
		input = string([]rune(input)[:s.maxFieldLength])
	}

	re := regexp.MustCompile(`[^a-zA-Z0-9_]`)
	input = re.ReplaceAllString(input, "_")

	if len(input) > 0 && input[0] >= '0' && input[0] <= '9' {
		input = "_" + input
	}

	return input
}

func (s *OutputSanitizer) SanitizeForLog(input string) string {
	input = strings.ReplaceAll(input, "\n", "\\n")
	input = strings.ReplaceAll(input, "\r", "\\r")
	input = strings.ReplaceAll(input, "\t", "\\t")

	if len(input) > 500 {
		input = input[:500] + "...[truncated]"
	}

	return input
}

func (s *OutputSanitizer) SanitizeJSONInput(input string) string {
	input = strings.TrimSpace(input)

	if strings.Contains(input, "```") {
		input = stripCodeFence(input)
	}

	return input
}

func (s *OutputSanitizer) SanitizeMap(data map[string]any, depth int) map[string]any {
	if depth > s.maxDepth {
		return map[string]any{"__error": "max depth exceeded"}
	}

	result := make(map[string]any)
	for k, v := range data {
		safeKey := s.SanitizeFieldName(k)
		result[safeKey] = s.sanitizeValue(v, depth+1)
	}
	return result
}

func (s *OutputSanitizer) sanitizeValue(val any, depth int) any {
	if depth > s.maxDepth {
		return "__max_depth__"
	}

	switch v := val.(type) {
	case string:
		return s.SanitizeString(v)
	case map[string]any:
		return s.SanitizeMap(v, depth)
	case []any:
		result := make([]any, len(v))
		for i, item := range v {
			result[i] = s.sanitizeValue(item, depth+1)
		}
		return result
	default:
		return val
	}
}

func stripCodeFence(content string) string {
	if len(content) > 6 && content[:3] == "```" {
		endIdx := len(content) - 3
		for i := len(content) - 3; i >= 3; i-- {
			if content[i:i+3] == "```" {
				endIdx = i
				break
			}
		}
		content = content[3:endIdx]
		if len(content) > 0 && content[0] == '\n' {
			content = content[1:]
		}
		if len(content) > 4 {
			lower := strings.ToLower(content[:5])
			if lower == "json\n" || lower == "json" {
				for i := 0; i < len(content); i++ {
					if content[i] == '\n' {
						content = content[i+1:]
						break
					}
				}
			}
		}
	}
	return content
}

type MemoryImportValidator struct {
	maxContentLength  int
	forbiddenPatterns []string
}

func NewMemoryImportValidator() *MemoryImportValidator {
	return &MemoryImportValidator{
		maxContentLength: 50000,
		forbiddenPatterns: []string{
			`(?i)system\s*instruction`,
			`(?i)ignore\s*previous`,
			`(?i)you\s*are\s*now`,
			`(?i)new\s*persona`,
			`(?i)override\s*system`,
		},
	}
}

func (v *MemoryImportValidator) ValidateMemoryContent(content string) error {
	if len(content) > v.maxContentLength {
		return fmt.Errorf("content exceeds maximum length of %d", v.maxContentLength)
	}

	for _, pattern := range v.forbiddenPatterns {
		matched, err := regexp.MatchString(pattern, content)
		if err != nil {
			continue
		}
		if matched {
			return fmt.Errorf("content contains forbidden pattern: prompt injection attempt detected")
		}
	}

	return nil
}

func ValidateConfidence(value float64) error {
	if value < 0.0 || value > 1.0 {
		return fmt.Errorf("confidence must be between 0.0 and 1.0, got %f", value)
	}
	return nil
}

func ValidateImportance(value float64) error {
	if value < 0.0 || value > 1.0 {
		return fmt.Errorf("importance must be between 0.0 and 1.0, got %f", value)
	}
	return nil
}

func ValidateID(id string) error {
	if id == "" {
		return fmt.Errorf("ID cannot be empty")
	}
	if len(id) > 256 {
		return fmt.Errorf("ID exceeds maximum length of 256")
	}
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_\-.:]+$`, id)
	if !matched {
		return fmt.Errorf("ID contains invalid characters")
	}
	return nil
}
