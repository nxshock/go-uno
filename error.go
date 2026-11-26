package uno

import (
	"errors"
	"fmt"
)

type DecodeError struct {
	Line      uint
	Err       error
	FieldName string
	Expected  string
	Got       string
}

func (de *DecodeError) Error() string {
	if errors.Is(de.Err, ErrFieldIsNotExported) || errors.Is(de.Err, ErrFieldIsNotExists) {
		return fmt.Sprintf("line %d: %v: %s", de.Line, de.Err, de.FieldName)
	}

	return fmt.Sprintf("line %d: %v, expected %s, got %s", de.Line, de.Err, de.Expected, de.Got)
}

func (de *DecodeError) Unwrap() error {
	return de.Err
}

func newDecodeError(line uint, err error, fieldName string, expected string, got string) error {
	return fmt.Errorf("%w", &DecodeError{
		Line:      line,
		Err:       err,
		FieldName: fieldName,
		Expected:  expected,
		Got:       got})
}
