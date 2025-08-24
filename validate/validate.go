package validate

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/goflash/flash/v2/ctx"
)

// Validator is the global validator instance for goflash validation helpers.
// You can register custom tags and tag name functions on it.
var Validator = validator.New()

// messageFunc, if set by the application, converts a FieldError to a human message.
// This allows applications to plug in locale-specific translations (e.g.,
// using universal-translator and validator default translations) without the
// framework importing any locale/translation packages.
var messageFunc func(validator.FieldError) string

// SetMessageFunc sets a custom function used to render human-readable messages
// for validator.FieldError values. If not set, a small built-in fallback is used.
//
// Example (app-level):
//
//	// create translator and register default translations on validate.Validator
//	// then:
//	validate.SetMessageFunc(func(fe validator.FieldError) string { return fe.Translate(trans) })
func SetMessageFunc(fn func(validator.FieldError) string) { messageFunc = fn }

// Context key for storing a per-request message function.
type ctxKeyMsgFunc struct{}

// WithMessageFunc attaches a message function to a non-nil context and returns the derived context.
// Context must be non-nil (use c.Context() from goflash Ctx or context.Background()).
func WithMessageFunc(ctx context.Context, fn func(validator.FieldError) string) context.Context {
	return context.WithValue(ctx, ctxKeyMsgFunc{}, fn)
}

// MessageFuncFromContext retrieves a message function from context if present.
func MessageFuncFromContext(ctx context.Context) func(validator.FieldError) string {
	if ctx == nil {
		return nil
	}
	if v, ok := ctx.Value(ctxKeyMsgFunc{}).(func(validator.FieldError) string); ok {
		return v
	}
	return nil
}

func init() {
	// Use `json` tag names in error messages instead of struct field names.
	Validator.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := fld.Tag.Get("json")
		if name == "" || name == "-" {
			return ""
		}
		if idx := strings.Index(name, ","); idx >= 0 {
			name = name[:idx]
		}
		return name
	})
}

// Struct validates a struct using `validate` tags and the global Validator.
// Returns a ValidationErrors error if validation fails.
func Struct(s any) error { return Validator.Struct(s) }

// FieldErrors is an error type that carries a map of field->message.
// Useful for mapping JSON binding or custom validation errors to field errors.
type FieldErrors map[string]string

func (e FieldErrors) Error() string { return "field validation errors" }

// ToFieldErrors converts various error types into a simple field->message map.
// Supports:
// - flash ctx.FieldErrors (BindJSON errors for unknown fields/type mismatches)
// - go-playground validator.ValidationErrors
// - validate.FieldErrors (this package)
// Falls back to {"_error": err.Error()} otherwise.
func ToFieldErrors(err error) map[string]string { return ToFieldErrorsWith(err, messageFunc) }

// ToFieldErrorsWith is like ToFieldErrors but allows providing a custom message function
// for this call (e.g., a request-scoped translator). If fn is nil, the global SetMessageFunc
// (if any) and then the built-in fallback will be used.
func ToFieldErrorsWith(err error, fn func(validator.FieldError) string) map[string]string {
	res := map[string]string{}
	if err == nil {
		return res
	}

	switch err.(type) {
	case ctx.FieldErrors:
		_ = handleCtxFieldErrors(err, res)
		return res
	case validator.ValidationErrors:
		_ = handleValidationErrors(err, res, fn)
		return res
	case FieldErrors:
		_ = handleDirectFieldErrors(err, res)
		return res
	}
	if handled := handleStructuredErrorMessage(err, res); handled {
		return res
	}
	// Final fallback
	res["_error"] = err.Error()
	return res
}

// handleCtxFieldErrors maps ctx.FieldErrors into res and merges extras from the aggregated message.
func handleCtxFieldErrors(err error, res map[string]string) bool {
	fe, ok := err.(ctx.FieldErrors)
	if !ok {
		return false
	}
	for _, e := range fe.All() {
		f := normalizeFieldKey(e.Field())
		if f == "" {
			continue
		}
		res[f] = e.Message()
	}
	if extras := parseStructuredErrors(err.Error()); len(extras) > 0 {
		for k, v := range extras {
			if _, exists := res[k]; !exists {
				res[k] = v
			}
		}
	}
	return true
}

// handleValidationErrors maps go-playground validator.ValidationErrors into res.
func handleValidationErrors(err error, res map[string]string, fn func(validator.FieldError) string) bool {
	vErrs, ok := err.(validator.ValidationErrors)
	if !ok {
		return false
	}
	for _, fe := range vErrs {
		field := fe.Field()
		if field == "" {
			field = fe.StructField()
		}
		res[field] = humanMessageWith(fe, fn)
	}
	return true
}

// handleDirectFieldErrors copies this package's FieldErrors into res.
func handleDirectFieldErrors(err error, res map[string]string) bool {
	fe, ok := err.(FieldErrors)
	if !ok {
		return false
	}
	for k, v := range fe {
		res[k] = v
	}
	return true
}

// handleStructuredErrorMessage parses mapstructure/BindJSON-style errors and populates res.
func handleStructuredErrorMessage(err error, res map[string]string) bool {
	fieldErrors := parseStructuredErrors(err.Error())
	if len(fieldErrors) == 0 {
		return false
	}
	for k, v := range fieldErrors {
		res[k] = v
	}
	return true
}

// ToFieldErrorsWithContext uses a request-scoped message function from context
// (if set via WithMessageFunc). Falls back to global SetMessageFunc and then
// built-in defaults.
func ToFieldErrorsWithContext(ctx context.Context, err error) map[string]string {
	return ToFieldErrorsWith(err, MessageFuncFromContext(ctx))
}

// humanMessage returns a message for a FieldError using the global messageFunc if set.
func humanMessage(fe validator.FieldError) string { return humanMessageWith(fe, nil) }

// humanMessageWith returns a message for a FieldError using the provided fn if not nil,
// otherwise the global messageFunc, otherwise a default fallback.
func humanMessageWith(fe validator.FieldError, fn func(validator.FieldError) string) string {
	if fn != nil {
		if msg := fn(fe); msg != "" {
			return msg
		}
	}
	if messageFunc != nil {
		if msg := messageFunc(fe); msg != "" {
			return msg
		}
	}
	return defaultMessage(fe)
}

// normalizeFieldKey cleans a field key coming from ctx.FieldErrors.
// It returns empty string for aggregated/complex error messages that contain
// newlines or complex formatting, keeping only simple field names.
func normalizeFieldKey(s string) string {
	if s == "" {
		return ""
	}

	// If the original field key contains newlines, it's likely an aggregated
	// error message that should be filtered out entirely
	if strings.ContainsAny(s, "\r\n") {
		return ""
	}

	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	// Only allow simple tokens: letters, numbers, dot, dash, underscore
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if !(ch == '.' || ch == '-' || ch == '_' || (ch >= '0' && ch <= '9') || (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z')) {
			return ""
		}
	}
	return s
}

// parseStructuredErrors attempts to parse structured error messages from
// mapstructure/BindJSON errors and extract field-specific errors.
// It handles patterns like:
// "1 error(s) decoding:\n\n* 'field' expected type 'string', got unconvertible type 'float64', value: '1'"
func parseStructuredErrors(errMsg string) map[string]string {
	result := map[string]string{}

	// Look for mapstructure-style errors
	lines := strings.Split(errMsg, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "*") {
			continue
		}

		// Parse patterns like: "* 'field' expected type 'string', got unconvertible type 'float64', value: '1'"
		if matches := parseFieldTypeError(line); matches != nil {
			fieldName := matches["field"]
			expectedType := matches["expected"]
			gotType := matches["got"]

			if fieldName != "" {
				// Create a human-friendly error message
				var msg string
				if expectedType != "" && gotType != "" {
					msg = fmt.Sprintf("expected %s but got %s", expectedType, gotType)
				} else {
					msg = "invalid type"
				}
				result[fieldName] = msg
			}
			// continue; a single line can contain only one type error pattern
			continue
		}

		// Parse patterns like: "* '' has invalid keys: foo, bar" or "* 'root' has invalid keys: foo"
		if invalids := parseInvalidKeys(line); len(invalids) > 0 {
			for _, key := range invalids {
				// Don't overwrite any more specific message already parsed
				if _, exists := result[key]; !exists {
					result[key] = "unexpected"
				}
			}
			continue
		}
	}

	return result
}

// parseFieldTypeError extracts field name and type information from mapstructure error lines
func parseFieldTypeError(line string) map[string]string {
	// Pattern: "* 'field' expected type 'string', got unconvertible type 'float64', value: '1'"
	// Also handle: "* 'field' expected type 'string', got unconvertible type 'float64'"

	result := map[string]string{}

	// Extract field name between first pair of single quotes
	if start := strings.Index(line, "'"); start != -1 {
		if end := strings.Index(line[start+1:], "'"); end != -1 {
			fieldName := line[start+1 : start+1+end]
			result["field"] = fieldName
		}
	}

	// Extract expected type
	if idx := strings.Index(line, "expected type '"); idx != -1 {
		start := idx + len("expected type '")
		if end := strings.Index(line[start:], "'"); end != -1 {
			expectedType := line[start : start+end]
			result["expected"] = expectedType
		}
	}

	// Extract actual type
	if idx := strings.Index(line, "got unconvertible type '"); idx != -1 {
		start := idx + len("got unconvertible type '")
		if end := strings.Index(line[start:], "'"); end != -1 {
			gotType := line[start : start+end]
			result["got"] = gotType
		}
	} else if idx := strings.Index(line, "got "); idx != -1 {
		// Handle simpler "got type" patterns
		start := idx + len("got ")
		parts := strings.Fields(line[start:])
		if len(parts) > 0 {
			gotType := strings.Trim(parts[0], "',")
			result["got"] = gotType
		}
	}

	// Only return if we found at least a field name
	if result["field"] != "" {
		return result
	}
	return nil
}

// parseInvalidKeys extracts a list of unexpected field keys from a mapstructure
// error line such as: "* ” has invalid keys: foo, bar".
func parseInvalidKeys(line string) []string {
	// Find the colon after the phrase and take the remainder
	idx := strings.Index(line, "has invalid keys:")
	if idx == -1 {
		return nil
	}
	rest := line[idx+len("has invalid keys:"):]
	// Split by commas, trim spaces and quotes
	parts := strings.Split(rest, ",")
	keys := make([]string, 0, len(parts))
	for _, p := range parts {
		k := strings.TrimSpace(p)
		k = strings.Trim(k, "'\"")
		// Remove trailing punctuation like ")" or "."
		k = strings.TrimRight(k, ") .")
		if k != "" {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return nil
	}
	return keys
}

// defaultMessage provides a minimal, dependency-free fallback for common tags.
func defaultMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "min":
		return fmt.Sprintf("must be at least %s", fe.Param())
	case "max":
		return fmt.Sprintf("must be at most %s", fe.Param())
	case "len":
		return fmt.Sprintf("must be length %s", fe.Param())
	case "email":
		return "must be a valid email"
	case "oneof":
		return fmt.Sprintf("must be one of %s", fe.Param())
	case "gte":
		return fmt.Sprintf("must be greater than or equal to %s", fe.Param())
	case "lte":
		return fmt.Sprintf("must be less than or equal to %s", fe.Param())
	case "url":
		return "must be a valid URL"
	case "uuid":
		return "must be a valid UUID"
	case "alpha":
		return "must contain only letters"
	case "alphanum":
		return "must contain only letters and numbers"
	case "numeric":
		return "must contain only numbers"
	case "contains":
		return fmt.Sprintf("must contain %s", fe.Param())
	case "excludes":
		return fmt.Sprintf("must not contain %s", fe.Param())
	case "startswith":
		return fmt.Sprintf("must start with %s", fe.Param())
	case "endswith":
		return fmt.Sprintf("must end with %s", fe.Param())
	case "base64":
		return "must be a valid base64 string"
	case "json":
		return "must be valid JSON"
	case "ip":
		return "must be a valid IP address"
	case "cidr":
		return "must be a valid CIDR notation"
	case "ascii":
		return "must contain only ASCII characters"
	case "printascii":
		return "must contain only printable ASCII characters"
	case "multibyte":
		return "must contain multibyte characters"
	case "iscolor":
		return "must be a valid color"
	case "isbn":
		return "must be a valid ISBN"
	case "isbn10":
		return "must be a valid ISBN-10"
	case "isbn13":
		return "must be a valid ISBN-13"
	default:
		return fmt.Sprintf("failed %s", fe.Tag())
	}
}
