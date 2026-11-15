package uno

import (
	"bytes"
	"os"
)

func Unmarshal(in []byte, out any) error {
	return NewDecoder(bytes.NewReader(in)).Decode(out)
}

func Marshal(in any) (out []byte, err error) {
	buf := new(bytes.Buffer)

	err = NewEncoder(buf).Encode(in)

	return buf.Bytes(), err
}

func DecodeFile(filePath string, out any) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	return NewDecoder(f).Decode(&out)
}
