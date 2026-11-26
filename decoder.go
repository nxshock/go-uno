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
	scanner        *bufio.Scanner
	lastLineNumber uint

	TimeLayouts                   []string
	IgnoreFieldNameCase           bool
	SkipNonExistentFields         bool
	SkipNonExistentMetadataFields bool
}

func NewDecoder(r io.Reader) *Decoder {
	scanner := bufio.NewScanner(r)
	scanner.Split(tokenSplitFunc)
	return &Decoder{
		scanner:                       scanner,
		lastLineNumber:                0,
		TimeLayouts:                   defaultDecoderTimeLayouts,
		SkipNonExistentMetadataFields: true}
}

func (d *Decoder) nextToken() (string, error) {
	var err error
	if !d.scanner.Scan() && d.scanner.Err() == nil {
		err = io.EOF
	}
	return d.scanner.Text(), err
}

func (d *Decoder) scanUntil(suffix string, trimSpace bool) (result string, err error) {
	sb := new(strings.Builder)

	for d.scanner.Scan() {
		if err = d.scanner.Err(); err != nil {
			return result, err
		}

		sb.WriteString(d.scanner.Text())

		if strings.HasSuffix(sb.String(), suffix) {
			break
		}
	}

	result = sb.String()

	if !strings.HasSuffix(result, suffix) && suffix != "\n" {
		return result, io.ErrUnexpectedEOF
	}

	result = strings.TrimSuffix(result, suffix)
	if trimSpace {
		result = strings.TrimSpace(result)
	}

	return result, nil
}

func (d *Decoder) skipCurrentBlock(indent uint, listType itemMarker) (rec *record, err error) {
	for {
		rec, err = d.readRecord("")
		if err == io.EOF {
			return nil, nil
		} else if err != nil {
			return nil, err
		}

		if rec.Indent == indent && (rec.Key != "" || listType != rec.ItemMarker) {
			break
		}
	}

	return rec, nil
}

func (d *Decoder) readRecord(comment string) (rec *record, err error) {
	for {
		var token string
		token, err = d.nextToken()
		if err != nil {
			break
		}
		if token == "\n" {
			d.lastLineNumber++
			continue
		}
		if token == "" {
			// no more tokens
			return rec, io.EOF
		}

		rec = &record{
			Comment:    comment,
			lineNumber: d.lastLineNumber + 1}

		if validMultilineBoundary(token) {
			rec.multilineStringBoundary = token[:len(token)-1]
			s, err := d.scanUntil("\n"+token, false)
			if err != nil {
				return rec, err
			}
			rec.Value = s
			break
		}

		switch token {
		case "// ":
			d.lastLineNumber++
			commentLine, err := d.scanUntil("\n", true)
			if err != nil {
				return nil, err
			}
			if comment == "" {
				return d.readRecord(commentLine)
			} else {
				return d.readRecord(comment + "\n" + commentLine)
			}
		case "/'\n":
			commentMultiLine, err := d.scanUntil("\n'/\n", false)
			if err != nil {
				return nil, err
			}
			d.lastLineNumber += uint(strings.Count(commentMultiLine, "\n") + 3)
			continue
		case "# ":
			rec.Directive = directiveMetadata
			token, err = d.nextToken()
			if err != nil {
				if err == io.EOF {
					return rec, io.ErrUnexpectedEOF
				} else {
					return rec, err
				}
			}
		case ".", ".\n":
			rec.ItemMarker = itemMarkerMap
			return rec, nil
		}

		rec.Indent = countIndentLevel(token)
		token = token[rec.Indent:]

	loop:
		for {
			suffix := ""
			if len(token) >= 2 {
				suffix = token[len(token)-2:]
			}
			switch suffix {
			case "- ":
				rec.ItemMarker = itemMarkerSliceString
				rec.Delimiter = delimiterString
				val, err := d.scanUntil("\n", true)
				if err != nil {
					return nil, err
				}
				rec.Key = strings.TrimSuffix(token, suffix)
				rec.Value = val
				break loop
			case "= ":
				rec.ItemMarker = itemMarkerSliceTyped
				rec.Delimiter = delimiterTyped
				val, err := d.scanUntil("\n", true)
				if err != nil {
					return rec, err
				}
				rec.Key = strings.TrimSuffix(token, suffix)
				rec.Value = val
				break loop
			case ". ":
				rec.ItemMarker = itemMarkerMap
				rec.Key, err = d.nextToken()
				if err == io.EOF {
					return rec, io.ErrUnexpectedEOF
				} else if err != nil {
					return rec, err
				}
			default:
				rec.ItemMarker = itemMarkerNone
				rec.Key += token
			}

			if strings.HasSuffix(rec.Key, ": ") || strings.HasSuffix(rec.Key, "= ") {
				switch rec.Key[len(rec.Key)-2:] {
				case ": ":
					rec.Delimiter = delimiterString
				case "= ":
					rec.Delimiter = delimiterTyped
				}
				rec.Key = rec.Key[:len(rec.Key)-2]
				if rec.Key == "''" {
					rec.Key = ""
				}

				rec.Value, err = d.scanUntil("\n", true)
				if err != nil {
					return rec, err
				}
				break
			} else if strings.HasSuffix(rec.Key, ":\n") {
				rec.Key = strings.TrimSuffix(rec.Key, ":\n")
				rec.Delimiter = delimiterString
				break
			} else if strings.Contains(rec.Key, "\n") {
				if strings.HasPrefix(rec.Key, "'") {
					return nil, ErrInvalidMultilineStringBoundary
				}
				if strings.HasPrefix(rec.Key, "/'") {
					return nil, ErrInvalidMultilineCommentBoundary
				}
			}

			token, err = d.nextToken()
			if err != nil {
				return rec, err
			}
		}

		if strings.HasPrefix(rec.Key, " ") || strings.HasSuffix(rec.Key, " ") {
			return rec, ErrSpaceInKey
		}

		if strings.HasPrefix(rec.Value, "'") && strings.HasSuffix(rec.Value, "'") && rec.Value != "'" {
			rec.Value = strings.TrimPrefix(rec.Value, "'")
			rec.Value = strings.TrimSuffix(rec.Value, "'")
		}

		break
	}

	d.lastLineNumber += rec.size()

	return rec, err
}

func (dec *Decoder) decode(field reflect.Value, indent uint, nextRec *record) (unprocessedRow *record, err error) {
	for {
		var rec *record
		if nextRec != nil {
			rec = nextRec
			nextRec = nil
		} else {
			rec, err = dec.readRecord("")
			if err == io.EOF {
				err = nil
				break
			} else if err != nil {
				break
			}
		}

		if slices.Contains([]reflect.Kind{reflect.Array, reflect.Map, reflect.Slice}, field.Kind()) && rec.Indent > indent && rec.ItemMarker == itemMarkerMap {
			return nil, newDecodeError(rec.lineNumber, ErrInvalidIndentation, rec.Key, fmt.Sprint(indent), fmt.Sprint(rec.Indent))
		}

		if rec.Indent < indent || (rec.ItemMarker == itemMarkerNone && slices.Contains([]reflect.Kind{reflect.Array, reflect.Slice, reflect.Map}, field.Kind())) { // TODO: implement mixed type arrays
			return rec, nil
		}

		if rec.Key != "" && rec.Value == "" && rec.Delimiter == delimiterString {
			// New field with types: struct, array, map, multiline string

			field = createAndGetField(field)

			if field.Kind() == reflect.Map {
				v := reflect.New(field.Type().Elem()).Elem()

				nextRec, err = dec.decode(v, indent+1, nil)
				if err != nil {
					return nil, err
				}

				field.SetMapIndex(reflect.ValueOf(rec.Key), v)
				continue
			}

			if sf, ok := field.Type().FieldByName(rec.Key); ok {
				if getStructFieldName(sf) == "-" {
					nextRec, err = dec.readRecord("")
					if err != nil {
						return nil, err
					}

					nextRec, err = dec.skipCurrentBlock(nextRec.Indent, nextRec.ItemMarker)
					if err != nil {
						return nil, newDecodeError(rec.lineNumber, err, rec.Key, "", "") // TODO: add proper error.Error() format
					}
					continue
				}
			}
			field := field.FieldByName(rec.Key)
			if !field.IsValid() {
				if dec.SkipNonExistentFields {
					nextRec, err = dec.readRecord("")
					if err != nil {
						return nil, err
					}

					nextRec, err = dec.skipCurrentBlock(uint(indent), nextRec.ItemMarker)
					if err != nil {
						return nil, newDecodeError(rec.lineNumber, err, rec.Key, "", "") // TODO: add proper error.Error() format
					}
					continue
				} else {
					return nil, newDecodeError(rec.lineNumber, ErrFieldIsNotExists, rec.Key, "", "")
				}
			}

			nextRec, err = dec.readRecord("")
			if err != nil {
				return nil, newDecodeError(rec.lineNumber, err, rec.Key, "", "") // TODO: add proper error.Error() format
			}

			if nextRec.multilineStringBoundary != "" {
				field.Set(reflect.ValueOf(nextRec.Value))
				nextRec = nil
				continue
			}

			switch nextRec.ItemMarker {
			case itemMarkerMap:
				if field.Kind() != reflect.Map && nextRec.Key != "" {
					return nil, newDecodeError(nextRec.lineNumber, ErrTryingToDecodeMapToNonMap, nextRec.Key, reflect.Map.String(), field.Kind().String())
				}

				nextRec, err = dec.decode(field, indent, nextRec)
				if err != nil {
					return nil, err
				}
			case itemMarkerSliceString:
				if field.Kind() != reflect.Slice {
					return nil, newDecodeError(nextRec.lineNumber, ErrTryingToDecodeSliceToNonSlice, rec.Key, reflect.Slice.String(), field.Kind().String())
				}
				if targetType(field.Type().Elem()).Kind() != reflect.String {
					return nil, newDecodeError(nextRec.lineNumber, ErrInvalidSliceType, rec.Key, reflect.String.String(), targetType(field.Type().Elem()).Kind().String())
				}

				nextRec, err = dec.decode(field, indent, nextRec) // TODO: implement mixed type arrays
				if err != nil {
					return nil, err
				}
			case itemMarkerSliceTyped:
				if field.Kind() != reflect.Slice {
					return nil, newDecodeError(nextRec.lineNumber, ErrTryingToDecodeSliceToNonSlice, rec.Key, reflect.Slice.String(), field.Kind().String())
				}
				if targetType(field.Type().Elem()).Kind() == reflect.String {
					return nil, newDecodeError(nextRec.lineNumber, ErrInvalidSliceType, rec.Key, reflect.String.String(), targetType(field.Type().Elem()).Kind().String())
				}

				nextRec, err = dec.decode(field, indent, nextRec) // TODO: implement mixed type arrays
				if err != nil {
					return nil, err
				}
			default:
				field = createAndGetField(field)
				for f, v := range field.Fields() {
					if getStructFieldComment(f) {
						v.Set(reflect.ValueOf(rec.Comment))
						break
					}
				}

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
							return nil, newDecodeError(rec.lineNumber, ErrFieldIsNotExported, fieldName, "", "")
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
						nextRec, err = dec.skipCurrentBlock(indent, itemMarkerNone)
					} else {
						return nil, newDecodeError(rec.lineNumber, ErrFieldIsNotExists, rec.Key, "", "")
					}
				}
			} else if field.Kind() == reflect.Map {
				if field.Type().Key().Kind() != reflect.String {
					return nil, newDecodeError(rec.lineNumber, ErrMapKeyIsNotString, rec.Key, reflect.String.String(), field.Type().Key().Kind().String())
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
				return nil, newDecodeError(rec.lineNumber, ErrInvalidParentTypeOfAnonymousObject, "", reflect.Slice.String(), field.Kind().String()) // TODO: add parent field name
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

func (dec *Decoder) decodeRaw(field reflect.Value, indent uint, nextRec *record, listType itemMarker, sliceParent bool) (unprocessedRow *record, err error) {
	for {
		var rec *record
		if nextRec != nil {
			rec = nextRec
			nextRec = nil
		} else {
			rec, err = dec.readRecord("")
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

		if rec.Indent > indent {
			return nil, newDecodeError(rec.lineNumber, ErrInvalidIndentation, "", fmt.Sprint(indent), fmt.Sprint(rec.Indent)) // TODO: add field name
		}

		if slices.Contains([]itemMarker{itemMarkerSliceString, itemMarkerSliceTyped}, rec.ItemMarker) &&
			slices.Contains([]itemMarker{itemMarkerSliceString, itemMarkerSliceTyped}, itemMarkerSliceTyped) {
		} else if rec.ItemMarker != listType && field.Kind() == reflect.Pointer && field.Elem().Kind() != reflect.Map {
			return rec, nil
		} else if rec.ItemMarker == itemMarkerNone && slices.Contains([]itemMarker{itemMarkerSliceString, itemMarkerSliceTyped}, listType) {
			return rec, nil
		} else if rec.ItemMarker == itemMarkerNone && listType == itemMarkerMap && !sliceParent {
			return rec, nil
		}

		if rec.Key != "" && rec.Value == "" && rec.Delimiter == delimiterString {
			// New field with types: struct, array, map, multiline string

			nextRec, err = dec.readRecord("")
			if err != nil {
				return nil, err // TODO: which kind of error can be?
			}

			if nextRec.Indent < indent {
				return nextRec, nil
			}

			if nextRec.multilineStringBoundary != "" {
				s, err := dec.scanUntil(nextRec.multilineStringBoundary, false)
				if err != nil {
					return nil, newDecodeError(rec.lineNumber, err, nextRec.Key, "", "")
				}

				field.SetMapIndex(reflect.ValueOf(rec.Key), reflect.ValueOf(s))
				nextRec = nil
				continue
			}

			switch nextRec.ItemMarker {
			case itemMarkerMap:
				var v reflect.Value
				if nextRec.Key == "" {
					v = reflect.ValueOf(&[]Raw{}).Elem() // anonymous objects
				} else {
					v = reflect.ValueOf(&Raw{}).Elem() // named objects
				}

				newIndent := indent
				if rec.ItemMarker == itemMarkerMap { // map in map
					newIndent += 1
				}

				nextRec, err = dec.decodeRaw(v, newIndent, nextRec, itemMarkerMap, false)
				if err != nil {
					return nil, err
				}
				switch field.Kind() {
				case reflect.Map:
					field.SetMapIndex(reflect.ValueOf(rec.Key), v)
				case reflect.Slice:
					field.Set(reflect.Append(field, v))
				}

			case itemMarkerSliceString, itemMarkerSliceTyped:
				v := reflect.ValueOf(&[]string{}).Elem()

				nextRec, err = dec.decodeRaw(v, indent, nextRec, itemMarkerSliceString, false) // TODO: implement mixed type arrays
				if err != nil {
					return nil, err
				}
				field.SetMapIndex(reflect.ValueOf(rec.Key), v)
			default:
				v := reflect.ValueOf(Raw{})
				nextRec, err = dec.decodeRaw(v, indent+1, nextRec, itemMarkerNone, sliceParent)
				if err != nil {
					return nil, err
				}
				switch field.Kind() {
				case reflect.Slice:
					field.Set(reflect.Append(field, v))
				case reflect.Map:
					field.SetMapIndex(reflect.ValueOf(rec.Key), v)
				default:
					return nil, fmt.Errorf("unexpected field type: expected slice or map, got %s", field.Kind())
				}
			}
		} else if rec.Key != "" && rec.Value != "" {
			// New field with simple value

			field = createAndGetField(field)

			if field.Kind() == reflect.Map {
				if field.Type().Key().Kind() != reflect.String {
					return nil, newDecodeError(rec.lineNumber, ErrMapKeyIsNotString, rec.Key, reflect.TypeFor[string]().String(), field.Type().Key().String())
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
				return nil, newDecodeError(rec.lineNumber, ErrInvalidParentTypeOfAnonymousObject, "", reflect.Slice.String(), field.Kind().String()) // TODO: get parent field name
			}

			obj := reflect.New(field.Type().Elem())

			nextRec, err = dec.decodeRaw(obj, indent+1, nextRec, itemMarkerMap, field.Kind() == reflect.Slice)
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
			_, err = dec.decodeRaw(value, 0, nil, itemMarkerNone, false)
			return err
		}
	default:
		err = newDecodeError(0, ErrInvalidUnmarshalTargetType, "", "", value.Type().String())
	}

	return err
}
