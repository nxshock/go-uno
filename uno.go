package uno

import (
	"bytes"
)

func Unmarshal(in []byte, out any) error {
	return NewDecoder(bytes.NewReader(in)).Decode(out)
}

func Marshal(in any) (out []byte, err error) {
	buf := new(bytes.Buffer)

	err = NewEncoder(buf).Encode(in)

	return buf.Bytes(), nil
}
