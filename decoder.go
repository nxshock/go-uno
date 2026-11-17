package uno

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
)

type Decoder struct {
	r *rowReader

	TimeLayouts           []string
	IgnoreFieldNameCase   bool
	SkipNonExistentFields bool
}

type rowReader struct {
	r          *bufio.Reader
	lineNumber uint
}

func (r *rowReader) ReadMultilineString(endString string) (string, error) {
	b := new(strings.Builder)
	for {
		s, err := r.r.ReadString('\n')
		if err == io.EOF {
			if strings.HasPrefix(s, endString) {
				break
			} else {
				return b.String(), io.ErrUnexpectedEOF
			}
		} else if err != nil {
			return b.String(), err
		}

		if strings.HasPrefix(s, endString) {
			break
		}

		_, err = b.WriteString(s)
		if err != nil {
			return "", err
		}
	}

	s := b.String()

	return s[:len(s)-1], nil
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

		if strings.HasPrefix(s, "'") {
			// start/end of multiline string
			l := 0
			for _, v := range s {
				if v == '\'' {
					l += 1
				} else {
					break
				}
			}
			if len(s) != l {
				return nil, fmt.Errorf("line %d: %w: %s", r.lineNumber, ErrWrongMultilineStringBoundary, s)
			}
			return &record{multilineStringBoundary: s}, nil
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

func (dec *Decoder) SkipCurrentBlock(indent uint) (rec *record, err error) {
	for {
		rec, err = dec.r.ReadRecord("")
		if err == io.EOF {
			return nil, nil
		} else if err != nil {
			return nil, err
		}

		if rec.Indent == indent && rec.Key != "" {
			break
		}
	}

	return rec, nil
}

func (dec *Decoder) decode(value reflect.Value, indent uint, nextRec *record) (unprocessedRow *record, err error) {
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

		if rec.Indent < indent ||
			(rec.ItemMarker == itemMarkerNone && value.Kind() == reflect.Slice) || // TODO: implement mixed type arrays
			(rec.ItemMarker == itemMarkerNone && value.Kind() == reflect.Map) ||
			(rec.ItemMarker == itemMarkerNone && value.Kind() == reflect.Array) {

			return rec, nil
		}

		if rec.Key != "" && rec.Value == "" && rec.Delimiter == delimiterString {
			// New field with types: struct, array, map, multiline string
			field := value.FieldByName(rec.Key)
			if !field.IsValid() {
				if dec.SkipNonExistentFields {
					nextRec, err = dec.SkipCurrentBlock(uint(indent))
					if err != nil {
						return nil, err
					}
					continue
				} else {
					return nil, fmt.Errorf("line %d: %w: \"%s\"", rec.lineNumber, ErrFieldIsNotExists, rec.Key) // TODO: add option to skip fields
				}
			}

			nextRec, err = dec.r.ReadRecord("")
			if err != nil {
				return nil, err // TODO: which kind of error can be?
			}

			if nextRec.multilineStringBoundary != "" {
				s, err := dec.r.ReadMultilineString(nextRec.multilineStringBoundary)
				if err != nil {
					return nil, fmt.Errorf("line %d: %w", rec.lineNumber, err)
				}

				field.Set(reflect.ValueOf(s))
				nextRec = nil
				continue
			}

			switch nextRec.ItemMarker {
			case itemMarkerMap:
				nextRec, err = dec.decode(field, indent, nextRec)
				if err != nil {
					return nil, err
				}
			case itemMarkerArrayString:
				nextRec, err = dec.decode(field, indent, nextRec) // TODO: implement mixed type arrays
				if err != nil {
					return nil, err
				}
				continue
			case itemMarkerArrayTyped:
				nextRec, err = dec.decode(field, indent, nextRec) // TODO: implement mixed type arrays
				if err != nil {
					return nil, err
				}
				continue
			default:
				nextRec, err = dec.decode(field, indent+1, nextRec)
				if err != nil {
					return nil, err
				}
			}
		} else if rec.Key != "" && rec.Value != "" {
			// New field with simple value

			if value.Kind() == reflect.Struct {
				found := false
				for field, value := range value.Fields() {
					key := rec.Key
					fieldName := getStructFieldName(field)
					if dec.IgnoreFieldNameCase {
						key = strings.ToLower(rec.Key)
						fieldName = strings.ToLower(fieldName)
					}
					if fieldName == key {
						if !field.IsExported() {
							return nil, fmt.Errorf("line %d: %w: \"%s\"", rec.lineNumber, ErrFieldIsNotExported, fieldName)
						}

						v, err := rec.decodeValue(field.Type)
						if err != nil {
							return nil, err
						}
						if v != nil {
							setValue(value, v)
						}
						found = true
						break
					}
				}
				if !found {
					if dec.SkipNonExistentFields {
						nextRec, err = dec.SkipCurrentBlock(indent)
					} else {
						return nil, fmt.Errorf("line %d: %w: %s", rec.lineNumber, ErrFieldIsNotExists, rec.Key)
					}
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
		} else if rec.Key == "" && rec.Value != "" {
			v, err := rec.decodeValue(value.Type().Elem())
			if err != nil {
				return nil, err
			}
			newV := reflect.Append(value, reflect.ValueOf(v))
			value.Set(newV)
		} else {
			return nil, errors.New("unwknown conditions")
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

	_, err = dec.decode(e, 0, nil)
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
