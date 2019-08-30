package log

import (
	"flag"
)

func EncodingFlag(name string, defaultEncoding Encoding, usage string) *Encoding {
	e := defaultEncoding
	flag.Var(&e, name, usage)
	return &e
}
