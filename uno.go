package uno

import (
	"bytes"
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

func Unmarshal(in []byte, out any) error {
	return NewDecoder(bytes.NewReader(in)).Decode(out)
}

func Marshal(in any) (out []byte, err error) {
	buf := new(bytes.Buffer)

	err = NewEncoder(buf).Encode(in)

	return buf.Bytes(), nil
}
