package main

import (
	"log"
	"net/http"

	v10 "github.com/go-playground/validator/v10"
	"github.com/goflash/flash/v2"
	mw "github.com/goflash/validator/v2"
	"github.com/goflash/validator/v2/validate"

	// App-level i18n wiring
	"github.com/go-playground/locales/en"
	"github.com/go-playground/locales/es"
	ut "github.com/go-playground/universal-translator"
	enTranslations "github.com/go-playground/validator/v10/translations/en"
	esTranslations "github.com/go-playground/validator/v10/translations/es"
)

// User represents a user for the validation example.
type User struct {
	Name string `json:"name" validate:"required,min=2"`
	Age  int    `json:"age"  validate:"gte=0,lte=130"`
}

func main() {
	// Prepare translators
	enLoc := en.New()
	esLoc := es.New()
	uni := ut.New(enLoc, enLoc, esLoc)

	translators := map[string]ut.Translator{}
	if t, ok := uni.GetTranslator("en"); ok {
		_ = enTranslations.RegisterDefaultTranslations(validate.Validator, t)
		translators["en"] = t
	}
	if t, ok := uni.GetTranslator("es"); ok {
		_ = esTranslations.RegisterDefaultTranslations(validate.Validator, t)
		translators["es"] = t
	}

	app := flash.New()

	// Install validator i18n middleware: derive locale from :lang and attach translator function.
	app.Use(mw.ValidatorI18n(mw.ValidatorI18nConfig{
		DefaultLocale: "en",
		MessageFuncFor: func(locale string) func(v10.FieldError) string {
			if trans, ok := translators[locale]; ok {
				return func(fe v10.FieldError) string { return fe.Translate(trans) }
			}
			if trans, ok := translators["en"]; ok {
				return func(fe v10.FieldError) string { return fe.Translate(trans) }
			}
			return nil
		},
		SetGlobal: true, // optional: set global fallback to DefaultLocale
	}))

	// POST /<lang>/users accepts a JSON user and validates fields using framework validation.
	app.POST("/:lang/users", func(c flash.Ctx) error {
		var u User
		if err := c.BindJSON(&u); err != nil {
			return c.Status(http.StatusUnprocessableEntity).JSON(map[string]any{
				"message":        "invalid payload structure",
				"fields":         validate.ToFieldErrorsWithContext(c.Context(), err),
				"original_error": err.Error(),
			})
		}
		if err := validate.Struct(u); err != nil {
			return c.Status(http.StatusUnprocessableEntity).JSON(map[string]any{
				"message":        "validation failed",
				"fields":         validate.ToFieldErrorsWithContext(c.Context(), err),
				"original_error": err.Error(),
			})
		}
		return c.JSON(u)
	})

	log.Fatal(http.ListenAndServe(":8080", app))
}
