package uno

import (
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"
)

type Encoder struct {
	w io.Writer
}

func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w}
}

func (enc *Encoder) Encode(in any) error {
	e := reflect.ValueOf(in)
	for e.Kind() == reflect.Pointer {
		e = e.Elem()
	}

	for field, value := range e.Fields() {
		err := enc.encode(field.Name, value, 0)
		if err != nil {
			return err
		}
	}

	return nil
}

func (enc *Encoder) encode(fieldName string, val reflect.Value, level int) error {
	if val.Kind() == reflect.Pointer {
		val = val.Elem()
	}

	if val.Type().Name() == "Time" {
		fmt.Fprintf(enc.w, "%s%s%s %v\n", strings.Repeat("\t", level), fieldName, string(delimiterTyped), val.Interface().(time.Time).Format(time.RFC3339Nano))
		return nil
	}

	switch val.Kind() {
	case reflect.Int, reflect.Int64, reflect.Uint, reflect.Uint64, reflect.Float64, reflect.Bool:
		fmt.Fprintf(enc.w, "%s%s%s %v\n", strings.Repeat("\t", level), fieldName, string(delimiterTyped), val.Interface())
	case reflect.String:
		delimiter := string(delimiterString)
		if fieldName == "" {
			delimiter = "-"
		}
		fmt.Fprintf(enc.w, "%s%s%s %v\n", strings.Repeat("\t", level), fieldName, delimiter, val.Interface())
	case reflect.Slice:
		fmt.Fprintf(enc.w, "%s%s:\n", strings.Repeat("\t", level), fieldName)

		for i := range val.Len() {
			enc.encode("", val.Index(i), level)
		}
	case reflect.Struct:
		fmt.Fprintf(enc.w, "%s%s:\n", strings.Repeat("\t", level), fieldName)
		for field, value := range val.Fields() {
			err := enc.encode(field.Name, value, level+1)
			if err != nil {
				return err
			}
		}
	case reflect.Map:
		fmt.Fprintf(enc.w, "%s%s:\n", strings.Repeat("\t", level), fieldName)
		for _, key := range val.MapKeys() {
			err := enc.encode(key.String(), val.MapIndex(key), level+1)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
