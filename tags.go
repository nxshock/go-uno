package uno

import (
	"reflect"
	"slices"
	"strings"
)

func getStructFieldName(field reflect.StructField) string {
	// prevents strings.Split() allocations
	if field.Tag.Get("uno") == "" {
		return field.Name
	}

	tag := strings.Split(field.Tag.Get("uno"), ",")
	if tag[0] != "" {
		return tag[0]
	}

	return field.Name
}

func getStructFieldOmitEmpty(field reflect.StructField) bool {
	// prevents strings.Split() allocations
	tag := field.Tag.Get("uno")
	if tag == "" {
		return false
	}

	return slices.Contains(strings.Split(tag, ","), "omitempty")
}

func getStructFieldMetadata(field reflect.StructField) bool {
	// prevents strings.Split() allocations
	tag := field.Tag.Get("uno")
	if tag == "" {
		return false
	}

	return slices.Contains(strings.Split(tag, ","), "metadata")
}
