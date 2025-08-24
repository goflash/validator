package validate

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	ut "github.com/go-playground/universal-translator"
	globalValidator "github.com/go-playground/validator/v10"
	"github.com/goflash/flash/v2/ctx"
	"github.com/stretchr/testify/assert"
)

type user struct {
	Name string `json:"name" validate:"required,min=2"`
	Age  int    `json:"age" validate:"min=0,max=130"`
}

func TestValidateStructAndToFieldErrors(t *testing.T) {
	u := user{Name: "a", Age: -1}
	if err := Struct(u); err == nil {
		t.Fatalf("expected error")
	}
	// --- Coverage for ctx.FieldErrors mapping branch ---

	u = user{Name: "ab", Age: 10}
	if err := Struct(u); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestToFieldErrors(t *testing.T) {
	u := user{Name: "a", Age: -1}
	err := Struct(u)
	m := ToFieldErrors(err)
	if len(m) == 0 {
		t.Fatalf("expected field errors")
	}
	if _, ok := m["name"]; !ok {
		t.Fatalf("expected name error mapped")
	}
}

func TestToFieldErrorsWithNonValidationError(t *testing.T) {
	m := ToFieldErrors(assert.AnError)
	if _, ok := m["_error"]; !ok {
		t.Fatalf("expected _error entry")
	}
}

func TestFieldErrorsError(t *testing.T) {
	var err error = FieldErrors{"name": "is required"}
	if err.Error() != "field validation errors" {
		t.Fatalf("unexpected error string: %q", err.Error())
	}
}

func TestToFieldErrorsNilReturnsEmpty(t *testing.T) {
	m := ToFieldErrors(nil)
	if len(m) != 0 {
		t.Fatalf("expected empty map for nil error, got %v", m)
	}
}

func TestToFieldErrorsFieldErrorsPassthrough(t *testing.T) {
	in := FieldErrors{"foo": "bar"}
	m := ToFieldErrors(in)
	if len(m) != 1 || m["foo"] != "bar" {
		t.Fatalf("expected passthrough FieldErrors, got %v", m)
	}
}

func TestToFieldErrorsFallbackToStructField(t *testing.T) {
	type S struct {
		Secret string `json:"-" validate:"required"`
	}
	err := Struct(S{}) // missing Secret triggers validation error
	m := ToFieldErrors(err)
	// When tag name func returns empty for json:"-", we should fallback to StructField()
	if _, ok := m["Secret"]; !ok {
		t.Fatalf("expected fallback to StructField key 'Secret', got %v", m)
	}
}

func TestSetMessageFuncOverridesDefault(t *testing.T) {
	// Ensure we reset global after test
	defer SetMessageFunc(nil)
	SetMessageFunc(func(fe globalValidator.FieldError) string { return "OVERRIDDEN" })
	u := user{Name: "a", Age: -1}
	err := Struct(u)
	m := ToFieldErrors(err)
	if m["name"] != "OVERRIDDEN" {
		t.Fatalf("expected overridden message, got %v", m["name"])
	}
}

func TestMessageFuncFromContext_Nil(t *testing.T) {
	if MessageFuncFromContext(context.TODO()) != nil {
		t.Fatalf("expected nil when ctx is nil")
	}
}

func TestMessageFuncFromContext_NoValue(t *testing.T) {
	if MessageFuncFromContext(context.Background()) != nil {
		t.Fatalf("expected nil when no message func in context")
	}
}

func TestMessageFuncFromContext_WithValue(t *testing.T) {
	ctx := WithMessageFunc(context.Background(), func(fe globalValidator.FieldError) string { return "CTX" })
	fn := MessageFuncFromContext(ctx)
	if fn == nil {
		t.Fatalf("expected non-nil message func from context")
	}
	// Ensure ToFieldErrorsWithContext uses the context function
	u := user{} // Name required -> triggers error
	err := Struct(u)
	m := ToFieldErrorsWithContext(ctx, err)
	assert.Equal(t, "CTX", m["name"])
}

func TestDefaultMessage_ManyTags(t *testing.T) {
	// Ensure default messages are used
	defer SetMessageFunc(nil)

	type DM struct {
		Req        string `json:"req" validate:"required"`
		Min        string `json:"min" validate:"min=3"`
		Max        string `json:"max" validate:"max=5"`
		Len        string `json:"len" validate:"len=2"`
		Email      string `json:"email" validate:"email"`
		One        string `json:"one" validate:"oneof=a b c"`
		GTE        int    `json:"gte" validate:"gte=5"`
		LTE        int    `json:"lte" validate:"lte=3"`
		URL        string `json:"url" validate:"url"`
		UUID       string `json:"uuid" validate:"uuid"`
		Alpha      string `json:"alpha" validate:"alpha"`
		Alphanum   string `json:"alphanum" validate:"alphanum"`
		Numeric    string `json:"numeric" validate:"numeric"`
		Contains   string `json:"contains" validate:"contains=x"`
		Excludes   string `json:"excludes" validate:"excludes=y"`
		Base64     string `json:"base64" validate:"base64"`
		JSON       string `json:"json" validate:"json"`
		IP         string `json:"ip" validate:"ip"`
		CIDR       string `json:"cidr" validate:"cidr"`
		ASCII      string `json:"ascii" validate:"ascii"`
		PrintASCII string `json:"printascii" validate:"printascii"`
		Multibyte  string `json:"multibyte" validate:"multibyte"`
		IsColor    string `json:"iscolor" validate:"iscolor"`
		ISBN       string `json:"isbn" validate:"isbn"`
		ISBN10     string `json:"isbn10" validate:"isbn10"`
		ISBN13     string `json:"isbn13" validate:"isbn13"`
	}

	bad := DM{
		Req:        "",
		Min:        "ab",
		Max:        "abcdef",
		Len:        "a",
		Email:      "not-an-email",
		One:        "z",
		GTE:        3,
		LTE:        5,
		URL:        "not-a-url",
		UUID:       "x",
		Alpha:      "abc123",
		Alphanum:   "ab!",
		Numeric:    "abc",
		Contains:   "abc",
		Excludes:   "has y inside",
		Base64:     "****",
		JSON:       "{",
		IP:         "999.0.0.1",
		CIDR:       "999.0.0.1/33",
		ASCII:      "✓",
		PrintASCII: "a" + string(rune(7)),
		Multibyte:  "abc",
		IsColor:    "not-a-color",
		ISBN:       "foo",
		ISBN10:     "123",
		ISBN13:     "123",
	}

	err := Struct(bad)
	if err == nil {
		t.Fatalf("expected validation errors")
	}
	m := ToFieldErrors(err)
	assert.Equal(t, "is required", m["req"])
	assert.Equal(t, "must be at least 3", m["min"])
	assert.Equal(t, "must be at most 5", m["max"])
	assert.Equal(t, "must be length 2", m["len"])
	assert.Equal(t, "must be a valid email", m["email"])
	assert.Equal(t, "must be one of a b c", m["one"])
	assert.Equal(t, "must be greater than or equal to 5", m["gte"])
	assert.Equal(t, "must be less than or equal to 3", m["lte"])
	assert.Equal(t, "must be a valid URL", m["url"])
	assert.Equal(t, "must be a valid UUID", m["uuid"])
	assert.Equal(t, "must contain only letters", m["alpha"])
	assert.Equal(t, "must contain only letters and numbers", m["alphanum"])
	assert.Equal(t, "must contain only numbers", m["numeric"])
	assert.Equal(t, "must contain x", m["contains"])
	assert.Equal(t, "must not contain y", m["excludes"])
	assert.Equal(t, "must be a valid base64 string", m["base64"])
	assert.Equal(t, "must be valid JSON", m["json"])
	assert.Equal(t, "must be a valid IP address", m["ip"])
	assert.Equal(t, "must be a valid CIDR notation", m["cidr"])
	assert.Equal(t, "must contain only ASCII characters", m["ascii"])
	assert.Equal(t, "must contain only printable ASCII characters", m["printascii"])
	assert.Equal(t, "must contain multibyte characters", m["multibyte"])
	assert.Equal(t, "must be a valid color", m["iscolor"])
	assert.Equal(t, "must be a valid ISBN", m["isbn"])
	assert.Equal(t, "must be a valid ISBN-10", m["isbn10"])
	assert.Equal(t, "must be a valid ISBN-13", m["isbn13"])
}

func TestDefaultMessage_StartsWithEndsWith(t *testing.T) {
	// Ensure default messages for startswith/endswith are covered
	defer SetMessageFunc(nil)

	type SE struct {
		S string `json:"s" validate:"startswith=ab"`
		E string `json:"e" validate:"endswith=yz"`
	}
	bad := SE{S: "cd", E: "xy"}
	err := Struct(bad)
	if err == nil {
		t.Fatalf("expected validation errors")
	}
	m := ToFieldErrors(err)
	assert.Equal(t, "must start with ab", m["s"])
	assert.Equal(t, "must end with yz", m["e"])
}

type allTags struct {
	Req   string `json:"req" validate:"required"`
	Min   string `json:"min" validate:"min=2"`
	Max   string `json:"max" validate:"max=3"`
	Len   string `json:"len" validate:"len=2"`
	Email string `json:"email" validate:"email"`
	OneOf string `json:"one" validate:"oneof=a b"`
}

type dashTag struct {
	Field string `json:"-" validate:"required"`
}

type customTag struct {
	F string `json:"f" validate:"iscolor"`
}

func TestHumanMessageBranchesViaValidation(t *testing.T) {
	v := globalValidator.New()
	// use json tag names
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := fld.Tag.Get("json")
		if idx := strings.Index(name, ","); idx >= 0 {
			name = name[:idx]
		}
		return name
	})
	a := allTags{Req: "", Min: "a", Max: "abcd", Len: "a", Email: "x", OneOf: "z"}
	err := v.Struct(a)
	ves, ok := err.(globalValidator.ValidationErrors)
	if !ok {
		t.Fatalf("expected validation errors")
	}
	// Map with our human messages
	for _, fe := range ves {
		_ = humanMessage(fe)
	}
	m := ToFieldErrors(err)
	if len(m) == 0 {
		t.Fatalf("expected mapped errors")
	}
}

func TestToFieldErrorsNonValidation(t *testing.T) {
	m := ToFieldErrors(errors.New("boom"))
	if m["_error"] == "" {
		t.Fatalf("expected _error mapping")
	}
}

func TestValidatorInitTagNameDash(t *testing.T) {
	// ensure package-level Validator tag name func removes '-' fields
	d := dashTag{}
	err := Struct(d) // uses package Validator from init()
	if err == nil {
		t.Fatalf("expected error")
	}
	m := ToFieldErrors(err)
	if got, ok := m["Field"]; !ok || got != "is required" {
		t.Fatalf("expected fallback key 'Field' with message 'is required', got: %#v", m)
	}
}

func TestHumanMessageDefaultBranch(t *testing.T) {
	v := globalValidator.New()
	v.RegisterValidation("iscolor", func(fl globalValidator.FieldLevel) bool { return false })
	v.RegisterTagNameFunc(func(fld reflect.StructField) string { return fld.Tag.Get("json") })
	u := customTag{F: "xxx"}
	err := v.Struct(u)
	ves := err.(globalValidator.ValidationErrors)
	got := humanMessage(ves[0])
	if got == "" || got == "is required" {
		t.Fatalf("expected default message, got %q", got)
	}
}

func TestDefaultMessage_UnknownTag_DefaultFallback(t *testing.T) {
	// Ensure default branch (unknown tag) returns "failed <tag>"
	defer SetMessageFunc(nil)
	v := globalValidator.New()
	v.RegisterValidation("madeup", func(fl globalValidator.FieldLevel) bool { return false })
	v.RegisterTagNameFunc(func(fld reflect.StructField) string { return fld.Tag.Get("json") })
	type X struct {
		A string `json:"a" validate:"madeup"`
	}
	err := v.Struct(X{A: "value"})
	if err == nil {
		t.Fatalf("expected validation error")
	}
	ves := err.(globalValidator.ValidationErrors)
	got := humanMessage(ves[0])
	if got != "failed madeup" {
		t.Fatalf("expected 'failed madeup', got %q", got)
	}
}

type noJSONTag struct {
	Name string `validate:"required"`
}

func TestToFieldErrorsFallsBackToStructField(t *testing.T) {
	u := noJSONTag{}
	err := Struct(u)
	m := ToFieldErrors(err)
	if _, ok := m["Name"]; !ok {
		t.Fatalf("expected StructField fallback key, got: %#v", m)
	}
}

func TestToFieldErrorsNil(t *testing.T) {
	m := ToFieldErrors(nil)
	if len(m) != 0 {
		t.Fatalf("expected empty map for nil error")
	}
}

func TestToFieldErrorsNonValidationDup(t *testing.T) {
	m := ToFieldErrors(errors.New("x"))
	if m["_error"] == "" {
		t.Fatalf("expected _error")
	}
}

type commaJSON struct {
	Field string `json:"pretty_name,omitempty" validate:"required"`
}

func TestValidatorInitTagNameCommaStripsOptions(t *testing.T) {
	// Uses package-level Validator via Struct()
	var v commaJSON
	err := Struct(v)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	m := ToFieldErrors(err)
	if _, ok := m["pretty_name"]; !ok {
		t.Fatalf("expected key 'pretty_name', got: %#v", m)
	}
}

func TestToFieldErrorsDashJSONFallbackAndMessages(t *testing.T) {
	obj := struct {
		Hidden string `json:"-" validate:"required"`
	}{}
	err := Struct(obj)
	m := ToFieldErrors(err)
	if got, ok := m["Hidden"]; !ok || got != "is required" {
		t.Fatalf("expected fallback 'Hidden' -> 'is required', got: %#v", m)
	}
}

// --- Coverage for ctx.FieldErrors mapping branch in ToFieldErrorsWith ---

type fakeCtxFieldError struct{ f, m string }

func (e fakeCtxFieldError) Field() string   { return e.f }
func (e fakeCtxFieldError) Message() string { return e.m }

type fakeCtxFieldErrors struct{ list []ctx.FieldError }

func (f fakeCtxFieldErrors) Error() string         { return "field validation errors" }
func (f fakeCtxFieldErrors) All() []ctx.FieldError { return f.list }

// Variant that carries the full aggregated error message to test merging
type fakeCtxFieldErrorsWithMsg struct {
	list []ctx.FieldError
	msg  string
}

func (f fakeCtxFieldErrorsWithMsg) Error() string         { return f.msg }
func (f fakeCtxFieldErrorsWithMsg) All() []ctx.FieldError { return f.list }

func TestToFieldErrorsWith_CtxFieldErrors(t *testing.T) {
	fe := fakeCtxFieldErrors{list: []ctx.FieldError{
		fakeCtxFieldError{f: "name", m: "unexpected"},
		fakeCtxFieldError{f: "age", m: "invalid type"},
		// A noisy/aggregated key should be ignored by normalizeFieldKey
		fakeCtxFieldError{f: "foo\n* 'bar' expected type 'string'", m: "unexpected"},
	}}
	m := ToFieldErrorsWith(fe, nil)
	if len(m) != 2 || m["name"] != "unexpected" || m["age"] != "invalid type" {
		t.Fatalf("unexpected map: %#v", m)
	}
}

func TestToFieldErrorsWith_CtxFieldErrors_EmptyList(t *testing.T) {
	fe := fakeCtxFieldErrors{list: nil}
	m := ToFieldErrorsWith(fe, nil)
	if len(m) != 0 {
		t.Fatalf("expected empty map for empty ctx.FieldErrors, got %#v", m)
	}
}

func TestToFieldErrorsWith_CtxFieldErrors_MergesStructuredErrorString(t *testing.T) {
	fe := fakeCtxFieldErrorsWithMsg{
		list: []ctx.FieldError{
			fakeCtxFieldError{f: "extra", m: "unexpected"},
		},
		msg: "2 error(s) decoding:\n\n* 'name' expected type 'string', got unconvertible type 'float64', value: '1'\n* '' has invalid keys: extra, foo",
	}
	m := ToFieldErrorsWith(fe, nil)
	// Original from All()
	assert.Equal(t, "unexpected", m["extra"]) // preserved
	// Parsed from aggregated string
	assert.Equal(t, "expected string but got float64", m["name"]) // type error
	assert.Equal(t, "unexpected", m["foo"])                       // additional invalid key
}

func TestToFieldErrorsWith_ValidationErrors_Normal(t *testing.T) {
	// Build a validation error via package Validator
	type X struct {
		A string `json:"a" validate:"required"`
		B int    `json:"b" validate:"min=2"`
	}
	err := Struct(X{A: "", B: 1})
	if err == nil {
		t.Fatalf("expected validation error")
	}
	m := ToFieldErrorsWith(err, nil)
	if m["a"] == "" || m["b"] == "" {
		t.Fatalf("expected mapped messages for a and b, got %#v", m)
	}
}

type ctxUser struct {
	Name string `json:"name" validate:"required,min=2"`
}

// Test error types for structured error parsing tests
type simpleError struct{ msg string }

func (e simpleError) Error() string { return e.msg }

type simpleError2 struct{ msg string }

func (e simpleError2) Error() string { return e.msg }

func TestToFieldErrorsWithContext_UsesRequestScopedFunc(t *testing.T) {
	defer SetMessageFunc(nil) // ensure global is cleared
	// Build a validation error
	var u ctxUser
	err := Struct(u)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	// Provide request-scoped translator via context
	ctx := WithMessageFunc(context.Background(), func(fe globalValidator.FieldError) string { return "CTXMSG" })
	m := ToFieldErrorsWithContext(ctx, err)
	if got := m["name"]; got != "CTXMSG" {
		t.Fatalf("expected CTXMSG, got %q (map=%v)", got, m)
	}
}

func TestToFieldErrorsWithContext_FallsBackToGlobal(t *testing.T) {
	defer SetMessageFunc(nil)
	SetMessageFunc(func(fe globalValidator.FieldError) string { return "GLOBALMSG" })
	var u ctxUser
	err := Struct(u)
	m := ToFieldErrorsWithContext(context.Background(), err)
	if got := m["name"]; got != "GLOBALMSG" {
		t.Fatalf("expected GLOBALMSG, got %q (map=%v)", got, m)
	}
}

func TestMessageFuncFromContext_Present(t *testing.T) {
	mfExp := func(fe globalValidator.FieldError) string { return "X" }
	ctx := WithMessageFunc(context.TODO(), mfExp)
	mf := MessageFuncFromContext(ctx)
	if mf == nil || mf(nil) != "X" {
		t.Fatalf("expected message function from context to work")
	}
}

// Note: We avoid passing a literal nil context to satisfy staticcheck SA1012.
func TestMessageFuncFromContext_NilCtx_Reflect(t *testing.T) {
	// Call MessageFuncFromContext with a nil context via reflection to cover the nil branch
	fnVal := reflect.ValueOf(MessageFuncFromContext)
	ctxIface := reflect.TypeOf((*context.Context)(nil)).Elem()
	out := fnVal.Call([]reflect.Value{reflect.Zero(ctxIface)})
	if len(out) != 1 || !out[0].IsNil() {
		t.Fatalf("expected nil function for nil context, got %#v", out)
	}
}

func TestWithMessageFunc_NonNilContext(t *testing.T) {
	ctx := WithMessageFunc(context.Background(), func(fe globalValidator.FieldError) string { return "ok" })
	if mf := MessageFuncFromContext(ctx); mf == nil || mf(nil) != "ok" {
		t.Fatalf("expected stored message function to be retrievable")
	}
}

func TestToFieldErrorsWithContext_EmptyFn_FallsBackToGlobal(t *testing.T) {
	defer SetMessageFunc(nil)
	SetMessageFunc(func(fe globalValidator.FieldError) string { return "GLOBAL_FALLBACK" })
	// Build a validation error
	type X struct {
		A string `json:"a" validate:"required"`
	}
	err := Struct(X{})
	if err == nil {
		t.Fatalf("expected validation error")
	}
	// Provide request-scoped fn that returns empty string -> should fallback to global
	ctx := WithMessageFunc(context.Background(), func(fe globalValidator.FieldError) string { return "" })
	m := ToFieldErrorsWithContext(ctx, err)
	if got := m["a"]; got != "GLOBAL_FALLBACK" {
		t.Fatalf("expected GLOBAL_FALLBACK, got %q (map=%v)", got, m)
	}
}

func TestToFieldErrorsWithContext_EmptyFn_DefaultFallback(t *testing.T) {
	defer SetMessageFunc(nil)
	// Ensure no global message func
	SetMessageFunc(nil)
	// Build a validation error
	type X struct {
		A string `json:"a" validate:"required"`
	}
	err := Struct(X{})
	if err == nil {
		t.Fatalf("expected validation error")
	}
	// Provide request-scoped fn that returns empty string -> should fallback to defaultMessage
	ctx := WithMessageFunc(context.Background(), func(fe globalValidator.FieldError) string { return "" })
	m := ToFieldErrorsWithContext(ctx, err)
	if got := m["a"]; got != "is required" {
		t.Fatalf("expected default 'is required', got %q (map=%v)", got, m)
	}
}

func TestGlobalMessageFunc_EmptyReturns_DefaultFallback(t *testing.T) {
	// If a global message func is set but returns empty, we should fallback to default messages
	defer SetMessageFunc(nil)
	SetMessageFunc(func(fe globalValidator.FieldError) string { return "" })

	type X struct {
		A string `json:"a" validate:"required"`
	}
	err := Struct(X{})
	if err == nil {
		t.Fatalf("expected validation error")
	}
	m := ToFieldErrors(err)
	if got := m["a"]; got != "is required" {
		t.Fatalf("expected default 'is required', got %q (map=%v)", got, m)
	}
}

func TestToFieldErrorsWith_Direct_NonValidationError(t *testing.T) {
	m := ToFieldErrorsWith(assert.AnError, nil)
	if got := m["_error"]; got != assert.AnError.Error() {
		t.Fatalf("expected _error mapping, got %q", got)
	}
}

func TestToFieldErrorsWith_Direct_FieldErrorsPassthrough(t *testing.T) {
	in := FieldErrors{"a": "b"}
	m := ToFieldErrorsWith(in, nil)
	if len(m) != 1 || m["a"] != "b" {
		t.Fatalf("expected passthrough, got %#v", m)
	}
}

func TestToFieldErrorsWith_Direct_NilError(t *testing.T) {
	m := ToFieldErrorsWith(nil, nil)
	if len(m) != 0 {
		t.Fatalf("expected empty map for nil error, got %#v", m)
	}
}

func TestToFieldErrorsWith_CustomFnWins(t *testing.T) {
	// Build a validation error
	type X struct {
		A string `json:"a" validate:"required"`
	}
	err := Struct(X{})
	m := ToFieldErrorsWith(err, func(fe globalValidator.FieldError) string { return "CUSTOM" })
	if got := m["a"]; got != "CUSTOM" {
		t.Fatalf("expected CUSTOM, got %q (map=%v)", got, m)
	}
}

func TestToFieldErrorsWith_DashTag_FallbackKey(t *testing.T) {
	type D struct {
		Hidden string `json:"-" validate:"required"`
	}
	err := Struct(D{})
	m := ToFieldErrorsWith(err, nil)
	if got, ok := m["Hidden"]; !ok || got == "" {
		t.Fatalf("expected fallback StructField key 'Hidden', got: %#v", m)
	}
}

func TestToFieldErrors_EmptyValidationErrorsReturnsEmpty(t *testing.T) {
	// Create an error of type validator.ValidationErrors with zero length
	verrs := make(globalValidator.ValidationErrors, 0)
	var err error = verrs
	m := ToFieldErrors(err)
	if len(m) != 0 {
		t.Fatalf("expected empty map for empty ValidationErrors, got %#v", m)
	}
}

func TestToFieldErrorsWith_EmptyValidationErrorsReturnsEmpty(t *testing.T) {
	// Create an error of type validator.ValidationErrors with zero length
	// Explicitly make empty (non-nil) slice to ensure type assertion passes
	verrs := make(globalValidator.ValidationErrors, 0)
	var err error = verrs
	m := ToFieldErrorsWith(err, nil)
	if len(m) != 0 {
		t.Fatalf("expected empty map for empty ValidationErrors, got %#v", m)
	}
}

func TestToFieldErrors_FallbackWhenFieldEmptyFromTagNameFunc(t *testing.T) {
	// Use a custom validator instance that forces Field() to return empty
	v := globalValidator.New()
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		if fld.Name == "A" {
			return "" // simulate no tag name -> Field() == ""
		}
		return fld.Tag.Get("json")
	})
	type Z struct {
		A string `json:"a" validate:"required"`
	}
	err := v.Struct(Z{})
	if err == nil {
		t.Fatalf("expected validation error")
	}
	m := ToFieldErrors(err)
	if _, ok := m["A"]; !ok {
		t.Fatalf("expected fallback to StructField 'A', got %#v", m)
	}
}

func TestToFieldErrorsWith_FallbackWhenFieldEmptyFromTagNameFunc(t *testing.T) {
	// Use a custom validator instance that forces Field() to return empty
	v := globalValidator.New()
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		if fld.Name == "A" {
			return "" // simulate no tag name -> Field() == ""
		}
		return fld.Tag.Get("json")
	})
	type Z struct {
		A string `json:"a" validate:"required"`
	}
	err := v.Struct(Z{})
	if err == nil {
		t.Fatalf("expected validation error")
	}
	m := ToFieldErrorsWith(err, nil)
	if _, ok := m["A"]; !ok {
		t.Fatalf("expected fallback to StructField 'A', got %#v", m)
	}
}

// Test cases for structured error parsing
func TestParseStructuredErrors_MapstructureStyle(t *testing.T) {
	errMsg := "1 error(s) decoding:\n\n* 'name' expected type 'string', got unconvertible type 'float64', value: '1'"
	result := parseStructuredErrors(errMsg)

	if len(result) != 1 {
		t.Fatalf("expected 1 field error, got %d: %#v", len(result), result)
	}

	if msg, ok := result["name"]; !ok {
		t.Fatalf("expected 'name' field, got %#v", result)
	} else if msg != "expected string but got float64" {
		t.Fatalf("expected proper error message, got %q", msg)
	}
}

func TestParseStructuredErrors_MultipleFields(t *testing.T) {
	errMsg := `3 error(s) decoding:

* 'name' expected type 'string', got unconvertible type 'float64', value: '1'
* 'age' expected type 'int', got unconvertible type 'string', value: 'abc'
* 'email' expected type 'string', got unconvertible type 'bool', value: 'true'`

	result := parseStructuredErrors(errMsg)

	if len(result) != 3 {
		t.Fatalf("expected 3 field errors, got %d: %#v", len(result), result)
	}

	expected := map[string]string{
		"name":  "expected string but got float64",
		"age":   "expected int but got string",
		"email": "expected string but got bool",
	}

	for field, expectedMsg := range expected {
		if msg, ok := result[field]; !ok {
			t.Fatalf("expected field %q, got %#v", field, result)
		} else if msg != expectedMsg {
			t.Fatalf("for field %q, expected %q, got %q", field, expectedMsg, msg)
		}
	}
}

func TestParseStructuredErrors_EmptyInput(t *testing.T) {
	result := parseStructuredErrors("")
	if len(result) != 0 {
		t.Fatalf("expected empty result for empty input, got %#v", result)
	}
}

func TestParseStructuredErrors_NoValidPattern(t *testing.T) {
	errMsg := "some random error message without field information"
	result := parseStructuredErrors(errMsg)
	if len(result) != 0 {
		t.Fatalf("expected empty result for non-matching input, got %#v", result)
	}
}

func TestToFieldErrorsWith_StructuredErrorFallback(t *testing.T) {
	// Create a simple error that contains structured error message
	errMsg := "1 error(s) decoding:\n\n* 'name' expected type 'string', got unconvertible type 'float64', value: '1'"
	err := simpleError{msg: errMsg}

	result := ToFieldErrorsWith(err, nil)

	if len(result) != 1 {
		t.Fatalf("expected 1 field error, got %d: %#v", len(result), result)
	}

	if msg, ok := result["name"]; !ok {
		t.Fatalf("expected 'name' field, got %#v", result)
	} else if msg != "expected string but got float64" {
		t.Fatalf("expected proper error message, got %q", msg)
	}
}

func TestToFieldErrorsWith_FallsBackToRawError(t *testing.T) {
	// Create an error that doesn't match any pattern
	err := simpleError2{msg: "some unparseable error"}
	result := ToFieldErrorsWith(err, nil)

	if len(result) != 1 {
		t.Fatalf("expected 1 fallback error, got %d: %#v", len(result), result)
	}

	if msg, ok := result["_error"]; !ok {
		t.Fatalf("expected '_error' fallback, got %#v", result)
	} else if msg != "some unparseable error" {
		t.Fatalf("expected original error message, got %q", msg)
	}
}

func TestParseInvalidKeys_Simple(t *testing.T) {
	line := "* '' has invalid keys: foo, bar"
	keys := parseInvalidKeys(line)
	assert.ElementsMatch(t, []string{"foo", "bar"}, keys)
}

func TestParseStructuredErrors_InvalidKeysAndTypeErrors_Merge(t *testing.T) {
	// Simulate an aggregated error that includes both a type error and invalid keys
	msg := `2 error(s) decoding:

* 'name' expected type 'string', got unconvertible type 'float64', value: '1'
* '' has invalid keys: extra, foo`

	got := parseStructuredErrors(msg)
	// Should include both the type error for name and unexpected markers for extra and foo
	assert.Equal(t, "expected string but got float64", got["name"])
	assert.Equal(t, "unexpected", got["extra"])
	assert.Equal(t, "unexpected", got["foo"])
}

func TestParseStructuredErrors_DoesNotOverwriteExistingKey(t *testing.T) {
	// Ensure when the same key appears as a type error and also in invalid keys,
	// the earlier, more specific message isn't overwritten by "unexpected".
	msg := `2 error(s) decoding:

* 'dup' expected type 'string', got unconvertible type 'float64'
* '' has invalid keys: dup, other`
	got := parseStructuredErrors(msg)
	assert.Equal(t, "expected string but got float64", got["dup"]) // preserved
	assert.Equal(t, "unexpected", got["other"])                    // added
}

func TestToFieldErrorsWith_CtxFieldErrors_EmptyAggregatedMessage(t *testing.T) {
	fe := fakeCtxFieldErrorsWithMsg{
		list: []ctx.FieldError{
			fakeCtxFieldError{f: "field", m: "bad"},
		},
		msg: "", // empty Error() should skip structured parsing branch
	}
	m := ToFieldErrorsWith(fe, nil)
	assert.Equal(t, map[string]string{"field": "bad"}, m)
}

func TestToFieldErrorsWith_CtxFieldErrors_NonEmptyMsgNoExtras(t *testing.T) {
	// Non-empty aggregated message that doesn't match parsing patterns -> no extras merged
	fe := fakeCtxFieldErrorsWithMsg{
		list: []ctx.FieldError{
			fakeCtxFieldError{f: "a", m: "oops"},
		},
		msg: "1 error(s) decoding:\n\n* something random without patterns",
	}
	m := ToFieldErrorsWith(fe, nil)
	assert.Equal(t, map[string]string{"a": "oops"}, m)
}

func TestToFieldErrorsWith_CtxFieldErrors_DuplicateKeyNotOverwritten(t *testing.T) {
	// All() provides a field 'name', aggregated message also mentions 'name' type error.
	// Merge should NOT overwrite the original value in res.
	fe := fakeCtxFieldErrorsWithMsg{
		list: []ctx.FieldError{
			fakeCtxFieldError{f: "name", m: "fromAll"},
		},
		msg: "1 error(s) decoding:\n\n* 'name' expected type 'string', got unconvertible type 'float64'",
	}
	m := ToFieldErrorsWith(fe, nil)
	assert.Equal(t, "fromAll", m["name"]) // ensure not overwritten by merged extras
}

func TestToFieldErrorsWith_CtxFieldErrors_OnlyAggregatedExtras(t *testing.T) {
	// All() is empty; extras should be populated solely from aggregated message parsing
	fe := fakeCtxFieldErrorsWithMsg{
		list: nil,
		msg:  "2 error(s) decoding:\n\n* 'name' expected type 'string', got unconvertible type 'float64'\n* '' has invalid keys: extraKey",
	}
	m := ToFieldErrorsWith(fe, nil)
	// Should include both parsed entries even when All() is empty
	assert.Equal(t, "expected string but got float64", m["name"])
	assert.Equal(t, "unexpected", m["extraKey"])
}

func TestNormalizeFieldKey_OnlyWhitespaceAndCR(t *testing.T) {
	if got := normalizeFieldKey("   \t   "); got != "" {
		t.Fatalf("expected empty after trimming whitespace, got %q", got)
	}
	if got := normalizeFieldKey("foo\rbar"); got != "" {
		t.Fatalf("expected empty for CR newline, got %q", got)
	}
}

func TestNormalizeFieldKey_Variants(t *testing.T) {
	// empty input
	if got := normalizeFieldKey(""); got != "" {
		t.Fatalf("expected empty for empty input, got %q", got)
	}
	// whitespace trimmed
	if got := normalizeFieldKey("  name  "); got != "name" {
		t.Fatalf("expected 'name', got %q", got)
	}
	// allowed characters
	allowed := []string{"user.name", "user-name", "user_name", "aB9"}
	for _, s := range allowed {
		if got := normalizeFieldKey(s); got != s {
			t.Fatalf("expected %q to be allowed, got %q", s, got)
		}
	}
	// invalid characters -> empty
	invalid := []string{"user:name", "user/name", "sp ace", "名"}
	for _, s := range invalid {
		if got := normalizeFieldKey(s); got != "" {
			t.Fatalf("expected empty for %q, got %q", s, got)
		}
	}
	// embedded newline -> empty
	if got := normalizeFieldKey("foo\nbar"); got != "" {
		t.Fatalf("expected empty for newline, got %q", got)
	}
}

func TestParseFieldTypeError_Variants(t *testing.T) {
	// simpler 'got' pattern (no quotes and no 'unconvertible')
	line := "* 'age' expected type 'int', got bool, value: 1"
	m := parseFieldTypeError(line)
	if m == nil || m["field"] != "age" || m["expected"] != "int" || m["got"] != "bool" {
		t.Fatalf("unexpected map for simple got pattern: %#v", m)
	}

	// missing expected, only 'got unconvertible type'
	line2 := "* 'x' got unconvertible type 'float64'"
	m2 := parseFieldTypeError(line2)
	if m2 == nil || m2["field"] != "x" || m2["expected"] != "" || m2["got"] != "float64" {
		t.Fatalf("unexpected map when expected missing: %#v", m2)
	}

	// no field (no single quotes anywhere) -> should return nil
	line3 := "* expected type string, got int"
	if m3 := parseFieldTypeError(line3); m3 != nil {
		t.Fatalf("expected nil when field name missing, got %#v", m3)
	}
}

func TestParseStructuredErrors_InvalidTypeFallback(t *testing.T) {
	// When either expected or got is missing, message should be generic 'invalid type'
	msg := "1 error(s) decoding:\n\n* 'name' got unconvertible type 'float64'"
	got := parseStructuredErrors(msg)
	if got["name"] != "invalid type" {
		t.Fatalf("expected 'invalid type', got %#v", got)
	}
}

func TestParseInvalidKeys_TrimmingAndEmpty(t *testing.T) {
	// Trimming quotes, dots, and closing parens
	line := "* '' has invalid keys: 'foo'., bar)."
	keys := parseInvalidKeys(line)
	// Note: current implementation may leave a trailing single quote when punctuation follows quotes
	assert.ElementsMatch(t, []string{"foo'", "bar"}, keys)

	// No keys after colon -> nil
	if keys2 := parseInvalidKeys("* '' has invalid keys:   "); len(keys2) != 0 {
		t.Fatalf("expected nil/empty for no keys, got %#v", keys2)
	}
}

func TestParseInvalidKeys_NoPhrase(t *testing.T) {
	if keys := parseInvalidKeys("totally unrelated line"); keys != nil {
		t.Fatalf("expected nil for no-phrase, got %#v", keys)
	}
}

func TestParseStructuredErrors_SkipsUnmatchedStarLines(t *testing.T) {
	msg := "2 error(s) decoding:\n\n* something random\n* another random without patterns"
	if got := parseStructuredErrors(msg); len(got) != 0 {
		t.Fatalf("expected empty for unmatched star lines, got %#v", got)
	}
}

type emptyErr struct{}

func (emptyErr) Error() string { return "" }

func TestToFieldErrorsWith_EmptyRawErrorMessage(t *testing.T) {
	// Ensure branch where err.Error() == "" in the raw error fallback path is covered
	m := ToFieldErrorsWith(emptyErr{}, nil)
	if _, ok := m["_error"]; !ok {
		t.Fatalf("expected _error key for empty error message, got %#v", m)
	}
}

// --- Direct helper coverage for negative branches ---

func Test_handleCtxFieldErrors_NotMatchingType(t *testing.T) {
	res := map[string]string{}
	ok := handleCtxFieldErrors(assert.AnError, res)
	assert.False(t, ok)
	assert.Empty(t, res)
}

func Test_handleValidationErrors_NotMatchingType(t *testing.T) {
	res := map[string]string{}
	ok := handleValidationErrors(assert.AnError, res, nil)
	assert.False(t, ok)
	assert.Empty(t, res)
}

func Test_handleDirectFieldErrors_NotMatchingType(t *testing.T) {
	res := map[string]string{}
	ok := handleDirectFieldErrors(assert.AnError, res)
	assert.False(t, ok)
	assert.Empty(t, res)
}

type msgOnlyErr struct{ s string }

func (e msgOnlyErr) Error() string { return e.s }

func Test_handleStructuredErrorMessage_NoStructured(t *testing.T) {
	res := map[string]string{}
	ok := handleStructuredErrorMessage(msgOnlyErr{"no patterns here"}, res)
	assert.False(t, ok)
	assert.Empty(t, res)
}

// Mock FieldError to trigger field=="" fallback path in handleValidationErrors
type fakeValFE struct{}

func (fakeValFE) Tag() string                    { return "" }
func (fakeValFE) ActualTag() string              { return "" }
func (fakeValFE) Namespace() string              { return "" }
func (fakeValFE) StructNamespace() string        { return "" }
func (fakeValFE) Field() string                  { return "" } // force empty
func (fakeValFE) StructField() string            { return "S" }
func (fakeValFE) Param() string                  { return "" }
func (fakeValFE) Kind() reflect.Kind             { return reflect.String }
func (fakeValFE) Type() reflect.Type             { return reflect.TypeOf("") }
func (fakeValFE) Value() any                     { return "" }
func (fakeValFE) Translate(ut.Translator) string { return "" }
func (fakeValFE) Error() string                  { return "" }

func Test_handleValidationErrors_FieldEmpty_FallbackToStructField(t *testing.T) {
	// Build a validator.ValidationErrors with our fake FieldError
	verrs := globalValidator.ValidationErrors{fakeValFE{}}
	var err error = verrs
	m := ToFieldErrorsWith(err, func(fe globalValidator.FieldError) string { return "OK" })
	assert.Equal(t, map[string]string{"S": "OK"}, m)
}
