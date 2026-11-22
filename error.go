package uno

import (
	"errors"
	"fmt"
)

type DecodeError[T any] struct { // TODO: limit types to interface{ uint | string }?
	Line      uint
	Err       error
	FieldName string
	Expected  T
	Got       T
}

func (de *DecodeError[T]) Error() string {
	if errors.Is(de.Err, ErrFieldIsNotExported) || errors.Is(de.Err, ErrFieldIsNotExists) {
		return fmt.Sprintf("line %d: %v: %s", de.Line, de.Err, de.FieldName)
	}

	return fmt.Sprintf("line %d: %v, expected %v, got %v", de.Line, de.Err, de.Expected, de.Got)
}

func (de *DecodeError[T]) Unwrap() error {
	return de.Err
}

func newDecodeError[T any](line uint, err error, fieldName string, expected T, got T) error {
	return fmt.Errorf("%w", &DecodeError[T]{
		Line:      line,
		Err:       err,
		FieldName: fieldName,
		Expected:  expected,
		Got:       got})
}
