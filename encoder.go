package uno

import (
	"bufio"
	"encoding"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"time"
)

type Encoder struct {
	w io.Writer

	TimeLayout  string
	SortMapKeys bool
}

func NewEncoder(w io.Writer) *Encoder {
	enc := Encoder{
		w:           w,
		TimeLayout:  defaultEncoderTimeLayout,
		SortMapKeys: true}

	return &enc
}

func (enc *Encoder) Encode(in any) (err error) {
	e := reflect.ValueOf(in)
	for e.Kind() == reflect.Pointer {
		e = e.Elem()
	}

	switch e.Kind() {
	case reflect.Struct:
		if e.Type().String() == "time.Time" {
			_, err = enc.w.Write([]byte(e.Interface().(time.Time).Format(enc.TimeLayout)))
			break
		}

		for field, value := range e.Fields() {
			err = enc.encode(&field, getStructFieldName(field), value, "")
			if err != nil {
				break
			}
		}
	case reflect.Map:
		if enc.SortMapKeys {
			keys := e.MapKeys()
			sort.Sort(sortKeys(keys))

			for _, key := range keys {
				keyStr := key.Interface().(string)

				err := enc.encode(nil, keyStr, e.MapIndex(key), iif(e.Type() == reflect.TypeFor[map[string]any](), "", ". "))
				if err != nil {
					break
				}
			}
		} else {
			iter := e.MapRange()
			for iter.Next() {
				key := iter.Key().Interface().(string)

				err := enc.encode(nil, key, iter.Value(), "")
				if err != nil {
					break
				}
			}
		}
	case reflect.Slice:
		return enc.encode(nil, "", reflect.ValueOf(in), "")
	default:
		_, err = marshalValue(reflect.ValueOf(in), enc.w)
	}

	return err
}

func multilineBoundary(s string) string {
	r := bufio.NewReader(strings.NewReader(s))

	max := 0
	for {
		s, err := r.ReadString('\n')
		if err == io.EOF {
			break
		}

		l := 0
		for _, v := range s {
			if v == '\'' {
				l += 1
			} else {
				break
			}
		}
		if l > max {
			max = l
		}
	}

	return strings.Repeat("'", max+1)
}

func (enc *Encoder) encode(field *reflect.StructField, fieldName string, val reflect.Value, prefix string) error {
	if fieldName == "-" {
		return nil
	}

	if field != nil && getStructFieldOmitEmpty(*field) && valueIsEmpty(val) {
		return nil
	}

	if val.Type().String() != "time.Time" {
		if f, ok := getPointer(val.Interface()).(encoding.TextMarshaler); ok {
			b, err := f.MarshalText()
			if err != nil {
				return err
			}
			fmt.Fprintf(enc.w, "%s%s%s %v\n", prefix, fieldName, string(delimiterString), string(b))
			return nil
		}
	}

	for val.Kind() == reflect.Pointer {
		val = val.Elem()
	}

	directiveStr := ""
	if field != nil && getStructFieldMetadata(*field) {
		directiveStr = "# "
	}

	if val.Kind() == reflect.Interface { // TODO: add proper interface handler
		val = val.Elem()
	}

	switch val.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64,
		reflect.Bool:

		fmt.Fprintf(enc.w, "%s%s%s%s %v\n", prefix, directiveStr, fieldName, string(delimiterTyped), val.Interface())
	case reflect.String:
		str := val.Interface().(string)
		if strings.Contains(str, "\n") {
			mb := multilineBoundary(str)
			fmt.Fprintf(enc.w, "%s%s%s:\n%s\n%v\n%s\n", prefix, directiveStr, fieldName, mb, str, mb)
		} else {
			delimiter := string(delimiterString)
			if fieldName == "" {
				delimiter = "-"
			}
			fmt.Fprintf(enc.w, "%s%s%s%s %v\n", prefix, directiveStr, fieldName, delimiter, str)
		}
	case reflect.Slice, reflect.Array:
		if fieldName != "" {
			fmt.Fprintf(enc.w, "%s%s%s:\n", prefix, directiveStr, fieldName)
		}

		for i := range val.Len() {
			err := enc.encode(nil, "", val.Index(i), prefix)
			if err != nil {
				return err
			}
		}
	case reflect.Struct:
		if val.Type().String() == "time.Time" {
			fmt.Fprintf(enc.w, "%s%s%s%s %v\n", prefix, directiveStr, fieldName, string(delimiterTyped), val.Interface().(time.Time).Format(time.RFC3339Nano))
			return nil
		} else {
			if fieldName == "" {
				fmt.Fprintf(enc.w, "%s%s.\n", prefix, directiveStr)
			} else {
				fmt.Fprintf(enc.w, "%s%s%s:\n", prefix, directiveStr, fieldName)
			}
			newPrefix := strings.TrimSuffix(prefix, ". ")
			for field, value := range val.Fields() {
				err := enc.encode(&field, getStructFieldName(field), value, newPrefix+"\t")
				if err != nil {
					return err
				}
			}
		}
	case reflect.Map:
		if val.Type().Key().Kind() != reflect.String {
			return fmt.Errorf("%w: %s", ErrMapKeyIsNotString, val.Type().Key())
		}

		if fieldName != "" {
			fmt.Fprintf(enc.w, "%s%s%s:\n", prefix, directiveStr, fieldName)
		} else {
			fmt.Fprintf(enc.w, "%s%s\n", prefix, string(itemMarkerMap))
		}
		keys := val.MapKeys()
		if enc.SortMapKeys {
			sort.Sort(sortKeys(keys))
		}
		for _, key := range keys {
			newPrefix := strings.ReplaceAll(prefix, ". ", "\t")
			if fieldName == "" {
				newPrefix += "\t"
			} else {
				newPrefix += ". "
			}
			err := enc.encode(nil, key.String(), val.MapIndex(key), newPrefix)
			if err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedType, val.Type().String())
	}

	return nil
}

func marshalValue(val reflect.Value, w io.Writer) (int, error) {
	switch val.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64,
		reflect.Bool:

		return fmt.Fprint(w, val.Interface())
	case reflect.String:
		str := val.Interface().(string)
		if strings.Contains(str, "\n") {
			mb := multilineBoundary(str)
			return fmt.Fprintf(w, "%s\n%s\n%s\n", mb, str, mb)
		} else {
			return w.Write([]byte(str))
		}
	}

	return 0, fmt.Errorf("%w: %s", ErrUnsupportedType, val.Type().String())
}
