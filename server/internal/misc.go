// SPDX-License-Identifier: AGPL-3.0-or-later

package internal

import (
	"encoding"
	"fmt"
	"sync"
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

var OnTerm = struct {
	sync.RWMutex

	Closed chan struct{}
	ToDo   []func()
}{Closed: make(chan struct{})}
