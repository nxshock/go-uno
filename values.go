package uno

import "reflect"

func valueIsEmpty(v reflect.Value) bool {
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return true
		}

		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Map, reflect.String, reflect.Array, reflect.Slice:
		return v.IsZero()
	}

	return false
}
