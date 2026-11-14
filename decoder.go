package uno

import (
	"bufio"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"
)

type Decoder struct {
	r *rowReader
}

type rowReader struct {
	r *bufio.Reader
}

func (r *rowReader) ReadRow(comment string) (*record, error) {
	s, err := r.r.ReadString('\n')
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
		return r.ReadRow(comment)

	}

	return parseRecord(s, comment)
}

func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: &rowReader{r: bufio.NewReader(r)}}
}

func (dec *Decoder) decode(value reflect.Value, ident int) (unprocessedRow *record, err error) {
	var firstRow *record
	for {
		var row *record
		if firstRow != nil {
			row = firstRow
			firstRow = nil
		} else {
			row, err = dec.r.ReadRow("")
			if err == io.EOF {
				return nil, nil
			} else if err != nil {
				break
			}
		}

		if row.Indent < ident {
			return row, nil
		}

		if row.Key != "" && row.Value == "" && row.Delimiter == delimiterString {
			field := value.FieldByName(row.Key)
			if !field.IsValid() {
				return nil, fmt.Errorf("field %s is not exists in destination", row.Key)
			}

			firstRow, err = dec.decode(field, ident+1)
			if err != nil {
				return nil, err
			}
			continue
		} else if row.Key != "" && row.Value != "" {
			field := value.FieldByName(row.Key)
			if !field.IsValid() {
				return nil, fmt.Errorf("field %s is not exists in destination", row.Key)
			}

			f, ok := valParseFuncs[field.Kind()]
			if ok {
				v, err := f(row)
				if err != nil {
					return nil, err
				}
				field.Set(reflect.ValueOf(v))
				continue
			}

			if field.Type().Name() == "Time" {
				valueReady := false
				for _, layout := range []string{time.RFC3339Nano, time.RFC3339, time.DateOnly, time.TimeOnly} {
					t, err := time.Parse(layout, row.Value)
					if err == nil {
						field.Set(reflect.ValueOf(t))
						valueReady = true
						break
					}
				}
				if !valueReady {
					return nil, fmt.Errorf("unsupported time layout: %s", row.Value)
				}
				continue
			}

			return nil, fmt.Errorf("unsupported type: %s", field.Kind())
		}
	}
	return nil, err
}

func (dec *Decoder) Decode(out any) (err error) {
	e := reflect.ValueOf(out)
	for e.Kind() == reflect.Pointer {
		e = e.Elem()
	}

	_, err = dec.decode(e, 0)
	return err
}
