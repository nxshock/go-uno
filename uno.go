package uno

import (
	"bytes"
	"fmt"
	"math/bits"
	"reflect"
	"strconv"
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
	itemMarkerNone   = itemMarker(' ')
	itemMarkerString = itemMarker('-')
	itemMarkerTyped  = itemMarker('=')
	itemMarkerStruct = itemMarker('.')
)

type delimiter rune

const (
	delimiterNone   = delimiter(' ')
	delimiterString = delimiter(':')
	delimiterTyped  = delimiter('=')
)

type valueParseFuncs map[reflect.Kind]func(*record) (value any, err error)

var valParseFuncs valueParseFuncs

func Unmarshal(in []byte, out any) error {
	return NewDecoder(bytes.NewReader(in)).Decode(out)
}

func Marshal(in any) (out []byte, err error) {
	buf := new(bytes.Buffer)

	err = NewEncoder(buf).Encode(in)

	return buf.Bytes(), nil
}

func init() {
	valParseFuncs = make(valueParseFuncs)

	valParseFuncs[reflect.Int] = func(r *record) (any, error) {
		i, err := strconv.ParseInt(r.Value, 10, bits.UintSize)
		return int(i), err
	}

	valParseFuncs[reflect.Int64] = func(r *record) (any, error) {
		return strconv.ParseInt(r.Value, 10, 64)
	}

	valParseFuncs[reflect.Float64] = func(r *record) (any, error) {
		return strconv.ParseFloat(r.Value, 64)
	}

	valParseFuncs[reflect.Uint] = func(r *record) (any, error) {
		i, err := strconv.ParseUint(r.Value, 10, bits.UintSize)
		return uint(i), err
	}

	valParseFuncs[reflect.Uint64] = func(r *record) (any, error) {
		return strconv.ParseUint(r.Value, 10, 64)
	}

	valParseFuncs[reflect.Bool] = func(r *record) (any, error) {
		if r.Delimiter != delimiterTyped {
			return false, fmt.Errorf("delimiter is not typed")
		}

		switch r.Value {
		case "true", "on", "open", "yes":
			return true, nil
		case "false", "off", "close", "no":
			return false, nil
		}

		return false, fmt.Errorf("wrong bool value: %s", r.Value)
	}

	valParseFuncs[reflect.String] = func(r *record) (any, error) {
		return r.Value, nil
	}
}
