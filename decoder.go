package uno

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"reflect"
	"slices"
	"strings"
)

type Decoder struct {
	r *rowReader

	TimeLayouts                   []string
	IgnoreFieldNameCase           bool
	SkipNonExistentFields         bool
	SkipNonExistentMetadataFields bool
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
			if s == endString+"\n" || s == endString {
				break
			} else {
				return b.String(), io.ErrUnexpectedEOF
			}
		} else if err != nil {
			return b.String(), err
		}

		if s == endString+"\n" {
			break
		}

		b.WriteString(s)
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
				return nil, fmt.Errorf("line %d: %w: %s", r.lineNumber, ErrInvalidMultilineStringBoundary, s)
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
		r:                             &rowReader{r: bufio.NewReader(r)},
		TimeLayouts:                   defaultDecoderTimeLayouts,
		SkipNonExistentMetadataFields: true,
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

func (dec *Decoder) decodeRaw(field reflect.Value, indent uint, nextRec *record, listType itemMarker) (unprocessedRow *record, err error) {
	for {
		var rec *record
		if nextRec != nil {
			rec = nextRec
			nextRec = nil
		} else {
			rec, err = dec.r.ReadRecord("")
			if err == io.EOF {
				err = nil
				break
			} else if err != nil {
				break
			}
		}

		if rec.Indent < indent {
			return rec, nil
		}

		if rec.Indent > indent /*&& field.Kind() != reflect.Map*/ {
			return nil, fmt.Errorf("line %d: %w, expected %d, got %d", rec.lineNumber, ErrInvalidIndentation, indent, rec.Indent)
		}

		if slices.Contains([]itemMarker{itemMarkerSliceString, itemMarkerSliceTyped}, rec.ItemMarker) &&
			slices.Contains([]itemMarker{itemMarkerSliceString, itemMarkerSliceTyped}, itemMarkerSliceTyped) {
		} else if rec.ItemMarker != listType {
			return rec, nil
		}

		if rec.Key != "" && rec.Value == "" && rec.Delimiter == delimiterString {
			// New field with types: struct, array, map, multiline string

			nextRec, err = dec.r.ReadRecord("")
			if err != nil {
				return nil, err // TODO: which kind of error can be?
			}

			if nextRec.Indent < indent {
				return nextRec, nil
			}

			if nextRec.multilineStringBoundary != "" {
				s, err := dec.r.ReadMultilineString(nextRec.multilineStringBoundary)
				if err != nil {
					return nil, fmt.Errorf("line %d: %w", rec.lineNumber, err)
				}

				field.SetMapIndex(reflect.ValueOf(rec.Key), reflect.ValueOf(s))
				nextRec = nil
				continue
			}

			switch nextRec.ItemMarker {
			case itemMarkerMap:
				v := reflect.ValueOf(&Raw{}).Elem()

				newIndent := indent
				if rec.ItemMarker == itemMarkerMap { // map in map
					newIndent += 1
				}

				nextRec, err = dec.decodeRaw(v, newIndent, nextRec, itemMarkerMap)
				if err != nil {
					return nil, err
				}
				field.SetMapIndex(reflect.ValueOf(rec.Key), v)
			case itemMarkerSliceString, itemMarkerSliceTyped:
				v := reflect.ValueOf(&[]string{}).Elem()

				nextRec, err = dec.decodeRaw(v, indent, nextRec, itemMarkerSliceString) // TODO: implement mixed type arrays
				if err != nil {
					return nil, err
				}
				field.SetMapIndex(reflect.ValueOf(rec.Key), v)
			default:
				v := reflect.ValueOf(Raw{})
				nextRec, err = dec.decodeRaw(v, indent+1, nextRec, itemMarkerNone)
				if err != nil {
					return nil, err
				}
				field.SetMapIndex(reflect.ValueOf(rec.Key), v)
			}
		} else if rec.Key != "" && rec.Value != "" {
			// New field with simple value

			field = createAndGetField(field)

			if field.Kind() == reflect.Map {
				if field.Type().Key().Kind() != reflect.String {
					return nil, fmt.Errorf("line %d: %w, got %s", rec.lineNumber, ErrMapKeyIsNotString, field.Type().Key().String())
				}
				if field.IsNil() {
					field.Set(reflect.MakeMap(field.Type()))
				}
				v, err := rec.decodeValue(reflect.TypeFor[string]())
				if err != nil {
					return nil, err
				}
				field.SetMapIndex(reflect.ValueOf(rec.Key), reflect.ValueOf(v))
			}
		} else if rec.Key == "" && rec.Value != "" {
			// element of slice

			v, err := rec.decodeValue(field.Type().Elem())
			if err != nil {
				return nil, err
			}

			if field.Type().Elem().Kind() == reflect.Pointer {
				v = getPointer(v)
			}

			newV := reflect.Append(field, reflect.ValueOf(v))
			field.Set(newV)
		} else if rec.Key == "" && rec.Value == "" && rec.ItemMarker == itemMarkerMap {
			if field.Kind() != reflect.Slice {
				return nil, fmt.Errorf("line %d: %w, expected %s, got %s", rec.lineNumber, ErrInvalidParentTypeOfAnonymousObject, reflect.Slice, field.Kind())
			}

			obj := reflect.New(field.Type().Elem())

			nextRec, err = dec.decodeRaw(obj, indent+1, nextRec, itemMarkerMap)
			if err != nil {
				return nil, err
			}
			field.Set(reflect.Append(field, obj.Elem()))
		} else {
			return nil, errors.New("unwknown conditions")
		}
	}
	return nil, err
}

func (dec *Decoder) decode(field reflect.Value, indent uint, nextRec *record) (unprocessedRow *record, err error) {
	for {
		var rec *record
		if nextRec != nil {
			rec = nextRec
			nextRec = nil
		} else {
			rec, err = dec.r.ReadRecord("")
			if err == io.EOF {
				err = nil
				break
			} else if err != nil {
				break
			}
		}

		if slices.Contains([]reflect.Kind{reflect.Array, reflect.Map, reflect.Slice}, field.Kind()) && rec.Indent > indent && rec.ItemMarker == itemMarkerMap {
			return nil, fmt.Errorf("line %d: %w: expected %d, got %d", rec.lineNumber, ErrInvalidIndentation, indent, rec.Indent)
		}

		if rec.Indent < indent || (rec.ItemMarker == itemMarkerNone && slices.Contains([]reflect.Kind{reflect.Array, reflect.Slice, reflect.Map}, field.Kind())) { // TODO: implement mixed type arrays
			return rec, nil
		}

		if rec.Key != "" && rec.Value == "" && rec.Delimiter == delimiterString {
			// New field with types: struct, array, map, multiline string

			field = createAndGetField(field)

			field := field.FieldByName(rec.Key)
			if !field.IsValid() {
				if dec.SkipNonExistentFields {
					nextRec, err = dec.SkipCurrentBlock(uint(indent))
					if err != nil {
						return nil, fmt.Errorf("line %d: %w", rec.lineNumber, err)
					}
					continue
				} else {
					return nil, fmt.Errorf("line %d: %w: \"%s\"", rec.lineNumber, ErrFieldIsNotExists, rec.Key)
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
				if field.Kind() != reflect.Map && nextRec.Key != "" {
					return nil, fmt.Errorf("line %d: %w: %s", nextRec.lineNumber, ErrTryingToDecodeMapToNonMap, field.Type().Name())
				}

				nextRec, err = dec.decode(field, indent, nextRec)
				if err != nil {
					return nil, err
				}
			case itemMarkerSliceString:
				if field.Kind() != reflect.Slice {
					return nil, fmt.Errorf("line %d: %w: %s", nextRec.lineNumber, ErrTryingToDecodeSliceToNonSlice, field.Type().Name())
				}
				if targetType(field.Type().Elem()).Kind() != reflect.String {
					return nil, fmt.Errorf("line %d: %w: expected %s, got %s", nextRec.lineNumber, ErrInvalidSliceType, reflect.String, targetType(field.Type().Elem()).Kind())
				}

				nextRec, err = dec.decode(field, indent, nextRec) // TODO: implement mixed type arrays
				if err != nil {
					return nil, err
				}
			case itemMarkerSliceTyped:
				if field.Kind() != reflect.Slice {
					return nil, fmt.Errorf("line %d: %w: %s", nextRec.lineNumber, ErrTryingToDecodeSliceToNonSlice, field.Type().Name())
				}
				if targetType(field.Type().Elem()).Kind() == reflect.String {
					return nil, fmt.Errorf("line %d: %w: expected non %s, got %s", nextRec.lineNumber, ErrInvalidSliceType, reflect.String, targetType(field.Type().Elem()).Kind())
				}

				nextRec, err = dec.decode(field, indent, nextRec) // TODO: implement mixed type arrays
				if err != nil {
					return nil, err
				}
			default:
				nextRec, err = dec.decode(field, indent+1, nextRec)
				if err != nil {
					return nil, err
				}
			}
		} else if rec.Key != "" && rec.Value != "" {
			// New field with simple value

			field = createAndGetField(field)

			if field.Kind() == reflect.Struct {
				found := false
				for field, value := range field.Fields() {
					key := rec.Key
					fieldName := getStructFieldName(field)
					if dec.IgnoreFieldNameCase {
						key = strings.ToLower(rec.Key)
						fieldName = strings.ToLower(fieldName)
					}
					if fieldName == key {
						if (rec.Directive == directiveMetadata) != getStructFieldMetadata(field) {
							continue
						}

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
				if !found && !(rec.Directive == directiveMetadata && dec.SkipNonExistentMetadataFields) {
					if dec.SkipNonExistentFields {
						nextRec, err = dec.SkipCurrentBlock(indent)
					} else {
						return nil, fmt.Errorf("line %d: %w: %s", rec.lineNumber, ErrFieldIsNotExists, rec.Key)
					}
				}
			} else if field.Kind() == reflect.Map {
				if field.Type().Key().Kind() != reflect.String {
					return nil, fmt.Errorf("line %d: %w, got %s", rec.lineNumber, ErrMapKeyIsNotString, field.Type().Key().String())
				}
				if field.IsNil() {
					field.Set(reflect.MakeMap(field.Type()))
				}
				v, err := rec.decodeValue(field.Type().Elem())
				if err != nil {
					return nil, err
				}
				field.SetMapIndex(reflect.ValueOf(rec.Key), reflect.ValueOf(v))
			}
		} else if rec.Key == "" && rec.Value != "" {
			v, err := rec.decodeValue(field.Type().Elem())
			if err != nil {
				return nil, err
			}

			if field.Type().Elem().Kind() == reflect.Pointer {
				v = getPointer(v)
			}
			field.Set(reflect.Append(field, reflect.ValueOf(v)))
		} else if rec.Key == "" && rec.Value == "" && rec.ItemMarker == itemMarkerMap {
			if field.Kind() != reflect.Slice {
				return nil, fmt.Errorf("line %d: %w, expected %s, got %s", rec.lineNumber, ErrInvalidParentTypeOfAnonymousObject, reflect.Slice, field.Kind())
			}

			obj := reflect.New(field.Type().Elem())

			nextRec, err = dec.decode(obj, indent+1, nextRec)
			if err != nil {
				return nil, err
			}
			field.Set(reflect.Append(field, obj.Elem()))
		} else {
			return nil, errors.New("unwknown conditions")
		}
	}
	return nil, err
}

func (dec *Decoder) Decode(out any) (err error) {
	value := reflect.ValueOf(out)
	for value.Kind() == reflect.Pointer {
		value = value.Elem()
	}

	switch value.Kind() {
	case reflect.Struct:
		_, err = dec.decode(value, 0, nil)
	case reflect.Map:
		if value.Type() == reflect.TypeFor[Raw]() {
			_, err = dec.decodeRaw(value, 0, nil, itemMarkerNone)
			return err
		}
		fallthrough
	default:
		err = fmt.Errorf("%w: expected struct or uno.Raw, got %s", ErrInvalidUnmarshalTargetType, value.Type())
	}

	return err
}
