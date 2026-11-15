package uno

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseRow(t *testing.T) {
	tests := []struct {
		s        string
		expected record
	}{
		// test keys https://uno.mudrium.ru/uno_core/oblast_klyucha.html
		{s: "'': val", expected: record{Indent: 0, Directive: directiveNone, ItemMarker: itemMarkerNone, Key: "", Delimiter: delimiterString, Value: "val"}},
		{s: "key: val", expected: record{Indent: 0, Directive: directiveNone, ItemMarker: itemMarkerNone, Key: "key", Delimiter: delimiterString, Value: "val"}},
		{s: "n_key: val", expected: record{Indent: 0, Directive: directiveNone, ItemMarker: itemMarkerNone, Key: "n_key", Delimiter: delimiterString, Value: "val"}},
		{s: "n-key: val", expected: record{Indent: 0, Directive: directiveNone, ItemMarker: itemMarkerNone, Key: "n-key", Delimiter: delimiterString, Value: "val"}},
		{s: "-key: val", expected: record{Indent: 0, Directive: directiveNone, ItemMarker: itemMarkerNone, Key: "-key", Delimiter: delimiterString, Value: "val"}},
		{s: "n key: val", expected: record{Indent: 0, Directive: directiveNone, ItemMarker: itemMarkerNone, Key: "n key", Delimiter: delimiterString, Value: "val"}},
		{s: "1234: val", expected: record{Indent: 0, Directive: directiveNone, ItemMarker: itemMarkerNone, Key: "1234", Delimiter: delimiterString, Value: "val"}},
	}

	for _, test := range tests {
		got, err := parseRecord(test.s, "")
		assert.NoError(t, err)
		assert.Equal(t, test.expected, *got)
	}
}

func TestListItem(t *testing.T) {
	tests := []struct {
		s        string
		expected record
	}{
		{s: "- text", expected: record{Indent: 0, Directive: directiveNone, ItemMarker: itemMarkerString, Key: "", Delimiter: delimiterNone, Value: "text"}},
		{s: "= 129.56", expected: record{Indent: 0, Directive: directiveNone, ItemMarker: itemMarkerTyped, Key: "", Delimiter: delimiterNone, Value: "129.56"}},
	}

	for _, test := range tests {
		got, err := parseRecord(test.s, "")
		assert.NoError(t, err)
		assert.Equal(t, test.expected, *got)
	}
}

func TestParseString(t *testing.T) {
	tests := []struct {
		s        string
		expected string
	}{
		{s: "key: text string", expected: "text string"},
		{s: "key: '    text string    '", expected: "    text string    "},
		{s: "key: ''text ''", expected: "'text '"},
	}

	for _, test := range tests {
		row, err := parseRecord(test.s, "")
		assert.NoError(t, err)
		v, err := row.decodeValue(reflect.TypeOf(""))
		assert.Equal(t, test.expected, v)
	}
}

func TestParseBool(t *testing.T) {
	tests := []struct {
		s        string
		expected bool
	}{
		{s: "key= true", expected: true},
		{s: "key= on", expected: true},
		{s: "key= open", expected: true},
		{s: "key= yes", expected: true},

		{s: "key= false", expected: false},
		{s: "key= off", expected: false},
		{s: "key= close", expected: false},
		{s: "key= no", expected: false},
	}

	for _, test := range tests {
		row, err := parseRecord(test.s, "")
		assert.NoError(t, err)
		v, err := row.decodeValue(reflect.TypeOf(false))
		assert.NoError(t, err)
		assert.Equal(t, test.expected, v)
	}
}

func TestParseFloat64(t *testing.T) {
	tests := []struct {
		s        string
		expected float64
	}{
		{s: "key= 1.0", expected: 1},
		{s: "key= 1", expected: 1},
		{s: "key= 3.141592653589793", expected: 3.141592653589793},
		{s: "key= -0.01", expected: -0.01},
		{s: "key= 12_334.567_235_27", expected: 12334.56723527},
		{s: "key= 1e+3", expected: 1e3},
		{s: "key= 2.745e+6", expected: 2.745e+6},
		{s: "key= 1e-02", expected: 1e-02},
		{s: "key= -2e-2", expected: -2e-2},
		{s: "key= 6.626e-34", expected: 6.626e-34},
	}

	for _, test := range tests {
		row, err := parseRecord(test.s, "")
		assert.NoError(t, err)
		v, err := row.decodeValue(reflect.TypeOf(float64(0)))
		assert.NoError(t, err)
		assert.Equal(t, test.expected, v)
	}

	infTests := []struct {
		s    string
		sign int
	}{
		{s: "key= inf", sign: 1},
		{s: "key= -inf", sign: -1},
	}

	for _, test := range infTests {
		row, err := parseRecord(test.s, "")
		assert.NoError(t, err)
		v, err := row.decodeValue(reflect.TypeOf(float64(0)))
		assert.NoError(t, err)
		assert.True(t, math.IsInf(v.(float64), test.sign))
	}

	row, err := parseRecord("key= nan", "")
	assert.NoError(t, err)
	v, err := row.decodeValue(reflect.TypeOf(float64(0)))
	assert.NoError(t, err)
	assert.True(t, math.IsNaN(v.(float64)))
}

func TestParseObject(t *testing.T) {
	s := "object:"
	expected := record{
		Indent:     0,
		Directive:  directiveNone,
		ItemMarker: itemMarkerNone,
		Key:        "object",
		Delimiter:  delimiterString,
		Value:      ""}

	row, err := parseRecord(s, "")
	assert.NoError(t, err)
	assert.Equal(t, expected, *row)
}

func TestUnmarshal(t *testing.T) {
	str := `Int= 1
Uint=    2
Float64= 0.3
Bool=    true
String:  string`

	type S struct {
		Int     int
		Uint    uint
		Float64 float64
		Bool    bool
		String  string
	}

	s := S{}
	expected := S{
		Int:     1,
		Uint:    2,
		Float64: 0.3,
		Bool:    true,
		String:  "string"}

	err := Unmarshal([]byte(str), &s)
	assert.NoError(t, err)
	assert.Equal(t, expected, s)
}

func TestMarshal(t *testing.T) {
	s := struct {
		Int     int
		Uint    uint
		Float64 float64
		Bool    bool
		String  string
		Time    time.Time
	}{
		Int:     1,
		Uint:    2,
		Float64: 0.3,
		Bool:    true,
		String:  "string",
		Time:    time.Date(2006, 5, 4, 3, 2, 1, 0, time.UTC),
	}

	expected := `Int= 1
Uint= 2
Float64= 0.3
Bool= true
String: string
Time= 2006-05-04T03:02:01Z
`

	b, err := Marshal(&s)
	assert.NoError(t, err)
	assert.Equal(t, expected, string(b))
}

func TestMarshalUnmarshal(t *testing.T) {
	type S struct {
		Int     int
		Int8    int8
		Int16   int16
		Int32   int32
		Int64   int64
		Uint    uint
		Uint8   uint8
		Uint16  uint16
		Uint32  uint32
		Uint64  uint64
		Float32 float32
		Float64 float64
		Bool    bool
		String  string
		Time    time.Time
	}

	s := S{
		Int:     -1,
		Int8:    -2,
		Int16:   -3,
		Int32:   -4,
		Int64:   -5,
		Uint:    6,
		Uint8:   7,
		Uint16:  8,
		Uint32:  9,
		Uint64:  10,
		Float32: -0.11,
		Float64: -0.12,
		Bool:    true,
		String:  "string",
		Time:    time.Date(2006, 5, 4, 3, 2, 1, 0, time.UTC)}

	b, err := Marshal(s)
	assert.NoError(t, err)

	newS := S{}
	err = Unmarshal(b, &newS)
	assert.NoError(t, err)
	assert.Equal(t, s, newS)
}

func TestMarshalArray(t *testing.T) {
	type Nested struct {
		Strings []string
	}

	type S struct {
		Ints    []int
		Strings []string
	}

	s := S{Ints: []int{1, 2}, Strings: []string{"3", "4"}}

	b, err := Marshal(s)
	assert.NoError(t, err)

	expected := `Ints:
= 1
= 2
Strings:
- 3
- 4
`

	assert.Equal(t, expected, string(b))
}

func TestMarshalStruct(t *testing.T) {
	type Nested struct {
		Str string
	}

	type S struct {
		Nested Nested
	}

	s := S{Nested: Nested{"123"}}

	b, err := Marshal(s)
	assert.NoError(t, err)

	expected := `Nested:
	Str: 123
`

	assert.Equal(t, expected, string(b))
}

func TestComment(t *testing.T) {
	type S struct {
		Key string
	}

	str := `// Comment line
Key: Value`

	expected := S{Key: "Value"}

	got := S{}
	err := Unmarshal([]byte(str), &got)
	assert.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestMultilineComment(t *testing.T) {
	type S struct {
		Key string
	}

	str := `// Comment line 1
// Comment line 2
Key: Value`

	expected := S{Key: "Value"}

	got := S{}
	err := Unmarshal([]byte(str), &got)
	assert.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestUnmarshalStruct(t *testing.T) {
	type Nested struct {
		Str string
	}

	type S struct {
		Nested  Nested
		Int     int
		Nested2 Nested
	}

	str := `Int= 123
Nested:
	Str: 456
Nested2:
	Str: 789`

	s := S{}
	expected := S{Int: 123, Nested: Nested{"456"}, Nested2: Nested{"789"}}

	err := Unmarshal([]byte(str), &s)
	assert.NoError(t, err)
	assert.Equal(t, expected, s)
}

func TestMarshalMap(t *testing.T) {
	type S struct {
		StringMap map[string]string
		IntMap    map[string]int
	}

	var tests = []struct {
		s        S
		expected string
	}{
		{s: S{StringMap: map[string]string{"Key1": "Value1", "Key2": "Value2"}, IntMap: map[string]int{"Key1": 123, "Key2": 456}},
			expected: `StringMap:
	Key1: Value1
	Key2: Value2
IntMap:
	Key1= 123
	Key2= 456
`}}

	for _, test := range tests {
		got, err := Marshal(test.s)
		assert.NoError(t, err)
		assert.Equal(t, test.expected, string(got))
	}
}

func TestUnmarshalMap(t *testing.T) {
	type S struct {
		StringMap map[string]string
		IntMap    map[string]int
	}

	var tests = []struct {
		str      string
		expected S
	}{
		{str: `StringMap:
	Key1: Value1
	Key2: Value2
IntMap:
	Key1= 123
	Key2= 456`, expected: S{StringMap: map[string]string{"Key1": "Value1", "Key2": "Value2"}, IntMap: map[string]int{"Key1": 123, "Key2": 456}}}}

	for _, test := range tests {
		got := S{}
		err := Unmarshal([]byte(test.str), &got)
		assert.NoError(t, err)
		assert.Equal(t, test.expected, got)
	}
}

func TestUnmarshalTime(t *testing.T) {
	type S struct {
		Key time.Time
	}

	var tests = []struct {
		str      string
		expected S
	}{
		{str: "Key: 2006-05-04", expected: S{Key: time.Date(2006, 5, 4, 0, 0, 0, 0, time.UTC)}},
		{str: "Key: 15:02:01", expected: S{Key: time.Date(0, 1, 1, 15, 2, 1, 0, time.UTC)}},
	}

	for _, test := range tests {
		got := S{}

		err := Unmarshal([]byte(test.str), &got)
		assert.NoError(t, err)
		assert.Equal(t, test.expected, got)
	}
}

func TestMarshalUnmarshalMap(t *testing.T) {
	type S struct {
		StringMap map[string]string
		IntMap    map[string]int
	}

	var tests = []struct {
		s S
	}{
		{s: S{StringMap: map[string]string{"Key1": "Value1", "Key2": "Value2"}, IntMap: map[string]int{"Key1": 123, "Key2": 456}}}}

	for _, test := range tests {
		b, err := Marshal(test.s)
		assert.NoError(t, err)

		newS := S{}
		err = Unmarshal(b, &newS)
		assert.NoError(t, err)

		assert.Equal(t, test.s, newS)
	}
}

func TestUnmarshalToWrongType(t *testing.T) {
	s := 0

	err := Unmarshal([]byte(""), &s)
	assert.Error(t, err)
}

func TestUnmarshalPointers(t *testing.T) {
	type S struct {
		Str   *string
		Int   *int
		Float *float64
		Time  *time.Time
	}

	str := `Str: 123
	Int= 456
	Float= 0.789
	Time= 2006-05-04`

	pStr := "123"
	pInt := 456
	pFloat := 0.789
	pTime := time.Date(2006, 5, 4, 0, 0, 0, 0, time.UTC)
	expected := S{Str: &pStr, Int: &pInt, Float: &pFloat, Time: &pTime}

	var got S
	err := Unmarshal([]byte(str), &got)
	assert.NoError(t, err)
	assert.EqualValues(t, expected, got)
}

func TestMarshalUnmarshalPointers(t *testing.T) {
	type S struct {
		Str   *string
		Int   *int
		Float *float64
		Time  *time.Time
	}

	pStr := "123"
	pInt := 456
	pFloat := 0.789
	pTime := time.Date(2006, 5, 4, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		s S
	}{
		{s: S{Str: &pStr, Int: &pInt, Float: &pFloat, Time: &pTime}},
	}

	for _, test := range tests {
		b, err := Marshal(test.s)
		assert.NoError(t, err)

		newS := S{}
		err = Unmarshal(b, &newS)
		assert.NoError(t, err)

		assert.EqualValues(t, test.s, newS)
	}
}

type field int

func (f *field) MarshalText() (text []byte, err error) {
	return fmt.Appendf(nil, "^%d^", *f), nil
}

func (f *field) UnmarshalText(text []byte) error {
	i, err := strconv.Atoi(strings.Trim(string(text), "^"))
	if err != nil {
		return err
	}

	*f = field(i)

	return nil
}

func TestMarshaler(t *testing.T) {
	type S struct {
		F field
	}

	i := 123

	b, err := Marshal(S{F: field(i)})
	assert.NoError(t, err)
	assert.Equal(t, "F: ^123^\n", string(b))
}

func TestUnmarshaler(t *testing.T) {
	type S struct {
		F field
	}

	str := "F: ^123^\n"

	s := S{}

	err := Unmarshal([]byte(str), &s)
	assert.NoError(t, err)
	assert.Equal(t, S{F: field(123)}, s)
}
