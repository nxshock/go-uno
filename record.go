package uno

import (
	"fmt"
	"io"
	"strings"
)

type record struct {
	Indent     int
	Directive  directive
	ItemMarker itemMarker
	Key        string
	Delimiter  delimiter
	Value      string
	Comment    string
}

func parseRecord(s string, comment string) (*record, error) {
	r := strings.NewReader(s)

	// indent
	indent := 0
	for {
		ch, _, err := r.ReadRune()
		if err != nil {
			return nil, fmt.Errorf("read ident: %v", err)
		}
		if ch == '\t' {
			indent += 1
		} else {
			err = r.UnreadRune()
			if err != nil {
				return nil, fmt.Errorf("unread ident: %v", err)
			}
			break
		}
	}

	// dir
	dir := directiveNone
	ch, _, err := r.ReadRune()
	if err != nil {
		return nil, fmt.Errorf("read directive: %v", err)
	}
	if ch != ' ' {
		switch directive(ch) {
		case directiveMetadata, directiveAssembly, directiveValidation:
			dir = directive(ch)
		default:
			err = r.UnreadRune()
			if err != nil {
				return nil, fmt.Errorf("unread directive: %v", err)
			}
		}
	}

	// item marker
	var keyBuilder strings.Builder
	itmMarker := itemMarkerNone
	ch, _, err = r.ReadRune()
	if err != nil {
		return nil, fmt.Errorf("read item marker: %v", err)
	}
	switch itemMarker(ch) {
	case itemMarkerString, itemMarkerTyped, itemMarkerStruct:
		itmMarker = itemMarker(ch)
		ch, _, err = r.ReadRune()
		if err != nil {
			return nil, fmt.Errorf("read rune after item marker: %v", err)
		}
		if ch != ' ' {
			keyBuilder.WriteRune(rune(itmMarker))
			keyBuilder.WriteRune(ch)
			itmMarker = itemMarkerNone
		}
	default:
		err = r.UnreadRune()
		if err != nil {
			return nil, fmt.Errorf("unread item marker: %v", err)
		}
	}

	// key and delim
	delim := delimiter(' ')
	if itmMarker == itemMarkerNone {
		for {
			brk := false
			ch, _, err = r.ReadRune()
			if err != nil {
				return nil, fmt.Errorf("read delimiter: %v", err)
			}
			switch delimiter(ch) {
			case delimiterString, delimiterTyped:
				delim = delimiter(ch)
				ch, _, err := r.ReadRune()
				if err == io.EOF {
					brk = true
				} else if err != nil {
					return nil, fmt.Errorf("read after delimiter: %v", err)
				}
				if !brk && ch != ' ' {
					return nil, fmt.Errorf("expected space after delimiter, got %s", string(ch))
				}
				brk = true
			default:
				keyBuilder.WriteString(string(ch))
			}

			if brk {
				break
			}
		}
	}
	key := keyBuilder.String()
	if key == "''" {
		key = ""
	}

	// value
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	value := strings.TrimSpace(string(b))
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
		Comment:    comment}

	return rec, nil
}
