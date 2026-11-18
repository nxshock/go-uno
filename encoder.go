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

func (enc *Encoder) Encode(in any) error {
	e := reflect.ValueOf(in)
	for e.Kind() == reflect.Pointer {
		e = e.Elem()
	}

	for field, value := range e.Fields() {
		err := enc.encode(&field, getStructFieldName(field), value, "")
		if err != nil {
			return err
		}
	}

	return nil
}

func getPointer(obj any) any {
	vp := reflect.New(reflect.TypeOf(obj))
	vp.Elem().Set(reflect.ValueOf(obj))
	return vp.Interface()
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

	switch val.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		fmt.Fprintf(enc.w, "%s%s%s%s %v\n", prefix, directiveStr, fieldName, string(delimiterTyped), val.Interface())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		fmt.Fprintf(enc.w, "%s%s%s%s %v\n", prefix, directiveStr, fieldName, string(delimiterTyped), val.Interface())
	case reflect.Float32, reflect.Float64:
		fmt.Fprintf(enc.w, "%s%s%s%s %v\n", prefix, directiveStr, fieldName, string(delimiterTyped), val.Interface())
	case reflect.Bool:
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
		fmt.Fprintf(enc.w, "%s%s%s:\n", prefix, directiveStr, fieldName)

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
			fmt.Fprintf(enc.w, "%s%s%s:\n", prefix, directiveStr, fieldName)
			for field, value := range val.Fields() {
				err := enc.encode(&field, getStructFieldName(field), value, prefix+"\t")
				if err != nil {
					return err
				}
			}
		}
	case reflect.Map:
		fmt.Fprintf(enc.w, "%s%s%s:\n", prefix, directiveStr, fieldName)
		keys := val.MapKeys()
		if enc.SortMapKeys {
			sort.Sort(sortKeys(keys))
		}
		for _, key := range keys {
			err := enc.encode(nil, key.String(), val.MapIndex(key), prefix+". ")
			if err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedType, val.Type().String())
	}

	return nil
}
