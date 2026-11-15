package app

import (
	"fmt"
	"unicode"
	"unicode/utf8"
)

type FieldErrors map[string]string

func Invalid(field, message string) error {
	e := FieldErrors{}
	e.Set(field, message)
	return e
}

func (e FieldErrors) Set(field, message string) {
	if message == "" {
		e[field] = ""
		return
	}
	first, size := utf8.DecodeRuneInString(message)
	e[field] = string(unicode.ToUpper(first)) + message[size:]
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
