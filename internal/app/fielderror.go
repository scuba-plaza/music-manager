package app

import "fmt"

type FieldErrors map[string]string

func Invalid(field, message string) error {
	return FieldErrors{field: message}
}

func (e FieldErrors) Error() string {
	if len(e) == 1 {
		for _, message := range e {
			return message
		}
	}
	return fmt.Sprintf("%d fields need fixing", len(e))
}

func (e FieldErrors) OrNil() error {
	if len(e) == 0 {
		return nil
	}
	return e
}
