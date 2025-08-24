# Validation helpers and i18n middleware for the GoFlash framework

<h1 align="center">
    <a href="https://pkg.go.dev/github.com/goflash/validator/v2@v2.0.0">
        <img src="https://pkg.go.dev/badge/github.com/goflash/validator.svg" alt="Go Reference">
    </a>
    <a href="https://goreportcard.com/report/github.com/goflash/validator">
        <img src="https://img.shields.io/badge/%F0%9F%93%9D%20Go%20Report-A%2B-75C46B?style=flat-square" alt="Go Report Card">
    </a>
    <a href="https://codecov.io/gh/goflash/validator">
        <img src="https://codecov.io/gh/goflash/validator/graph/badge.svg" alt="Coverage">
    </a>
    <a href="https://github.com/goflash/validator/actions?query=workflow%3ATest">
        <img src="https://img.shields.io/github/actions/workflow/status/goflash/validator/test-coverage.yml?branch=main&label=%F0%9F%A7%AA%20Tests&style=flat-square&color=75C46B" alt="Tests">
    </a>
    <img src="https://img.shields.io/badge/go-1.23%2B-00ADD8?logo=golang" alt="Go Version">
    <a href="https://docs.goflash.dev">
        <img src="https://img.shields.io/badge/%F0%9F%92%A1%20GoFlash-docs-00ACD7.svg?style=flat-square" alt="GoFlash Docs">
    </a>
    <img src="https://img.shields.io/badge/status-stable-green" alt="Status">
    <img src="https://img.shields.io/badge/license-MIT-blue" alt="License">
    <br>
    <div style="text-align:center">
      <a href="https://discord.gg/QHhGHtjjQG">
        <img src="https://dcbadge.limes.pink/api/server/https://discord.gg/QHhGHtjjQG" alt="Discord">
      </a>
    </div>
</h1>

Lightweight validation helpers powered by go-playground/validator and a tiny i18n middleware for the GoFlash framework. It gives you a ready-to-use validator instance, helper mappers to field->message maps, and a middleware to plug app-level translations without pulling locale packages into the framework.

## Features

- Uses github.com/go-playground/validator/v10 under the hood
- Global validate.Validator with JSON tag name support out of the box
- Helpers to map validator.ValidationErrors into map[field]message
- Pluggable message function: global or per-request via context
- i18n middleware that wires your translators in one place

## Installation

```sh
go get github.com/goflash/validator/v2
```

Go version: requires Go 1.23+. The module sets `go 1.23` and can be used with newer Go versions. If you use `GOTOOLCHAIN=auto`, the `toolchain` directive will ensure a compatible toolchain is used.

## Quick start

```go
import (
    "github.com/goflash/flash/v2"
    "github.com/goflash/validator/v2/validate"
)

type User struct {
    Name string `json:"name" validate:"required,min=2"`
    Age  int    `json:"age"  validate:"gte=0,lte=130"`
}

func main() {
    a := flash.New()
    a.POST("/users", func(c flash.Ctx) error {
        var u User
        if err := c.BindJSON(&u); err != nil {
            return c.JSON(map[string]any{"message": "invalid payload", "error": err.Error()})
        }
        if err := validate.Struct(u); err != nil {
            return c.JSON(map[string]any{"message": "validation failed", "fields": validate.ToFieldErrors(err)})
        }
        return c.JSON(u)
    })
}
```

## Configuration

Use ValidatorI18n to attach localized messages per request:

```go
import (
    "github.com/goflash/flash/v2"
    v10 "github.com/go-playground/validator/v10"
    "github.com/goflash/validator/v2/validate"
    mw "github.com/goflash/validator/v2"
)

app := flash.New()
app.Use(mw.ValidatorI18n(mw.ValidatorI18nConfig{
    DefaultLocale: "en",
    MessageFuncFor: func(locale string) func(v10.FieldError) string {
        // Look up your translator for the locale and return fe.Translate(trans)
        return func(fe v10.FieldError) string { return fe.Error() }
    },
    SetGlobal: true, // optionally set global fallback to DefaultLocale
}))
```

### Messages and mapping

- Register custom tags and tag-name functions directly on `validate.Validator`.
- Map errors with `validate.ToFieldErrors(err)` or `validate.ToFieldErrorsWithContext(ctx, err)`.
- Provide request-scoped message function via middleware or `validate.WithMessageFunc(ctx, fn)`.

### Default messages

Built-in minimal fallback messages cover common tags like required, min/max/len, email, oneof, gte/lte, url, uuid, alpha/alphanum/numeric, contains/excludes, startswith/endswith, base64, json, ip/cidr, ascii/printascii/multibyte, isbn/isbn10/isbn13.

### Context

The middleware stores a request-scoped message function on the request context. Use `validate.MessageFuncFromContext(c.Context())` to retrieve it if needed.

### Errors

When mapping errors, non-validation errors are returned under the `_error` key. You can also pass your own `validate.FieldErrors` map.

## Examples

Three runnable examples are included:

- examples/binding_json: simple JSON bind and respond
- examples/validation: struct validation and error mapping
- examples/validation_with_i18n: localized messages using the middleware

Try them locally (they use this module):

```sh
cd examples/binding_json && go run .
cd ../validation && go run .
cd ../validation_with_i18n && go run .
```

## Versioning and compatibility

- Module path: `github.com/goflash/validator/v2`
- Requires Go 1.23+
- Versioning starts at v2.0.0

## Contributing

Issues and PRs are welcome. Please run tests before submitting:

```sh
go test ./...
```

## License

MIT
