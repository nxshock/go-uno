package uno

import (
	"fmt"
	"reflect"
)

type sortKeys []reflect.Value

func (k sortKeys) Len() int {
	return len(k)
}

func (k sortKeys) Swap(i, j int) {
	k[i], k[j] = k[j], k[i]
}

func (k sortKeys) Less(i, j int) bool {
	return fmt.Sprint(k[i].Interface()) < fmt.Sprint(k[j].Interface())
}
