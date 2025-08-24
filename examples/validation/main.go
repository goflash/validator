package main

import (
	"log"
	"net/http"

	"github.com/goflash/flash/v2"
	"github.com/goflash/flash/v2/ctx"
	"github.com/goflash/validator/v2/validate"
)

type signupReq struct {
	Email string `json:"email" validate:"required,email"`
	Name  string `json:"name" validate:"required,min=2"`
}

func main() {
	a := flash.New()

	a.POST("/signup", func(c flash.Ctx) error {
		var in signupReq
		if err := c.BindJSON(&in, ctx.BindJSONOptions{ErrorUnused: true}); err != nil {
			return c.Status(http.StatusUnprocessableEntity).JSON(map[string]any{
				"message":        "invalid payload structure",
				"fields":         validate.ToFieldErrors(err),
				"original_error": err.Error(),
			})
		}
		if err := validate.Struct(in); err != nil {
			return c.Status(http.StatusUnprocessableEntity).JSON(map[string]any{
				"message":        "validation failed",
				"fields":         validate.ToFieldErrors(err),
				"original_error": err.Error(),
			})
		}
		return c.JSON(in)
	})
	log.Fatal(http.ListenAndServe(":8080", a))
}
