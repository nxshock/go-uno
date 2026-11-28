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
	return decodeValue(r.Value, valType)
}

func decodeValue(s string, valType reflect.Type) (any, error) {
	if valType.String() != "time.Time" {
		val := reflect.New(valType).Interface()
		if f, ok := val.(encoding.TextUnmarshaler); ok {
			err := f.UnmarshalText([]byte(s))
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
				t, err := time.Parse(layout, s)
				if err == nil {
					return t, nil
				}
			}
			return nil, ErrInvalidTimeLayout
		}
	case reflect.Int:
		i, err := strconv.ParseInt(s, 10, bits.UintSize)
		return int(i), err
	case reflect.Int8:
		i, err := strconv.ParseInt(s, 10, 8)
		return int8(i), err
	case reflect.Int16:
		i, err := strconv.ParseInt(s, 10, 16)
		return int16(i), err
	case reflect.Int32:
		i, err := strconv.ParseInt(s, 10, 32)
		return int32(i), err
	case reflect.Int64:
		return strconv.ParseInt(s, 10, 64)
	case reflect.Float32:
		f, err := strconv.ParseFloat(s, 32)
		return float32(f), err
	case reflect.Float64:
		return strconv.ParseFloat(s, 64)
	case reflect.Uint:
		i, err := strconv.ParseUint(s, 10, bits.UintSize)
		return uint(i), err
	case reflect.Uint8:
		u, err := strconv.ParseUint(s, 10, 8)
		return uint8(u), err
	case reflect.Uint16:
		u, err := strconv.ParseUint(s, 10, 16)
		return uint16(u), err
	case reflect.Uint32:
		u, err := strconv.ParseUint(s, 10, 32)
		return uint32(u), err
	case reflect.Uint64:
		return strconv.ParseUint(s, 10, 64)
	case reflect.Bool:
		switch s {
		case "true", "on", "open", "yes":
			return true, nil
		case "false", "off", "close", "no":
			return false, nil
		}

		return false, ErrInvalidBoolValue
	case reflect.String:
		return s, nil
	case reflect.Interface:
		return s, nil
	}

	return nil, ErrUnsupportedType
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

func (r *record) String() string {
	switch r.ItemMarker {
	case itemMarkerNone:
		dir := iif(r.Directive != directiveNone, string(r.Directive)+" ", "")
		key := iif(r.Key != "", r.Key, "''")
		val := iif(r.Value != "", r.Value, "''")

		return fmt.Sprintf("%s%s%s%s %v\n", strings.Repeat("\t", int(r.Indent)), dir, key, string(r.Delimiter), val)
	case itemMarkerMap:
		if r.Key == "" && r.Value == "" {
			return ".\n"
		}

		key := iif(r.Key != "", r.Key, "''")
		val := iif(r.Value != "", r.Value, "''")
		return fmt.Sprintf("%s%s%s%s %v\n", strings.Repeat("\t", int(r.Indent)), string(r.ItemMarker), key, string(r.Delimiter), val)
	case itemMarkerSliceString, itemMarkerSliceTyped:
		val := iif(r.Value != "", r.Value, "''")
		return fmt.Sprintf("%s%s %v\n", strings.Repeat("\t", int(r.Indent)), string(r.ItemMarker), val)
	}

	return ""
}
