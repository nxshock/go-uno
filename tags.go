package uno

import (
	"reflect"
	"slices"
	"strings"
)

func getStructFieldName(field reflect.StructField) string {
	tag := strings.Split(field.Tag.Get("uno"), ",")
	if tag[0] != "" {
		return tag[0]
	}

	return field.Name
}

func getStructFieldOmitEmpty(field reflect.StructField) bool {
	return slices.Contains(strings.Split(field.Tag.Get("uno"), ","), "omitempty")
}
