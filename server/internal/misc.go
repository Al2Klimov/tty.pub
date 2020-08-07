package internal

import (
	"encoding"
	"fmt"
)

type LoggableStringer struct {
	Str fmt.Stringer
}

var _ fmt.Stringer = LoggableStringer{}
var _ encoding.TextMarshaler = LoggableStringer{}

func (ls LoggableStringer) String() string {
	return ls.Str.String()
}

func (ls LoggableStringer) MarshalText() ([]byte, error) {
	return []byte(ls.String()), nil
}

type LoggableError struct {
	Err error
}

var _ fmt.Stringer = LoggableError{}
var _ encoding.TextMarshaler = LoggableError{}

func (le LoggableError) String() string {
	return le.Err.Error()
}

func (le LoggableError) MarshalText() ([]byte, error) {
	return []byte(le.String()), nil
}
