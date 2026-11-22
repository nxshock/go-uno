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

### Simple marshal/unmarshal

Marshal:

```go
s := struct {
	A int
	B string
}{
	A: 123,
	B: "456"}

bytes, err := uno.Marshal(s)
fmt.Println(string(bytes)) // "A= 123\nB: 456\n"
```

Unmarshal:

```go
bytes := []byte("A= 123\nB: 456\n")

s := struct {
	A int
	B string
}{}

err := uno.Unmarshal(bytes, &s)
fmt.Printf("%+v", s) // {A:123 B:456}
```

### Raw unmarshal

```go
bytes := []byte("A= 123\nB:\n\tB1: 789")

s := uno.Raw{}

err := uno.Unmarshal(bytes, &s)
fmt.Printf("%+v", s) // map[A:123 B:map[B1:456 B2:789]]
```
