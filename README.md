# UNO support for the Go language

UNO stands from UNderstandable UNiversal NOtation of Objects. This Go package provides a reflection interface similar to
Go's standard library `json` package.

This package currenlty targets UNO Core Specification: https://uno.mudrium.ru/uno_core.html.

> [!CAUTION]
> Early alpha stage, use with caution.

## Installation

```sh
go get github.com/nxshock/go-uno
```

## Usage

### Simple encode/decode

Encode:

```go
var s = struct {
	A int
	B string
}{
	A: 123,
	B: "456"}

bytes, err := uno.Marshal(s)
fmt.Println(string(bytes)) // "A= 123\nB: 456\n"
```

Decode:

```go
bytes := []byte("A= 123\nB: 456\n")

var s = struct {
	A int
	B string
}{}

err := uno.Unmarshal(bytes, &s)
fmt.Printf("%+v", s)
```
