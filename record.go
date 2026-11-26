package uno

import (
	"encoding"
	"fmt"
	"math/bits"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type directive rune

const (
	directiveNone       = directive(0)
	directiveMetadata   = directive('#')
	directiveAssembly   = directive('@')
	directiveValidation = directive('$')
)

type itemMarker rune

const (
	itemMarkerNone        = itemMarker(0)
	itemMarkerSliceString = itemMarker('-')
	itemMarkerSliceTyped  = itemMarker('=')
	itemMarkerMap         = itemMarker('.')
)

type delimiter rune

const (
	delimiterNone   = delimiter(0)
	delimiterString = delimiter(':')
	delimiterTyped  = delimiter('=')
)

type record struct {
	Indent     uint
	Directive  directive
	ItemMarker itemMarker
	Key        string
	Delimiter  delimiter
	Value      string

	Comment string

	lineNumber              uint
	multilineStringBoundary string
}

func (r *record) decodeValue(valType reflect.Type) (any, error) {
	if valType.String() != "time.Time" {
		val := reflect.New(valType).Interface()
		if f, ok := val.(encoding.TextUnmarshaler); ok {
			err := f.UnmarshalText([]byte(r.Value))
			return f, err
		}
	}

	for valType.Kind() == reflect.Pointer {
		valType = valType.Elem()
	}

	switch valType.Kind() {
	case reflect.Struct:
		if valType.String() == "time.Time" {
			for _, layout := range []string{time.RFC3339Nano, time.RFC3339, time.DateOnly, time.TimeOnly} {
				t, err := time.Parse(layout, r.Value)
				if err == nil {
					return t, nil
				}
			}
			return nil, newDecodeError(r.lineNumber, ErrInvalidTimeLayout, r.Key, "", "")
		}
	case reflect.Int:
		i, err := strconv.ParseInt(r.Value, 10, bits.UintSize)
		return int(i), err
	case reflect.Int8:
		i, err := strconv.ParseInt(r.Value, 10, 8)
		return int8(i), err
	case reflect.Int16:
		i, err := strconv.ParseInt(r.Value, 10, 16)
		return int16(i), err
	case reflect.Int32:
		i, err := strconv.ParseInt(r.Value, 10, 32)
		return int32(i), err
	case reflect.Int64:
		return strconv.ParseInt(r.Value, 10, 64)
	case reflect.Float32:
		f, err := strconv.ParseFloat(r.Value, 32)
		return float32(f), err
	case reflect.Float64:
		return strconv.ParseFloat(r.Value, 64)
	case reflect.Uint:
		i, err := strconv.ParseUint(r.Value, 10, bits.UintSize)
		return uint(i), err
	case reflect.Uint8:
		u, err := strconv.ParseUint(r.Value, 10, 8)
		return uint8(u), err
	case reflect.Uint16:
		u, err := strconv.ParseUint(r.Value, 10, 16)
		return uint16(u), err
	case reflect.Uint32:
		u, err := strconv.ParseUint(r.Value, 10, 32)
		return uint32(u), err
	case reflect.Uint64:
		return strconv.ParseUint(r.Value, 10, 64)
	case reflect.Bool:
		if r.Delimiter != delimiterTyped {
			return false, newDecodeError(r.lineNumber, ErrDelimiterIsNotTypedForBool, r.Key, "", "")
		}

		switch r.Value {
		case "true", "on", "open", "yes":
			return true, nil
		case "false", "off", "close", "no":
			return false, nil
		}

		return false, newDecodeError(r.lineNumber, ErrInvalidBoolValue, r.Key, "", r.Value)
	case reflect.String:
		return r.Value, nil
	}

	return nil, fmt.Errorf("line %d: %w: %s", r.lineNumber, ErrUnsupportedType, valType.String())
}

func (r *record) size() uint {
	if r == nil {
		return 0
	}

	newLinesCount := strings.Count(r.Value, "\n")

	if newLinesCount > 0 || r.multilineStringBoundary != "" {
		return uint(newLinesCount) + 3
	}

	return 1
}
