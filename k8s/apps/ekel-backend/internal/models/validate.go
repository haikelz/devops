package models

import "github.com/go-playground/validator/v10"

type CustomValidator struct {
	Validator *validator.Validate
}

func NewCustomValidator() *CustomValidator {
	return &CustomValidator{Validator: validator.New()}
}

func (cv *CustomValidator) Validate(i any) error {
	return cv.Validator.Struct(i)
}
