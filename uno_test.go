package uno

import (
	"bytes"
	"fmt"
	"io"
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
		{s: "'': val\n", expected: record{Indent: 0, Directive: directiveNone, ItemMarker: itemMarkerNone, Key: "", Delimiter: delimiterString, Value: "val", lineNumber: 1}},
		{s: "key: val\n", expected: record{Indent: 0, Directive: directiveNone, ItemMarker: itemMarkerNone, Key: "key", Delimiter: delimiterString, Value: "val", lineNumber: 1}},
		{s: "n_key: val\n", expected: record{Indent: 0, Directive: directiveNone, ItemMarker: itemMarkerNone, Key: "n_key", Delimiter: delimiterString, Value: "val", lineNumber: 1}},
		{s: "n-key: val\n", expected: record{Indent: 0, Directive: directiveNone, ItemMarker: itemMarkerNone, Key: "n-key", Delimiter: delimiterString, Value: "val", lineNumber: 1}},
		{s: "-key: val\n", expected: record{Indent: 0, Directive: directiveNone, ItemMarker: itemMarkerNone, Key: "-key", Delimiter: delimiterString, Value: "val", lineNumber: 1}},
		{s: "n key: val\n", expected: record{Indent: 0, Directive: directiveNone, ItemMarker: itemMarkerNone, Key: "n key", Delimiter: delimiterString, Value: "val", lineNumber: 1}},
		{s: "1234: val\n", expected: record{Indent: 0, Directive: directiveNone, ItemMarker: itemMarkerNone, Key: "1234", Delimiter: delimiterString, Value: "val", lineNumber: 1}},
		{s: "key: ''\n", expected: record{Indent: 0, Directive: directiveNone, ItemMarker: itemMarkerNone, Key: "key", Delimiter: delimiterString, Value: "", lineNumber: 1}},
		{s: "- Key: val\n", expected: record{Indent: 0, Directive: directiveNone, ItemMarker: itemMarkerSliceString, Key: "", Delimiter: delimiterString, Value: "Key: val", lineNumber: 1}},
		{s: "= Key: val\n", expected: record{Indent: 0, Directive: directiveNone, ItemMarker: itemMarkerSliceTyped, Key: "", Delimiter: delimiterTyped, Value: "Key: val", lineNumber: 1}},
		{s: "# key: val\n", expected: record{Indent: 0, Directive: directiveMetadata, ItemMarker: itemMarkerNone, Key: "key", Delimiter: delimiterString, Value: "val", lineNumber: 1}},
		{s: ".\n", expected: record{Indent: 0, Directive: directiveNone, ItemMarker: itemMarkerMap, Key: "", Delimiter: delimiterNone, Value: "", lineNumber: 1}},
	}

	for _, test := range tests {
		dec := NewDecoder(strings.NewReader(test.s))
		err := dec.readRecord("", &dec.lastRecord)
		assert.NoError(t, err)
		assert.Equal(t, test.expected, dec.lastRecord)

		str := dec.lastRecord.String()
		assert.Equal(t, test.s, str)
	}
}

func TestListItem(t *testing.T) {
	tests := []struct {
		s        string
		expected record
	}{
		{s: "- text", expected: record{Indent: 0, Directive: directiveNone, ItemMarker: itemMarkerSliceString, Key: "", Delimiter: delimiterString, Value: "text", lineNumber: 1}},
		{s: "= 129.56", expected: record{Indent: 0, Directive: directiveNone, ItemMarker: itemMarkerSliceTyped, Key: "", Delimiter: delimiterTyped, Value: "129.56", lineNumber: 1}},
	}

	for _, test := range tests {
		dec := NewDecoder(strings.NewReader(test.s))
		err := dec.readRecord("", &dec.lastRecord)
		assert.NoError(t, err)
		assert.Equal(t, test.expected, dec.lastRecord)
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
		{s: "key: '", expected: "'"},
	}

	for _, test := range tests {
		dec := NewDecoder(strings.NewReader(test.s))
		err := dec.readRecord("", &dec.lastRecord)
		assert.NoError(t, err)

		v, err := dec.lastRecord.decodeValue(reflect.TypeFor[string]())
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
		dec := NewDecoder(strings.NewReader(test.s))
		err := dec.readRecord("", &dec.lastRecord)
		assert.NoError(t, err)
		v, err := dec.lastRecord.decodeValue(reflect.TypeOf(false))
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
		dec := NewDecoder(strings.NewReader(test.s))
		err := dec.readRecord("", &dec.lastRecord)
		assert.NoError(t, err)
		v, err := dec.lastRecord.decodeValue(reflect.TypeOf(float64(0)))
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
		dec := NewDecoder(strings.NewReader(test.s))
		err := dec.readRecord("", &dec.lastRecord)
		assert.NoError(t, err)
		v, err := dec.lastRecord.decodeValue(reflect.TypeOf(float64(0)))
		assert.NoError(t, err)
		assert.True(t, math.IsInf(v.(float64), test.sign))
	}

	dec := NewDecoder(strings.NewReader("key= nan"))
	err := dec.readRecord("", &dec.lastRecord)
	assert.NoError(t, err)
	v, err := dec.lastRecord.decodeValue(reflect.TypeOf(float64(0)))
	assert.NoError(t, err)
	assert.True(t, math.IsNaN(v.(float64)))
}

func TestParseObject(t *testing.T) {
	s := "object:\n"
	expected := record{
		Indent:     0,
		Directive:  directiveNone,
		ItemMarker: itemMarkerNone,
		Key:        "object",
		Delimiter:  delimiterString,
		Value:      "",
		lineNumber: 1}

	dec := NewDecoder(strings.NewReader(s))
	err := dec.readRecord("", &dec.lastRecord)
	assert.NoError(t, err)
	assert.Equal(t, expected, dec.lastRecord)
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
		Int           int
		Int8          int8
		Int16         int16
		Int32         int32
		Int64         int64
		Uint          uint
		Uint8         uint8
		Uint16        uint16
		Uint32        uint32
		Uint64        uint64
		Float32       float32
		Float64       float64
		Bool          bool
		String        string
		Time          time.Time
		SliceOfString []string
		SliceOfInt    []int
		Map           map[string]string
	}

	s := S{
		Int:           -1,
		Int8:          -2,
		Int16:         -3,
		Int32:         -4,
		Int64:         -5,
		Uint:          6,
		Uint8:         7,
		Uint16:        8,
		Uint32:        9,
		Uint64:        10,
		Float32:       -0.11,
		Float64:       -0.12,
		Bool:          true,
		String:        "string",
		Time:          time.Date(2006, 5, 4, 3, 2, 1, 0, time.UTC),
		SliceOfString: []string{"val1", "val2", "val3"},
		SliceOfInt:    []int{1, 2, 3, 4, 5, 6, 7, 8, 9},
		Map:           map[string]string{"k1": "v1", "k2": "v2", "k3": "v3"},
	}

	b, err := Marshal(s)
	assert.NoError(t, err)

	newS := S{}
	err = Unmarshal(b, &newS)
	assert.NoError(t, err)
	assert.Equal(t, s, newS)
}

func TestMarshalSliceOfMaps(t *testing.T) {
	type S struct {
		Maps []map[string]string
	}

	expected := `Maps:
.
	k1: v1
.
	k2: v2
`

	s := S{Maps: []map[string]string{{"k1": "v1"}, {"k2": "v2"}}}

	got, err := Marshal(s)
	assert.NoError(t, err)
	assert.Equal(t, expected, string(got))
}

func TestUnmarshalSliceOfMaps(t *testing.T) {
	type S struct {
		Maps []map[string]string
	}

	str := `Maps:
.
	k1: v1
.
	k2: v2
`

	expected := S{Maps: []map[string]string{{"k1": "v1"}, {"k2": "v2"}}}
	got := S{}
	err := Unmarshal([]byte(str), &got)
	assert.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestMarshalUnmarshalMapSlice(t *testing.T) {
	s := map[string]any{"Map": []map[string]any{{"k1": "v1"}, {"k2": "v2"}, {"k3": "v3"}}}

	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	enc.SortMapKeys = false

	err := enc.Encode(s)
	assert.NoError(t, err)

	newS := map[string]any{}
	err = Unmarshal(buf.Bytes(), &newS)
	assert.NoError(t, err)
	assert.Equal(t, s, newS)
}

func TestMarshalSlice(t *testing.T) {
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

func TestUnmarshalSlice(t *testing.T) {
	type S struct {
		Ints    []int
		Strings []string
	}

	expected := S{Ints: []int{1, 2}, Strings: []string{"3", "4"}}

	str := `Ints:
= 1
= 2
Strings:
- 3
- 4
`

	got := S{}
	err := Unmarshal([]byte(str), &got)
	assert.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestMarshalUnmarshalSlice(t *testing.T) {
	type S struct {
		Ints    []int
		Strings []string
	}

	expected := S{Ints: []int{1, 2}, Strings: []string{"3", "4"}}

	b, err := Marshal(expected)
	assert.NoError(t, err)

	got := S{}
	err = Unmarshal(b, &got)
	assert.NoError(t, err)
	assert.Equal(t, expected, got)
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
		C string `uno:",comment"`

		Key string
	}

	str := "// Comment line\nKey: Value"

	expected := S{Key: "Value"}

	got := S{}
	err := Unmarshal([]byte(str), &got)
	assert.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestMultipleComment(t *testing.T) {
	type S struct {
		Key string
	}

	str := "// Comment line 1\n// Comment line 2\nKey: Value"

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
. Key1: Value1
. Key2: Value2
IntMap:
. Key1= 123
. Key2= 456
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
. Key1: Value1
. Key2: Value2
IntMap:
. Key1= 123
. Key2= 456`, expected: S{StringMap: map[string]string{"Key1": "Value1", "Key2": "Value2"}, IntMap: map[string]int{"Key1": 123, "Key2": 456}}}}

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

func TestUnmarshalPointers(t *testing.T) {
	type S struct {
		Str   *string
		Int   *int
		Float *float64
		Time  *time.Time
	}

	str := "Str: 123\nInt= 456\nFloat= 0.789\nTime= 2006-05-04\n"

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

func TestDontAllowSpaceInKeys(t *testing.T) {
	tests := []struct {
		s           string
		expectedErr error
	}{
		{s: " key: val", expectedErr: ErrSpaceInKey},
		{s: "key : val", expectedErr: ErrSpaceInKey},
		{s: " key : val", expectedErr: ErrSpaceInKey},
	}

	for _, test := range tests {
		dec := NewDecoder(strings.NewReader(test.s))
		err := dec.readRecord("", &dec.lastRecord)
		assert.ErrorIs(t, err, test.expectedErr)
	}
}

func TestBool(t *testing.T) {
	type test struct {
		valueStr      string
		expectedValue bool
		expectedErr   error
	}

	tests := []test{
		// true
		{valueStr: "true", expectedValue: true, expectedErr: nil},
		{valueStr: "on", expectedValue: true, expectedErr: nil},
		{valueStr: "open", expectedValue: true, expectedErr: nil},
		{valueStr: "yes", expectedValue: true, expectedErr: nil},
		// false
		{valueStr: "false", expectedValue: false, expectedErr: nil},
		{valueStr: "off", expectedValue: false, expectedErr: nil},
		{valueStr: "close", expectedValue: false, expectedErr: nil},
		{valueStr: "no", expectedValue: false, expectedErr: nil},
		// other
		{valueStr: "any", expectedValue: false, expectedErr: ErrInvalidBoolValue},
	}

	for _, test := range tests {
		type S struct {
			B bool
		}

		s := S{}
		str := fmt.Sprintf("B= %s", test.valueStr)

		err := Unmarshal([]byte(str), &s)
		assert.ErrorIs(t, err, test.expectedErr)
		assert.Equal(t, test.expectedValue, s.B)
	}
}

/*func TestErrLineNumbers(t *testing.T) {
	type S struct {
		F1 string
		F2 bool
	}

	tests := []struct {
		s        string
		expected uint
	}{
		{s: "F1: value1\nF2: true", expected: 2},
		{s: "F1: value1\n// Comment\nF2: true", expected: 3},
		{s: "F1: value1\n/'\nComment line 1\n\nComment line 2\n'/\nF2: true", expected: 7},
		{s: "F1:\n'\nline1\n'\nF2: true", expected: 5},
	}

	for _, test := range tests {
		s := S{}
		err := Unmarshal([]byte(test.s), &s)
		assert.ErrorIs(t, err, ErrDelimiterIsNotTypedForBool)
		assert.Equal(t, test.expected, errors.Unwrap(err).(*DecodeError).Line)
	}
}*/

func TestMarshalMultilineString(t *testing.T) {
	type S struct {
		Key1        string
		MulilineKey string
		Key2        string
	}

	tests := []struct {
		s        S
		expected string
	}{
		{s: S{MulilineKey: "line 1\nline 2"}, expected: "Key1: \nMulilineKey:\n'\nline 1\nline 2\n'\nKey2: \n"},
		{s: S{MulilineKey: "line 1\n'\nline 2"}, expected: "Key1: \nMulilineKey:\n''\nline 1\n'\nline 2\n''\nKey2: \n"},
	}

	for _, test := range tests {
		b, err := Marshal(test.s)
		assert.NoError(t, err)
		assert.Equal(t, test.expected, string(b))
	}
}

func TestUnmarshalMultilineString(t *testing.T) {
	type S struct {
		Key string
	}

	tests := []struct {
		str         string
		expected    S
		expectedErr error
	}{
		{str: "Key:\n'\nline 1\nline 2\n'\n", expected: S{Key: "line 1\nline 2"}, expectedErr: nil},
		{str: "Key:\n'\nline 1\nline 2\n\n'\n", expected: S{Key: "line 1\nline 2\n"}, expectedErr: nil},

		{str: "Key:\n'\nline 1\n''\nline 2\n'\n", expected: S{Key: "line 1\n''\nline 2"}, expectedErr: nil}, // valid mutliline string
		{str: "Key:\n'\nline 1\n''\nline 2\n'", expected: S{}, expectedErr: io.ErrUnexpectedEOF},            // no new line, must me io.ErrUnexpectedEOF

		{str: "Key:\n''\nline 1\nline 2\n'\n", expected: S{Key: ""}, expectedErr: io.ErrUnexpectedEOF},
		{str: "Key:\n'\nline 1\nline 2\n''\n", expected: S{Key: ""}, expectedErr: io.ErrUnexpectedEOF},
	}

	for _, test := range tests {
		s := S{}

		err := Unmarshal([]byte(test.str), &s)
		assert.ErrorIs(t, err, test.expectedErr)
		assert.Equal(t, test.expected, s)
	}
}

func TestMarshalUnmarshalMultilineString(t *testing.T) {
	type S struct {
		Key1        string
		MulilineKey string
		Key2        string
	}

	tests := []struct {
		s S
	}{
		{s: S{Key1: "Value1", MulilineKey: "Line1\n'\nLine2", Key2: "Value2"}},
	}

	for _, test := range tests {
		b, err := Marshal(test.s)
		assert.NoError(t, err)
		got := S{}
		err = Unmarshal(b, &got)
		assert.NoError(t, err)
		assert.Equal(t, test.s, got)
	}
}

func TestMarshalFieldTagFieldName(t *testing.T) {
	type S struct {
		Field string `uno:"altFieldName"`
	}

	expected := "altFieldName: value\n"

	s := S{Field: "value"}
	b, err := Marshal(s)
	assert.NoError(t, err)
	assert.Equal(t, expected, string(b))
}

func TestUnmarshalFieldTagFieldName(t *testing.T) {
	type S struct {
		Field string `uno:"altFieldName"`
	}

	str := "altFieldName: value\n"

	got := S{}
	err := Unmarshal([]byte(str), &got)
	assert.NoError(t, err)
	assert.Equal(t, S{Field: "value"}, got)
}

func TestMarshalUnmarshalTagFieldName(t *testing.T) {
	type S struct {
		Field string `uno:"altFieldName"`
	}

	s := S{Field: "value"}

	b, err := Marshal(s)
	assert.NoError(t, err)

	newS := S{}
	err = Unmarshal(b, &newS)
	assert.NoError(t, err)

	assert.Equal(t, s, newS)
}

func TestIgnoreFieldNameCase(t *testing.T) {
	type S struct {
		Field string
	}

	str := "field: value"

	dec := NewDecoder(strings.NewReader(str))
	dec.IgnoreFieldNameCase = true

	s := S{}
	err := dec.Decode(&s)
	assert.NoError(t, err)
	assert.Equal(t, S{Field: "value"}, s)
}

func TestIgnoreTagFieldNameCase(t *testing.T) {
	type S struct {
		Field string `uno:"altTagName"`
	}

	str := "alttagname: value"

	dec := NewDecoder(strings.NewReader(str))
	dec.IgnoreFieldNameCase = true

	s := S{}
	err := dec.Decode(&s)
	assert.NoError(t, err)
	assert.Equal(t, S{Field: "value"}, s)
}

func TestOmitEmptyTag(t *testing.T) {
	type S struct {
		EmptyString  string           `uno:",omitempty"`
		EmptyPointer *S               `uno:",omitempty"`
		EmptyMap     map[int]struct{} `uno:",omitempty"`
		EmptySlice   []string         `uno:",omitempty"`
		NotEmpty     string           `uno:",omitempty"`
		ZeroInt      int              `uno:",omitempty"`
	}

	s := S{
		EmptyString:  "",
		EmptyPointer: nil,
		EmptyMap:     nil,
		EmptySlice:   nil,
		NotEmpty:     "value"}

	expected := "NotEmpty: value\nZeroInt= 0\n"

	b, err := Marshal(s)
	assert.NoError(t, err)
	assert.Equal(t, expected, string(b))
}

func TestSkipFieldTag(t *testing.T) {
	type S struct {
		Skip    string `uno:"-"`
		NotSkip string
	}

	s := S{Skip: "Value", NotSkip: "Value"}

	expected := "NotSkip: Value\n"

	b, err := Marshal(s)
	assert.NoError(t, err)
	assert.Equal(t, expected, string(b))
}

func TestDecodeFile(t *testing.T) {
	type S struct {
		Key string
	}

	got := S{}

	err := DecodeFile("testdata/test_decode_file.uno", &got)
	assert.NoError(t, err)
	assert.Equal(t, S{Key: "Value"}, got)
}

func TestInvalidDecodeFile(t *testing.T) {
	type S struct {
		Key string
	}

	got := S{}

	err := DecodeFile("testdata/file_not_exists.uno", &got)
	assert.Error(t, err)
	assert.Equal(t, S{}, got)
}

func TestUnexportedField(t *testing.T) {
	type S struct {
		key string
	}

	str := "key: value"
	got := S{}
	err := Unmarshal([]byte(str), &got)
	assert.ErrorIs(t, err, ErrFieldIsNotExported)
}

func TestSkipNonExistentFields(t *testing.T) {
	type S struct {
		Key1 string
		Key2 string
		Key3 string
	}

	str := `Key1: value1
skip1: value
Key2: value2
skip2:
- 1
- 2
skip3:
	Key: value
Key3: value3
skip4= 1
`

	got := S{}
	dec := NewDecoder(strings.NewReader(str))
	err := dec.Decode(&got)
	assert.ErrorIs(t, err, ErrFieldIsNotExists)

	got = S{}
	dec = NewDecoder(strings.NewReader(str))
	dec.SkipNonExistentFields = true
	err = dec.Decode(&got)
	assert.NoError(t, err)
	assert.Equal(t, S{Key1: "value1", Key2: "value2", Key3: "value3"}, got)
}

func TestUnsupportedTypes(t *testing.T) {
	type S1 struct {
		Key complex64
	}

	type S2 struct {
		Key []complex64
	}

	tests := []any{
		S1{},
		S2{Key: []complex64{complex64(1)}}}

	for _, test := range tests {
		_, err := Marshal(test)
		assert.ErrorIs(t, err, ErrUnsupportedType)
	}
}

func TestInvalidMapIdent(t *testing.T) {
	type S struct {
		M map[string]int
	}

	str := `M:
	. Key1= 1
	. Key2= 2`

	s := S{}

	err := Unmarshal([]byte(str), &s)
	assert.ErrorIs(t, err, ErrInvalidIndentation)
}

func TestInvalidMultilineStringBoundary(t *testing.T) {
	str := `Key:
'a
text
'`

	var s struct{ Key string }

	err := Unmarshal([]byte(str), &s)
	assert.ErrorIs(t, err, ErrInvalidMultilineStringBoundary)
}

func TestInvalidMultilineCommentBoundary(t *testing.T) {
	str := `/'a
text
'/`

	var s struct{}

	err := Unmarshal([]byte(str), &s)
	assert.ErrorIs(t, err, ErrInvalidMultilineCommentBoundary)
}

func TestInvalidUnmarshalSliceToOtherType(t *testing.T) {
	tests := []struct {
		str         string
		expectedErr error
	}{
		{str: "Key:\n- 1", expectedErr: ErrTryingToDecodeSliceToNonSlice},
		{str: "Key:\n= 1", expectedErr: ErrTryingToDecodeSliceToNonSlice},
	}

	for _, test := range tests {
		s := struct{ Key any }{}

		err := Unmarshal([]byte(test.str), &s)
		assert.ErrorIs(t, err, test.expectedErr)
	}
}

func TestInvalidUnmarshalMapToOtherType(t *testing.T) {
	tests := []struct {
		str         string
		expectedErr error
	}{
		{str: "Key:\n. k1: 1", expectedErr: ErrTryingToDecodeMapToNonMap},
		{str: "Key:\n. k1= 1", expectedErr: ErrTryingToDecodeMapToNonMap},
	}

	for _, test := range tests {
		s := struct{ Key any }{}

		err := Unmarshal([]byte(test.str), &s)
		assert.ErrorIs(t, err, test.expectedErr)
	}
}

func TestInvalidSliceItemTypeStringToNonString(t *testing.T) {
	str := "Key:\n- 1\n= 2"

	s := struct{ Key []int }{}

	err := Unmarshal([]byte(str), &s)
	assert.ErrorIs(t, err, ErrInvalidSliceType)
}

func TestInvalidSliceItemTypeNontStringToString(t *testing.T) {
	str := `Key:
= 1
- 2`

	s := struct{ Key []string }{}

	err := Unmarshal([]byte(str), &s)
	assert.ErrorIs(t, err, ErrInvalidSliceType)
}

func TestUnmarshalToSliceOfPointers(t *testing.T) {
	str := "Key:\n- val1\n- val2\n"

	type S struct {
		Key []*string
	}

	v1 := "val1"
	v2 := "val2"

	expected := S{Key: []*string{&v1, &v2}}

	got := S{}
	err := Unmarshal([]byte(str), &got)
	assert.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestUnmarshalToNestedPointer(t *testing.T) {
	str := "Key= 2001-02-03"

	type S struct {
		Key **time.Time
	}

	expectedTime := time.Date(2001, 2, 3, 0, 0, 0, 0, time.UTC)
	ptr := &expectedTime
	expected := S{Key: &ptr}

	got := S{}

	err := Unmarshal([]byte(str), &got)
	assert.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestMarshalMetadata(t *testing.T) {
	str := "# MetadataField: metadata value\nField: field value\n# NoMetadata: value"

	type S struct {
		MetadataField string `uno:",metadata"`
		Field         string
		NoMetadata    string
	}

	expected := S{MetadataField: "metadata value", Field: "field value", NoMetadata: ""}

	got := S{}
	err := Unmarshal([]byte(str), &got)
	assert.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestUnmarshalMetadata(t *testing.T) {
	expected := "# MetadataField: metadata value\nField: field value\n"

	type S struct {
		MetadataField string `uno:",metadata"`
		Field         string
	}

	s := S{MetadataField: "metadata value", Field: "field value"}

	b, err := Marshal(s)
	assert.NoError(t, err)
	assert.Equal(t, expected, string(b))
}

func TestMarshalUnmarshalMetadata(t *testing.T) {
	type S struct {
		MetadataField string `uno:",metadata"`
		Field         string
	}

	expected := S{MetadataField: "metadata value", Field: "field value"}

	b, err := Marshal(expected)
	assert.NoError(t, err)

	got := S{}
	err = Unmarshal(b, &got)
	assert.NoError(t, err)

	assert.Equal(t, expected, got)
}

func TestErrInvalidTimeLayout(t *testing.T) {
	str := "Key= 99-01-01"

	s := struct{ Key time.Time }{}

	err := Unmarshal([]byte(str), &s)
	assert.ErrorIs(t, err, ErrInvalidTimeLayout)
}

func TestErrMapKeyIsNotString(t *testing.T) {
	str := "Key:\n. 1= 2"

	s := struct{ Key map[int]int }{}

	err := Unmarshal([]byte(str), &s)
	assert.ErrorIs(t, err, ErrMapKeyIsNotString)
}

func TestUnmarshalToPointerToStruct(t *testing.T) {
	type Nested2 struct {
		Key string
	}

	type Nested1 struct {
		Nested2 *Nested2
	}

	type S struct {
		Nested1 *Nested1
	}

	str := "Nested1:\n\tNested2:\n\t\tKey: value"
	expected := S{Nested1: &Nested1{&Nested2{Key: "value"}}}

	got := S{}

	err := Unmarshal([]byte(str), &got)
	assert.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestMarshalSliceOfObjects(t *testing.T) {
	type Object struct {
		Key1 string
		Key2 int
	}

	type S struct {
		Objects []Object
	}

	s := S{
		Objects: []Object{
			{Key1: "1", Key2: 2},
			{Key1: "3", Key2: 4}}}

	b, err := Marshal(s)
	assert.NoError(t, err)

	expected := "Objects:\n.\n\tKey1: 1\n\tKey2= 2\n.\n\tKey1: 3\n\tKey2= 4\n"
	assert.Equal(t, string(b), expected)
}

func TestUnmarshalSliceOfObjects(t *testing.T) {
	str := "Objects:\n.\n\tKey1: 1\n\tKey2= 2\n.\n\tKey1: 3\n\tKey2= 4\n"

	type Object struct {
		Key1 string
		Key2 int
	}

	type S struct {
		Objects []Object
	}

	expected := S{
		Objects: []Object{
			{Key1: "1", Key2: 2},
			{Key1: "3", Key2: 4}}}

	got := S{}
	err := Unmarshal([]byte(str), &got)
	assert.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestMarshalUnmarshalSliceOfObjects(t *testing.T) {
	type Object struct {
		Key1 string
		Key2 int
	}

	type S struct {
		Objects []Object
	}

	expected := S{
		Objects: []Object{
			{Key1: "1", Key2: 2},
			{Key1: "3", Key2: 4}}}

	b, err := Marshal(expected)
	assert.NoError(t, err)

	got := S{}
	err = Unmarshal(b, &got)

	assert.Equal(t, expected, got)
}

func TestMarshalUnmarshalSliceOfObjectsRaw(t *testing.T) {
	expected := map[string]any{
		"Objects": []map[string]any{
			{"Key1": "1", "Key2": "2"},
			{"Key1": "3", "Key2": "4"}}}

	b, err := Marshal(expected)
	assert.NoError(t, err)

	got := map[string]any{}
	err = Unmarshal(b, &got)
	assert.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestMarshalRaw(t *testing.T) {
	expected := "MapOfObjects:\n. Obj1:\n\t. Key: value\nNestedMap:\n. NestedMapKey: NestedMapValue\nSliceString:\n- 1\n- 2\n- 3\nString: string\n"

	s := map[string]any{
		"String":      "string",
		"SliceString": []string{"1", "2", "3"},
		"MapOfObjects": map[string]any{
			"Obj1": map[string]any{"Key": "value"}},
		"NestedMap": map[string]any{
			"NestedMapKey": "NestedMapValue"}}

	b, err := Marshal(s)
	assert.NoError(t, err)
	assert.Equal(t, expected, string(b))
}

func TestMarshalMapOfStructs(t *testing.T) {
	expected := `Map:
. Key1:
	Key1: 1
	Key2= 2
. Key2:
	Key1: 3
	Key2= 4
`

	type Nested struct {
		Key1 string
		Key2 int
	}

	type S struct {
		Map map[string]Nested
	}

	s := S{
		Map: map[string]Nested{
			"Key1": {Key1: "1", Key2: 2},
			"Key2": {Key1: "3", Key2: 4}}}
	got, err := Marshal(s)
	assert.NoError(t, err)
	assert.Equal(t, expected, string(got))
}
func TestUnmarshalMapOfStructs(t *testing.T) {
	str := `Map:
. Key1:
	NestedKey1: 1
	NestedKey2= 2
. Key2:
	NestedKey1: 3
	NestedKey2= 4
`

	type Nested struct {
		NestedKey1 string
		NestedKey2 int
	}

	type S struct {
		Map map[string]Nested
	}

	expected := S{
		Map: map[string]Nested{
			"Key1": {NestedKey1: "1", NestedKey2: 2},
			"Key2": {NestedKey1: "3", NestedKey2: 4}}}
	got := S{}

	err := Unmarshal([]byte(str), &got)
	assert.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestMarshalUnmarshalMapOfStructs(t *testing.T) {
	type Nested struct {
		Key1 string
		Key2 int
	}

	type S struct {
		Map map[string]Nested
	}

	expected := S{
		Map: map[string]Nested{
			"Key1": {Key1: "1", Key2: 2},
			"Key2": {Key1: "3", Key2: 4}}}

	b, err := Marshal(expected)
	assert.NoError(t, err)

	got := S{}
	err = Unmarshal(b, &got)
	assert.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestUnmarshalRaw(t *testing.T) {
	str := "Float64= 1\nString: string\nSliceString:\n- 1\n- 2\n- 3\nSliceTyped:\n= 4\n= 5\n= 6\nMapOfObjects:\n. Obj1:\n\tKey: value\nNestedMap:\n\tFloat64Slice:\n\t= 1\n\t= 2"

	expected := map[string]any{
		"Float64":     "1",
		"String":      "string",
		"SliceString": []string{"1", "2", "3"},
		"SliceTyped":  []string{"4", "5", "6"},
		"MapOfObjects": map[string]any{
			"Obj1": map[string]any{"Key": "value"}},
		"NestedMap": map[string]any{
			"Float64Slice": []string{"1", "2"}},
	}

	gotMap := map[string]any{}

	err := Unmarshal([]byte(str), &gotMap)
	assert.NoError(t, err)
	assert.Equal(t, expected, gotMap)
}

func TestMarshalUnmarshalRaw(t *testing.T) {
	expected := map[string]any{
		"String":      "string",
		"SliceString": []string{"1", "2", "3"},
		"MapOfObjects": map[string]any{
			"Obj1": map[string]any{"Key": "value"}},
		"NestedMap": map[string]any{
			"NestedMapKey": "NestedMapValue"},
	}

	b, err := Marshal(expected)
	assert.NoError(t, err)

	got := map[string]any{}
	err = Unmarshal(b, &got)
	assert.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestInvalidFieldOfString(t *testing.T) {
	type S struct {
		A struct {
			A1 string
		}
	}

	str := "A: 123\n\tA1: 456"

	err := Unmarshal([]byte(str), &S{})
	assert.ErrorIs(t, err, ErrUnsupportedType) // TODO: add proper error description

	got := map[string]any{}
	err = Unmarshal([]byte(str), &got)
	assert.ErrorIs(t, err, ErrInvalidIndentation)
}

func TestMultilineComment(t *testing.T) {
	type S struct {
		Key1 string
		Key2 string
	}

	tests := []struct {
		str           string
		expectedValue S
		expectedError error
	}{
		{str: "/'\nmulti\nline\ncomment\n'/\nKey1: Value1\nKey2: Value2", expectedValue: S{Key1: "Value1", Key2: "Value2"}, expectedError: nil},
		{str: "Key1: Value1\n/'\nmulti\nline\ncomment\n'/\nKey2: Value2", expectedValue: S{Key1: "Value1", Key2: "Value2"}, expectedError: nil},
		{str: "Key1: Value1\nKey2: Value2\n/'\nmulti\nline\ncomment\n'/\n", expectedValue: S{Key1: "Value1", Key2: "Value2"}, expectedError: nil},
		{str: "Key1: Value1\nKey2: Value2\n/'\nmulti\nline\ncomment\n", expectedValue: S{Key1: "Value1", Key2: "Value2"}, expectedError: io.ErrUnexpectedEOF},
	}

	for _, test := range tests {
		got := S{}
		err := Unmarshal([]byte(test.str), &got)
		assert.ErrorIs(t, err, test.expectedError)
		assert.Equal(t, test.expectedValue, got)
	}
}

/*func TestInvalidLines(t *testing.T) {
	tests := []string{
		"#",
		"# ",
	}

	for _, test := range tests {
		_, err := NewDecoder(strings.NewReader(test)).readRecord("")
		assert.Error(t, err)
	}
}*/

func TestStringTruncation(t *testing.T) {
	str := "Strings:\n- v1 \n-  v2  \n-   v3   "

	type S struct {
		Strings []string
	}

	expected := S{Strings: []string{"v1", "v2", "v3"}}

	got := S{}
	err := Unmarshal([]byte(str), &got)
	assert.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestInvalidMapKeyType(t *testing.T) {
	s := struct {
		Map map[int]string
	}{
		Map: map[int]string{1: "val1"}}

	_, err := Marshal(s)
	assert.Error(t, err)
}

func TestSkipBlock(t *testing.T) {
	s := `List1:
- 1
- 2
SkipList:
- 3
- 4
List3:
- 5
- 6
`

	type S struct {
		List1    []string
		SkipList []string `uno:"-"`
		List3    []string
	}

	expected := S{
		List1: []string{"1", "2"},
		List3: []string{"5", "6"},
	}

	got := S{}

	err := Unmarshal([]byte(s), &got)
	assert.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestMarshalInvalidMapKeyType(t *testing.T) {
	s := struct {
		F1 map[int]string
	}{}

	_, err := Marshal(s)
	assert.ErrorIs(t, err, ErrMapKeyIsNotString)
}

func TestUnmarshalMultilineStringRaw(t *testing.T) {
	str := "Key:\n'\nLine 1\nLine 2\n'\n"

	expected := map[string]any{"Key": "Line 1\nLine 2"}

	var got any
	err := Unmarshal([]byte(str), &got)
	assert.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestMarshalTypes(t *testing.T) {
	tests := []struct {
		s        any
		expected string
	}{
		{s: int(-1), expected: "-1"},
		{s: -0.2, expected: "-0.2"},
		{s: "string", expected: "string"},
		{s: true, expected: "true"},
		{s: time.Date(2006, 5, 4, 3, 2, 1, 0, time.UTC), expected: "2006-05-04T03:02:01Z"},
		{s: []string{"1", "2"}, expected: "- 1\n- 2\n"},
		{s: []int{1, 2}, expected: "= 1\n= 2\n"},
		{s: map[string]string{"Key1": "Val1", "Key2": "Val2"}, expected: ". Key1: Val1\n. Key2: Val2\n"},
	}

	for _, test := range tests {
		got, err := Marshal(test.s)
		assert.NoError(t, err)
		assert.Equal(t, test.expected, string(got))
	}
}

func TestUnmarshalTypes(t *testing.T) {
	tests := []struct {
		expected any
		str      string
	}{
		{expected: int(-1), str: "-1"},
		{expected: -0.2, str: "-0.2"},
		{expected: "string", str: "string"},
		{expected: true, str: "true"},
		{expected: time.Date(2006, 5, 4, 3, 2, 1, 0, time.UTC), str: "2006-05-04T03:02:01Z"},
		{expected: []string{"1", "2"}, str: "- 1\n- 2\n"},
		{expected: []int{1, 2}, str: "= 1\n= 2\n"},
		{expected: map[string]string{"Key1": "Val1", "Key2": "Val2"}, str: ". Key1: Val1\n. Key2: Val2\n"},
	}

	for _, test := range tests {
		got := reflect.Zero(reflect.TypeOf(test.expected)).Interface()

		err := Unmarshal([]byte(test.str), &got)
		assert.NoError(t, err)
		assert.Equal(t, test.expected, got)
	}
}

func TestRawMessageUnmarshal(t *testing.T) {
	str := "Key1: Value1\nKeyRaw:\n- 1\n- 2\nKey2: Value2"

	type S struct {
		Key1   string
		KeyRaw RawMessage
		Key2   string
	}

	expected := S{
		Key1:   "Value1",
		KeyRaw: RawMessage("- 1\n- 2\n"),
		Key2:   "Value2"}

	got := S{}

	err := Unmarshal([]byte(str), &got)
	assert.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestReadmeExampleMarshal(t *testing.T) {
	s := struct {
		A int
		B string
	}{
		A: 123,
		B: "456"}

	bytes, err := Marshal(s)
	assert.NoError(t, err)
	assert.Equal(t, "A= 123\nB: 456\n", string(bytes))
}

func TestReadmeExampleUnmarshal(t *testing.T) {
	bytes := []byte("A= 123\nB: 456\n")

	s := struct {
		A int
		B string
	}{}

	err := Unmarshal(bytes, &s)
	assert.NoError(t, err)
	assert.Equal(t, "{A:123 B:456}", fmt.Sprintf("%+v", s))
}

func TestReadmeExampleMarshalRaw(t *testing.T) {
	bytes := []byte("A= 123\nB:\n\tB1: 456\n\tB2: 789")

	var s map[string]any

	err := Unmarshal(bytes, &s)
	assert.NoError(t, err)
	assert.Equal(t, "map[A:123 B:map[B1:456 B2:789]]", fmt.Sprintf("%+v", s))
}

func TestEmptyStruct(t *testing.T) {
	str := `. Key1:
	- Value 1
	- Value 2
. EmptyKey:
. Key2:
	- Value 3
	- Value 4`

	var got map[string]any
	err := Unmarshal([]byte(str), &got)
	assert.NoError(t, err)

	expected := map[string]any{
		"Key1":     []string{"Value 1", "Value 2"},
		"EmptyKey": nil,
		"Key2":     []string{"Value 3", "Value 4"},
	}
	assert.Equal(t, expected, got)
}
