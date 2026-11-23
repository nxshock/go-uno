package uno

import (
	"encoding"
	"fmt"
	"io"
	"math/bits"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type directive rune

const (
	directiveNone       = directive(' ')
	directiveMetadata   = directive('#')
	directiveAssembly   = directive('@')
	directiveValidation = directive('$')
)

type itemMarker rune

const (
	itemMarkerNone        = itemMarker(' ')
	itemMarkerSliceString = itemMarker('-')
	itemMarkerSliceTyped  = itemMarker('=')
	itemMarkerMap         = itemMarker('.')
)

type delimiter rune

const (
	delimiterNone   = delimiter(' ')
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

func parseRecord(s string, comment string, lineNumber uint) (*record, error) {
	var firstChar *byte

	// indent
	indent := uint(0)
	for len(s) > 0 && s[0] == '\t' {
		ch := s[0]
		s = s[1:]

		if ch == '\t' {
			indent += 1
		} else {
			firstChar = &ch
			break
		}
	}

	// directive
	dir := directiveNone
	ch := byte(0)
	if firstChar != nil {
		ch = *firstChar
		firstChar = nil
	} else {
		ch = s[0]
		s = s[1:]
	}
	if ch != ' ' {
		switch directive(ch) {
		case directiveMetadata, directiveAssembly, directiveValidation:
			dir = directive(ch)

			if len(s) == 0 {
				return nil, fmt.Errorf("read directive: %v", ErrNoSpaceAfterDirective)
			}

			if s[0] == ' ' {
				s = s[1:]
			} else {
				return nil, fmt.Errorf("read directive: %v", ErrNoSpaceAfterDirective)
			}
		default:
			firstChar = &ch
		}
	} else {
		return nil, ErrSpaceInKey
	}

	// item marker
	var keyBuilder strings.Builder
	itmMarker := itemMarkerNone

	if firstChar != nil {
		ch = *firstChar
		firstChar = nil
	} else {
		if len(s) == 0 {
			return nil, io.ErrUnexpectedEOF // TODO: add proper error description
		}

		ch = s[0]
		s = s[1:]
	}

	switch itemMarker(ch) {
	case itemMarkerSliceString, itemMarkerSliceTyped, itemMarkerMap:
		itmMarker = itemMarker(ch)
		if len(s) == 0 {
			return &record{Indent: indent, Directive: dir, ItemMarker: itmMarker, Delimiter: delimiterNone, lineNumber: lineNumber}, nil
		}

		if s[0] != ' ' {
			keyBuilder.WriteRune(rune(itmMarker))
			keyBuilder.WriteRune(rune(s[0]))
			itmMarker = itemMarkerNone
		}
		s = s[1:]
	default:
		firstChar = &ch
	}

	// key and delim
	delim := delimiter(' ')
	if itmMarker == itemMarkerNone || itmMarker == itemMarkerMap {
		for {
			brk := false

			if firstChar != nil {
				ch = *firstChar
				firstChar = nil
			} else {
				ch = s[0]
				s = s[1:]
			}

			switch delimiter(rune(ch)) {
			case delimiterString, delimiterTyped:
				delim = delimiter(ch)

				if len(s) == 0 {
					brk = true
				}

				if !brk && s[0] != ' ' {
					return nil, fmt.Errorf("expected space after delimiter, got '%s'", string(ch))
				}
				if len(s) > 0 {
					s = s[1:]
				}
				brk = true
			default:
				keyBuilder.WriteByte(ch)
			}

			if brk {
				break
			}
		}
	}
	key := keyBuilder.String()
	if strings.HasSuffix(key, " ") {
		return nil, ErrSpaceInKey
	}
	if key == "''" {
		key = ""
	}

	// value
	value := strings.TrimSpace(s)
	if len(value) > 1 && value[0] == '\'' && value[len(value)-1] == '\'' {
		value = value[1 : len(value)-1]
	}

	rec := &record{
		Indent:     indent,
		Directive:  dir,
		ItemMarker: itmMarker,
		Key:        key,
		Value:      value,
		Delimiter:  delim,
		Comment:    comment,
		lineNumber: lineNumber}

	return rec, nil
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
			return nil, fmt.Errorf("line %d: %w: %s", r.lineNumber, ErrInvalidTimeLayout, r.Value)
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
			return false, fmt.Errorf("line %d: %w", r.lineNumber, ErrDelimiterIsNotTypedForBool)
		}

		switch r.Value {
		case "true", "on", "open", "yes":
			return true, nil
		case "false", "off", "close", "no":
			return false, nil
		}

		return false, fmt.Errorf("line %d: %w: \"%s\"", r.lineNumber, ErrInvalidBoolValue, r.Value)
	case reflect.String:
		return r.Value, nil
	}

	return nil, fmt.Errorf("line %d: %w: %s", r.lineNumber, ErrUnsupportedType, valType.String())
}
