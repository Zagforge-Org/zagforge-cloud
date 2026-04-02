package validate

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

// V is the shared validator instance.
var V = validator.New(validator.WithRequiredStructEnabled())

// Struct validates a struct and returns a user-friendly error.
func Struct[T any](s T) error {
	if err := V.Struct(s); err != nil {
		if ve, ok := errors.AsType[validator.ValidationErrors](err); ok {
			return formatErrors(ve)
		}
		return err
	}
	return nil
}

func formatErrors(ve validator.ValidationErrors) error {
	msgs := make([]string, 0, len(ve))
	for _, fe := range ve {
		msgs = append(msgs, fieldError(fe))
	}
	return errors.New(strings.Join(msgs, "; "))
}

func fieldError(fe validator.FieldError) string {
	field := fe.Field()
	switch fe.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "uuid":
		return fmt.Sprintf("%s must be a valid UUID", field)
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", field, fe.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s characters", field, fe.Param())
	case "min":
		return fmt.Sprintf("%s must be at least %s characters", field, fe.Param())
	default:
		return fmt.Sprintf("%s failed on %s validation", field, fe.Tag())
	}
}
