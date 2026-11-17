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
		{s: "'': val", expected: record{Indent: 0, Directive: directiveNone, ItemMarker: itemMarkerNone, Key: "", Delimiter: delimiterString, Value: "val", lineNumber: 1}},
		{s: "key: val", expected: record{Indent: 0, Directive: directiveNone, ItemMarker: itemMarkerNone, Key: "key", Delimiter: delimiterString, Value: "val", lineNumber: 1}},
		{s: "n_key: val", expected: record{Indent: 0, Directive: directiveNone, ItemMarker: itemMarkerNone, Key: "n_key", Delimiter: delimiterString, Value: "val", lineNumber: 1}},
		{s: "n-key: val", expected: record{Indent: 0, Directive: directiveNone, ItemMarker: itemMarkerNone, Key: "n-key", Delimiter: delimiterString, Value: "val", lineNumber: 1}},
		{s: "-key: val", expected: record{Indent: 0, Directive: directiveNone, ItemMarker: itemMarkerNone, Key: "-key", Delimiter: delimiterString, Value: "val", lineNumber: 1}},
		{s: "n key: val", expected: record{Indent: 0, Directive: directiveNone, ItemMarker: itemMarkerNone, Key: "n key", Delimiter: delimiterString, Value: "val", lineNumber: 1}},
		{s: "1234: val", expected: record{Indent: 0, Directive: directiveNone, ItemMarker: itemMarkerNone, Key: "1234", Delimiter: delimiterString, Value: "val", lineNumber: 1}},
	}

	for _, test := range tests {
		got, err := parseRecord(test.s, "", 1)
		assert.NoError(t, err)
		assert.Equal(t, test.expected, *got)
	}
}

func TestListItem(t *testing.T) {
	tests := []struct {
		s        string
		expected record
	}{
		{s: "- text", expected: record{Indent: 0, Directive: directiveNone, ItemMarker: itemMarkerArrayString, Key: "", Delimiter: delimiterNone, Value: "text", lineNumber: 1}},
		{s: "= 129.56", expected: record{Indent: 0, Directive: directiveNone, ItemMarker: itemMarkerArrayTyped, Key: "", Delimiter: delimiterNone, Value: "129.56", lineNumber: 1}},
	}

	for _, test := range tests {
		got, err := parseRecord(test.s, "", 1)
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
		{s: "key: '", expected: "'"},
	}

	for _, test := range tests {
		row, err := parseRecord(test.s, "", 1)
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
		row, err := parseRecord(test.s, "", 1)
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
		row, err := parseRecord(test.s, "", 1)
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
		row, err := parseRecord(test.s, "", 1)
		assert.NoError(t, err)
		v, err := row.decodeValue(reflect.TypeOf(float64(0)))
		assert.NoError(t, err)
		assert.True(t, math.IsInf(v.(float64), test.sign))
	}

	row, err := parseRecord("key= nan", "", 1)
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
		Value:      "",
		lineNumber: 1}

	row, err := parseRecord(s, "", 1)
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
		_, err := parseRecord(test.s, "", 1)
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

func TestErrLineNumbers(t *testing.T) {
	type S struct {
		F1 string
		F2 bool
	}

	str := `F1: value1
F2: true`

	s := S{}
	err := Unmarshal([]byte(str), &s)
	assert.ErrorIs(t, err, ErrDelimiterIsNotTypedForBool)
	assert.True(t, strings.HasPrefix(err.Error(), "line 2: "))
}

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
		str      string
		expected S
	}{
		{str: "Key:\n'\nline 1\nline 2\n'", expected: S{Key: "line 1\nline 2"}},
		{str: "Key:\n'\nline 1\nline 2\n\n'", expected: S{Key: "line 1\nline 2\n"}},
	}

	for _, test := range tests {
		s := S{}

		err := Unmarshal([]byte(test.str), &s)
		assert.NoError(t, err)
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

	str := "altFieldName: value"

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

func TestUnexportedField(t *testing.T) {
	type S struct {
		key string
	}

	str := `key: value`
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

	dec.SkipNonExistentFields = true
	err = dec.Decode(&got)
	assert.NoError(t, err)
	assert.Equal(t, S{Key1: "value1", Key2: "value2", Key3: "value3"}, got)
}

func TestUnsupportedTypes(t *testing.T) {
	type S struct {
		Key complex64
	}

	s := S{}

	_, err := Marshal(s)
	assert.ErrorIs(t, err, ErrUnsupportedType)
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
