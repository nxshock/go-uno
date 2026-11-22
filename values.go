package uno

import (
	"reflect"
)

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

func setValue(field reflect.Value, value any) {
	v := reflect.ValueOf(value)

	for v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	value = v.Interface()

	if field.Kind() == reflect.Pointer {
		for _, c := range field.Type().String() {
			if c != '*' {
				break
			}

			value = getPointer(value)
		}
		field.Set(reflect.ValueOf(value))
	} else {
		field.Set(v)
	}
}

func targetType(value reflect.Type) reflect.Type {
	for value.Kind() == reflect.Pointer {
		value = value.Elem()
	}

	return value
}

func getPointer(obj any) any {
	vp := reflect.New(reflect.TypeOf(obj))
	vp.Elem().Set(reflect.ValueOf(obj))
	return vp.Interface()
}

func createAndGetField(field reflect.Value) reflect.Value {
	// move to last pointer
	for field.Kind() == reflect.Pointer && field.Elem().Kind() != reflect.Invalid {
		field = field.Elem()
	}

	// if pointer to nil then init zero value
	if field.Kind() == reflect.Pointer && field.IsNil() {
		field.Set(reflect.New(field.Type().Elem()))
	}

	if field.Kind() != reflect.Pointer {
		if field.Kind() == reflect.Map && field.IsNil() {
			field.Set(reflect.MakeMap(field.Type())) // TODO: проверить, что нигде падать не будет
		}
	}

	// move to non-pointer
	if field.Kind() == reflect.Pointer {
		field = field.Elem()
	}

	return field
}
