package uno

import (
	"bufio"
	"fmt"
	"io"
	"reflect"
	"strings"
)

type Decoder struct {
	r *rowReader

	TimeLayouts []string
}

type rowReader struct {
	r          *bufio.Reader
	lineNumber uint
}

func (r *rowReader) ReadRecord(comment string) (rec *record, err error) {
	var s string
	for {
		r.lineNumber += 1

		s, err = r.r.ReadString('\n')
		if err == io.EOF {
			if s == "" {
				return nil, io.EOF
			}
		} else if err != nil {
			return nil, err
		}

		if len(s) > 0 && s[len(s)-1] == '\n' {
			s = s[:len(s)-1]
		}

		if strings.HasPrefix(strings.TrimSpace(s), "//") {
			// skip comment row
			if comment != "" {
				comment = comment + "\n" + strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), "//"))
			}
			continue
		}

		// skip empty rows
		if strings.TrimSpace(s) == "" {
			continue
		}

		return parseRecord(s, comment, r.lineNumber)
	}
}

func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{
		r:           &rowReader{r: bufio.NewReader(r)},
		TimeLayouts: defaultDecoderTimeLayouts,
	}
}

func (dec *Decoder) decode(value reflect.Value, ident int, parentItemMarker itemMarker, nextRec *record) (unprocessedRow *record, err error) {
	for {
		var rec *record
		if nextRec != nil {
			rec = nextRec
			nextRec = nil
		} else {
			rec, err = dec.r.ReadRecord("")
			if err == io.EOF {
				return nil, nil
			} else if err != nil {
				break
			}
		}

		if rec.Indent < ident || parentItemMarker != rec.ItemMarker { // TODO: implement mixed type arrays
			return rec, nil
		}

		if rec.Key != "" && rec.Value == "" && rec.Delimiter == delimiterString {
			// New field with types: struct, array, map

			field := value.FieldByName(rec.Key)
			if !field.IsValid() {
				return nil, fmt.Errorf("line %d: %w: \"%s\"", rec.lineNumber, ErrFieldIsNotExists, rec.Key) // TODO: add option to skip fields
			}

			nextRec, err = dec.r.ReadRecord("")
			if err != nil {
				return nil, err // TODO: which kind of error can be?
			}
			switch nextRec.ItemMarker {
			case itemMarkerMap:
				nextRec, err = dec.decode(field, ident, itemMarkerMap, nextRec)
				if err != nil {
					return nil, err
				}
			case itemMarkerArrayString, itemMarkerArrayTyped:
				nextRec, err = dec.decode(field, ident, itemMarkerArrayString, nextRec) // TODO: implement mixed type arrays
				if err != nil {
					return nil, err
				}
				continue
			default:
				nextRec, err = dec.decode(field, ident+1, itemMarkerNone, nextRec)
				if err != nil {
					return nil, err
				}
			}
		} else if rec.Key != "" && rec.Value != "" {
			// New field with simple value

			if value.Kind() == reflect.Struct {
				field := value.FieldByName(rec.Key)
				if !field.IsValid() {
					return nil, fmt.Errorf("line %d: %w: \"%s\"", rec.lineNumber, ErrFieldIsNotExists, rec.Key) // TODO: add option to skip fields
				}
				if !field.CanSet() {
					return nil, fmt.Errorf("line %d: %w: \"%s\"", rec.lineNumber, ErrFieldIsNotSettable, field.Type().Name())
				}

				v, err := rec.decodeValue(field.Type())
				if err != nil {
					return nil, err
				}
				if v != nil {
					setValue(field, v)
				}
			} else if value.Kind() == reflect.Map {
				if value.Type().Key().Kind() != reflect.String {
					return nil, fmt.Errorf("line %d: %w, got %s", rec.lineNumber, ErrMapKeyIsNotString, value.Type().Key().String())
				}
				if value.IsNil() {
					value.Set(reflect.MakeMap(value.Type()))
				}
				v, err := rec.decodeValue(value.Type().Elem())
				if err != nil {
					return nil, err
				}
				value.SetMapIndex(reflect.ValueOf(rec.Key), reflect.ValueOf(v))
			}
		}
	}
	return nil, err
}

func (dec *Decoder) Decode(out any) (err error) {
	e := reflect.ValueOf(out)
	for e.Kind() == reflect.Pointer {
		e = e.Elem()
	}

	if e.Kind() != reflect.Struct {
		return fmt.Errorf("can unmarshal only to %s, got %s", reflect.Struct, e.Kind().String())
	}

	_, err = dec.decode(e, 0, itemMarkerNone, nil)
	return err
}

func setValue(field reflect.Value, value any) {
	if field.Kind() == reflect.Pointer {
		v := reflect.New(field.Type().Elem())
		v.Elem().Set(reflect.ValueOf(value))
		field.Set(v)
	} else {
		if reflect.ValueOf(value).Kind() == reflect.Pointer {
			field.Set(reflect.ValueOf(value).Elem())
		} else {
			field.Set(reflect.ValueOf(value))
		}
	}
}
