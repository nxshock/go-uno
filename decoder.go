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
	r *bufio.Reader
}

func (r *rowReader) ReadRow(comment string) (rec *record, err error) {
	var s string
	for {
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

		return parseRecord(s, comment)
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
		var row *record
		if nextRec != nil {
			row = nextRec
			nextRec = nil
		} else {
			row, err = dec.r.ReadRow("")
			if err == io.EOF {
				return nil, nil
			} else if err != nil {
				break
			}
		}

		if row.Indent < ident || parentItemMarker != row.ItemMarker { // TODO: может поменяться, если массив разных типов
			return row, nil
		}

		if row.Key != "" && row.Value == "" && row.Delimiter == delimiterString {
			// New field with types: struct, array, map

			field := value.FieldByName(row.Key)
			if !field.IsValid() {
				return nil, fmt.Errorf("field %s is not exists in destination", row.Key)
			}

			nextRec, err = dec.r.ReadRow("")
			if err != nil {
				return nil, err // TODO: whick kind of error can be?
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
		} else if row.Key != "" && row.Value != "" {
			// New field with simple value

			if value.Kind() == reflect.Struct {
				field := value.FieldByName(row.Key)
				if !field.IsValid() {
					return nil, fmt.Errorf("field %s is not exists in destination", row.Key)
				}
				if !field.CanSet() {
					return nil, fmt.Errorf("field %s is not settable", field.Type().Name())
				}

				v, err := row.decodeValue(field.Type())
				if err != nil {
					return nil, err
				}
				if v != nil {
					setValue(field, v)
				}
			} else if value.Kind() == reflect.Map {
				if value.Type().Key().Kind() != reflect.String {
					return nil, fmt.Errorf("only string keys supported in maps, got %s", value.Type().Key().String())
				}
				if value.IsNil() {
					value.Set(reflect.MakeMap(value.Type()))
				}
				v, err := row.decodeValue(value.Type().Elem())
				if err != nil {
					return nil, err
				}
				value.SetMapIndex(reflect.ValueOf(row.Key), reflect.ValueOf(v))
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
