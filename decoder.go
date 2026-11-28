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

	lastRecord record
	nextRecord record

	useNextRecord bool

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

func (d *Decoder) skipCurrentBlock(indent uint, listType itemMarker, rec *record) (err error) {
	for {
		err = d.readRecord("", rec)
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}

		if rec.Indent == indent && (rec.Key != "" || listType != rec.ItemMarker) {
			break
		}
	}

	return nil
}

func (d *Decoder) readBlock(startRec *record, rec *record) (s string, err error) {
	sb := new(strings.Builder)

	firstRecProcessed := false

	for {
		if firstRecProcessed {
			err = d.readRecord("", rec)
		} else {
			rec = startRec
			firstRecProcessed = true
		}
		if err == io.EOF {
			err = nil
			break
		} else if err != nil {
			break
		}

		if rec.Indent == startRec.Indent && (rec.Key != "" || startRec.ItemMarker != rec.ItemMarker) {
			break
		}

		sb.WriteString(rec.String())
	}

	return sb.String(), err
}

func (d *Decoder) readRecord(comment string, rec *record) (err error) {
	var token string

	for {
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
			return io.EOF
		}

		rec.Indent = 0
		rec.Directive = directiveNone
		rec.ItemMarker = itemMarkerNone
		rec.Key = ""
		rec.Delimiter = delimiterNone
		rec.Value = ""
		rec.Comment = comment
		rec.lineNumber = d.lastLineNumber + 1
		rec.multilineStringBoundary = ""

		if validMultilineBoundary(token) {
			rec.multilineStringBoundary = token[:len(token)-1]
			s, err := d.scanUntil("\n"+token, false)
			if err != nil {
				return err
			}
			rec.Value = s
			break
		}

		switch token {
		case "// ":
			d.lastLineNumber++
			commentLine, err := d.scanUntil("\n", true)
			if err != nil {
				return err
			}
			if comment == "" {
				return d.readRecord(commentLine, rec)
			} else {
				return d.readRecord(comment+"\n"+commentLine, rec)
			}
		case "/'\n":
			commentMultiLine, err := d.scanUntil("\n'/\n", false)
			if err != nil {
				return err
			}
			d.lastLineNumber += uint(strings.Count(commentMultiLine, "\n") + 3)
			continue
		case "# ":
			rec.Directive = directiveMetadata
			token, err = d.nextToken()
			if err != nil {
				if err == io.EOF {
					return io.ErrUnexpectedEOF
				} else {
					return err
				}
			}
		case ".", ".\n":
			rec.ItemMarker = itemMarkerMap
			return nil
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
				rec.Delimiter = delimiterString
				val, err := d.scanUntil("\n", true)
				if err != nil {
					return err
				}
				rec.Key = strings.TrimSuffix(token, suffix)
				if rec.Key == "" {
					rec.ItemMarker = itemMarkerSliceString
				}
				rec.Value = val
				break loop
			case "= ":
				// rec.ItemMarker = itemMarkerSliceTyped
				rec.Delimiter = delimiterTyped
				val, err := d.scanUntil("\n", true)
				if err != nil {
					return err
				}
				rec.Key = strings.TrimSuffix(token, suffix)
				if rec.Key == "" {
					rec.ItemMarker = itemMarkerSliceTyped
				}
				rec.Value = val
				break loop
			case ". ":
				rec.ItemMarker = itemMarkerMap
				rec.Key, err = d.nextToken()
				if err == io.EOF {
					return io.ErrUnexpectedEOF
				} else if err != nil {
					return err
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
					return err
				}
				break
			} else if strings.HasSuffix(rec.Key, ":\n") {
				rec.Key = strings.TrimSuffix(rec.Key, ":\n")
				rec.Delimiter = delimiterString
				break
			} else if strings.Contains(rec.Key, "\n") {
				if strings.HasPrefix(rec.Key, "'") {
					return ErrInvalidMultilineStringBoundary
				}
				if strings.HasPrefix(rec.Key, "/'") {
					return ErrInvalidMultilineCommentBoundary
				}
			}

			token, err = d.nextToken()
			if err == io.EOF && rec.Key != "" && rec.Value == "" && rec.Delimiter == delimiterNone && rec.ItemMarker == itemMarkerNone {
				rec.Value = rec.Key
				rec.Key = ""
				return nil
			}
			if err != nil {
				return err
			}
		}

		if strings.HasPrefix(rec.Key, " ") || strings.HasSuffix(rec.Key, " ") {
			return ErrSpaceInKey
		}

		if strings.HasPrefix(rec.Value, "'") && strings.HasSuffix(rec.Value, "'") && rec.Value != "'" {
			rec.Value = strings.TrimPrefix(rec.Value, "'")
			rec.Value = strings.TrimSuffix(rec.Value, "'")
		}

		break
	}

	d.lastLineNumber += rec.size()

	return err
}

func (dec *Decoder) decode(field reflect.Value, indent uint, parentKey string, parentItemMarker itemMarker) (err error) {
	for {
		if dec.useNextRecord {
			dec.lastRecord = dec.nextRecord
			dec.useNextRecord = false
		} else {
			err = dec.readRecord("", &dec.lastRecord)
			if err == io.EOF {
				err = nil
				break
			} else if err != nil {
				break
			}
		}

		if slices.Contains([]reflect.Kind{reflect.Array, reflect.Map, reflect.Slice}, field.Kind()) && dec.lastRecord.Indent > indent && dec.lastRecord.ItemMarker == itemMarkerMap {
			return newDecodeError(dec.lastRecord.lineNumber, ErrInvalidIndentation, dec.lastRecord.Key, fmt.Sprint(indent), fmt.Sprint(dec.lastRecord.Indent))
		}

		if dec.lastRecord.Indent < indent {
			dec.nextRecord = dec.lastRecord
			dec.useNextRecord = true
			return nil
		}
		if dec.lastRecord.Indent > indent && !(parentItemMarker == itemMarkerMap && dec.lastRecord.ItemMarker == itemMarkerMap) && !(parentItemMarker == itemMarkerSliceString && dec.lastRecord.ItemMarker == itemMarkerSliceString) {
			return newDecodeError(dec.lastRecord.lineNumber, ErrInvalidIndentation, dec.lastRecord.Key, fmt.Sprint(indent), fmt.Sprint(dec.lastRecord.Indent))
		}

		if parentItemMarker == itemMarkerNone && parentItemMarker != dec.lastRecord.ItemMarker {
			dec.nextRecord = dec.lastRecord
			dec.useNextRecord = true
			return nil
		}
		if parentItemMarker != -1 && parentItemMarker != itemMarkerNone && dec.lastRecord.ItemMarker != parentItemMarker {
			dec.nextRecord = dec.lastRecord
			dec.useNextRecord = true
			return nil
		}

		if dec.lastRecord.ItemMarker == itemMarkerNone && field.Kind() == reflect.Map && field.Type().Key().Kind() == reflect.String && field.Type().Elem().Kind() == reflect.Interface {
			// decodding struct to map[string]any
		} else if dec.lastRecord.ItemMarker == itemMarkerNone && slices.Contains([]reflect.Kind{reflect.Array, reflect.Slice, reflect.Map}, field.Kind()) { // TODO: implement mixed type arrays
			dec.nextRecord = dec.lastRecord
			dec.useNextRecord = true
			return nil
		}

		if dec.lastRecord.Key != "" && dec.lastRecord.Value == "" && dec.lastRecord.Delimiter == delimiterString {
			// New field with types: struct, array, map, multiline string

			field = createAndGetField(field)

			if field.Kind() == reflect.Map {
				err = dec.readRecord("", &dec.nextRecord)
				if err != nil {
					return err
				}
				if dec.nextRecord.Indent == dec.lastRecord.Indent && dec.nextRecord.ItemMarker == itemMarkerMap && dec.lastRecord.ItemMarker == itemMarkerMap && dec.nextRecord.Key != "" {
					// empty value
					field.SetMapIndex(reflect.ValueOf(dec.lastRecord.Key), reflect.Zero(reflect.TypeFor[any]()))
					dec.useNextRecord = true
					continue
				}
				if dec.nextRecord.multilineStringBoundary != "" {
					field.SetMapIndex(reflect.ValueOf(dec.lastRecord.Key), reflect.ValueOf(dec.nextRecord.Value))
					continue
				}
				dec.useNextRecord = true

				var v reflect.Value
				if dec.nextRecord.Key == "" && dec.nextRecord.Value == "" && dec.nextRecord.ItemMarker == itemMarkerMap {
					// slice of anonymous maps
					v = reflect.New(reflect.TypeFor[[]map[string]any]()).Elem()
				} else {
					v = reflect.New(field.Type().Elem()).Elem()
				}
				key := dec.lastRecord.Key
				err = dec.decode(v, indent+iif(dec.nextRecord.ItemMarker != itemMarkerNone || strings.Contains(dec.nextRecord.Value, "\n"), uint(0), uint(1)), dec.lastRecord.Key, dec.nextRecord.ItemMarker)
				if err != nil {
					return err
				}

				field.SetMapIndex(reflect.ValueOf(key), v)

				continue
			}

			if sf, ok := field.Type().FieldByName(dec.lastRecord.Key); ok {
				if getStructFieldName(sf) == "-" {
					err = dec.readRecord("", &dec.nextRecord)
					if err != nil {
						return err
					}
					dec.useNextRecord = true

					err = dec.skipCurrentBlock(dec.nextRecord.Indent, dec.nextRecord.ItemMarker, &dec.nextRecord)
					if err != nil {
						return newDecodeError(dec.lastRecord.lineNumber, err, dec.lastRecord.Key, "", "") // TODO: add proper error.Error() format
					}
					continue
				}
			}
			field := field.FieldByName(dec.lastRecord.Key)
			if !field.IsValid() {
				if dec.SkipNonExistentFields {
					err = dec.readRecord("", &dec.nextRecord)
					if err != nil {
						return err
					}
					dec.useNextRecord = true

					err = dec.skipCurrentBlock(uint(indent), dec.nextRecord.ItemMarker, &dec.nextRecord)
					if err != nil {
						return newDecodeError(dec.lastRecord.lineNumber, err, dec.lastRecord.Key, "", "") // TODO: add proper error.Error() format
					}
					continue
				} else {
					return newDecodeError(dec.lastRecord.lineNumber, ErrFieldIsNotExists, dec.lastRecord.Key, "", "")
				}
			}

			err = dec.readRecord("", &dec.nextRecord)
			if err != nil {
				return newDecodeError(dec.lastRecord.lineNumber, err, dec.lastRecord.Key, "", "") // TODO: add proper error.Error() format
			}
			dec.useNextRecord = true

			if field.Type() == reflect.TypeFor[RawMessage]() {
				var s string
				s, err = dec.readBlock(&dec.nextRecord, &dec.nextRecord)
				if err != nil {
					return err
				}
				field.Set(reflect.ValueOf([]byte(s)))
				continue
			}

			if field.Type() == reflect.TypeFor[map[string]any]() {
				var str string
				str, err = dec.readBlock(&dec.nextRecord, &dec.nextRecord)
				if err != nil {
					return newDecodeError(dec.lastRecord.lineNumber, err, dec.lastRecord.Key, "", "") // TODO: add proper error.Error() format
				}
				field.Set(reflect.ValueOf([]byte(str)))
				continue
			}

			if dec.nextRecord.multilineStringBoundary != "" {
				field.Set(reflect.ValueOf(dec.nextRecord.Value))
				dec.useNextRecord = false
				continue
			}

			switch dec.nextRecord.ItemMarker {
			case itemMarkerMap:
				if field.Kind() != reflect.Map && dec.nextRecord.Key != "" {
					return newDecodeError(dec.nextRecord.lineNumber, ErrTryingToDecodeMapToNonMap, dec.nextRecord.Key, reflect.Map.String(), field.Kind().String())
				}

				err = dec.decode(field, indent, dec.lastRecord.Key, dec.nextRecord.ItemMarker)
				if err != nil {
					return err
				}
			case itemMarkerSliceString:
				if field.Kind() != reflect.Slice {
					return newDecodeError(dec.nextRecord.lineNumber, ErrTryingToDecodeSliceToNonSlice, dec.lastRecord.Key, reflect.Slice.String(), field.Kind().String())
				}
				if targetType(field.Type().Elem()).Kind() != reflect.String {
					return newDecodeError(dec.nextRecord.lineNumber, ErrInvalidSliceType, dec.lastRecord.Key, reflect.String.String(), targetType(field.Type().Elem()).Kind().String())
				}

				err = dec.decode(field, indent, dec.lastRecord.Key, dec.nextRecord.ItemMarker) // TODO: implement mixed type arrays
				if err != nil {
					return err
				}
			case itemMarkerSliceTyped:
				if field.Kind() != reflect.Slice {
					return newDecodeError(dec.nextRecord.lineNumber, ErrTryingToDecodeSliceToNonSlice, dec.lastRecord.Key, reflect.Slice.String(), field.Kind().String())
				}
				if targetType(field.Type().Elem()).Kind() == reflect.String {
					return newDecodeError(dec.nextRecord.lineNumber, ErrInvalidSliceType, dec.lastRecord.Key, reflect.String.String(), targetType(field.Type().Elem()).Kind().String())
				}

				err = dec.decode(field, indent, dec.lastRecord.Key, dec.nextRecord.ItemMarker) // TODO: implement mixed type arrays
				if err != nil {
					return err
				}
			default:
				field = createAndGetField(field)
				for f, v := range field.Fields() {
					if getStructFieldComment(f) {
						v.Set(reflect.ValueOf(dec.lastRecord.Comment))
						break
					}
				}

				err = dec.decode(field, indent+1, dec.lastRecord.Key, dec.nextRecord.ItemMarker)
				if err != nil {
					return err
				}
			}
		} else if dec.lastRecord.Key != "" && dec.lastRecord.Value != "" {
			// New field with simple value

			field = createAndGetField(field)

			if field.Kind() == reflect.Struct {
				found := false
				for field, value := range field.Fields() {
					key := dec.lastRecord.Key
					fieldName := getStructFieldName(field)
					if dec.IgnoreFieldNameCase {
						key = strings.ToLower(dec.lastRecord.Key)
						fieldName = strings.ToLower(fieldName)
					}
					if fieldName == key {
						if (dec.lastRecord.Directive == directiveMetadata) != getStructFieldMetadata(field) {
							continue
						}

						if !field.IsExported() {
							return newDecodeError(dec.lastRecord.lineNumber, ErrFieldIsNotExported, fieldName, "", "")
						}

						v, err := dec.lastRecord.decodeValue(field.Type)
						if err != nil {
							return err
						}
						if v != nil {
							setValue(value, v)
						}
						found = true
						break
					}
				}
				if !found && !(dec.lastRecord.Directive == directiveMetadata && dec.SkipNonExistentMetadataFields) {
					if dec.SkipNonExistentFields {
						err = dec.skipCurrentBlock(indent, itemMarkerNone, &dec.nextRecord)
						if err != nil {
							return err
						}
						dec.useNextRecord = true
					} else {
						return newDecodeError(dec.lastRecord.lineNumber, ErrFieldIsNotExists, dec.lastRecord.Key, "", "")
					}
				}
			} else if field.Kind() == reflect.Map {
				if field.Type().Key().Kind() != reflect.String {
					return newDecodeError(dec.lastRecord.lineNumber, ErrMapKeyIsNotString, dec.lastRecord.Key, reflect.String.String(), field.Type().Key().Kind().String())
				}
				if field.IsNil() {
					field.Set(reflect.MakeMap(field.Type()))
				}
				v, err := dec.lastRecord.decodeValue(field.Type().Elem())
				if err != nil {
					return err
				}
				field.SetMapIndex(reflect.ValueOf(dec.lastRecord.Key), reflect.ValueOf(v))
			}
		} else if dec.lastRecord.Key == "" && dec.lastRecord.Value != "" && dec.lastRecord.ItemMarker == itemMarkerNone {
			// simple value
			v, err := dec.lastRecord.decodeValue(field.Elem().Type())
			if err != nil {
				return err
			}

			field.Set(reflect.ValueOf(v))
		} else if dec.lastRecord.Key == "" && dec.lastRecord.Value != "" && slices.Contains([]itemMarker{itemMarkerSliceString, itemMarkerSliceTyped}, dec.lastRecord.ItemMarker) {
			// slice
			if field.Kind() == reflect.Interface {
				if field.IsNil() {
					field.Set(reflect.MakeSlice(reflect.TypeFor[[]string](), 0, 0))
				}
			}

			var sliceItemType reflect.Type
			if field.Kind() == reflect.Interface {
				sliceItemType = field.Elem().Type().Elem()
			} else {
				sliceItemType = field.Type().Elem()
			}

			v, err := dec.lastRecord.decodeValue(sliceItemType)
			if err != nil {
				return err
			}

			if slices.Contains([]reflect.Kind{reflect.Pointer, reflect.Interface}, field.Kind()) {
				if field.Elem().Type().Kind() == reflect.Pointer {
					v = getPointer(v)
				}
			}
			if field.Type().Kind() != reflect.Interface && field.Type().Elem().Kind() == reflect.Pointer {
				v = getPointer(v)
			}
			if field.Kind() == reflect.Interface {
				field.Set(reflect.Append(field.Elem(), reflect.ValueOf(v)))
			} else {
				field.Set(reflect.Append(field, reflect.ValueOf(v)))
			}
		} else if dec.lastRecord.Key == "" && dec.lastRecord.Value == "" && dec.lastRecord.ItemMarker == itemMarkerMap {
			if field.Type() == reflect.TypeFor[map[string]any]() {
				// decoding struct to map[string]any
			} else if field.Kind() != reflect.Slice {
				return newDecodeError(dec.lastRecord.lineNumber, ErrInvalidParentTypeOfAnonymousObject, parentKey, reflect.Slice.String(), field.Kind().String())
			}

			obj := reflect.New(field.Type().Elem())

			err = dec.decode(obj, indent+1, dec.lastRecord.Key, itemMarkerNone)
			if err != nil {
				return err
			}
			field.Set(reflect.Append(field, obj.Elem()))
		} else {
			return errors.New("unwknown conditions")
		}
	}
	return err
}

func (dec *Decoder) Decode(out any) (err error) {
	value := reflect.ValueOf(out)
	for value.Kind() == reflect.Pointer {
		value = value.Elem()
	}

	return dec.decode(value, 0, "<root>", -1)
}
