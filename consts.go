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
	ErrSpaceInKey     = errors.New("spaces is not allowed in keys")
	ErrWrongBoolValue = errors.New("wrong bool value")
)
