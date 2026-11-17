package uno

import (
	"errors"
	"time"
)

var defaultDecoderTimeLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	time.DateOnly,
	time.TimeOnly}

var defaultEncoderTimeLayout = time.RFC3339

var (
	ErrSpaceInKey                     = errors.New("spaces is not allowed in keys")
	ErrInvalidBoolValue               = errors.New("invalid bool value")
	ErrFieldIsNotExists               = errors.New("field is not exists")
	ErrFieldIsNotExported             = errors.New("field is not exported")
	ErrMapKeyIsNotString              = errors.New("map key is not string")
	ErrDelimiterIsNotTypedForBool     = errors.New("delimiter is not typed for bool")
	ErrUnsupportedType                = errors.New("unsupported type")
	ErrInvalidMultilineStringBoundary = errors.New("invalid multiline strng boundary")
	ErrInvalidIndentation             = errors.New("invalid indentation")
)
