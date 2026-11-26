package uno

import (
	"bytes"
	"strings"
)

func tokenSplitFunc(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	idx := -1

	lastMatchedTokenLength := 0
	for _, ch := range [][]byte{
		{' '},           // spaces
		{'\n'},          // new lines
		[]byte("/'\n"),  // multiline comment opening boundary
		[]byte("'/\n"),  // multiline comment closing boundary
		[]byte("'\n")} { // multiline string boundary

		// pick first longest token
		if i := bytes.Index(data, ch); i >= 0 {
			if idx == -1 || (i <= idx && len(ch) >= lastMatchedTokenLength) {
				idx = i
				lastMatchedTokenLength = len(ch)
			}
		}
	}

	if idx >= 0 {
		if data[0] == '\n' {
			return idx + lastMatchedTokenLength, data[:lastMatchedTokenLength], nil
		} else {
			return idx + lastMatchedTokenLength, data[0 : idx+lastMatchedTokenLength], nil
		}
	}

	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), data, nil
	}
	// Request more data.
	return 0, nil, nil
}

func countIndentLevel(s string) uint {
	indent := uint(0)

	for _, c := range s {
		if c == '\t' {
			indent++
		} else {
			break
		}
	}

	return indent
}

func validMultilineBoundary(s string) bool {
	n := 0
	for _, v := range s {
		if v == '\'' {
			n++
		}
	}

	return n > 0 && strings.Repeat("'", n)+"\n" == s
}
