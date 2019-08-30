package log

import (
	"fmt"
	"strings"

	"go.uber.org/zap/zapcore"
)

const (
	EncodingJSON    Encoding = "json"
	EncodingConsole Encoding = "console"
)

var (
	SupportedEncodings = Encodings{
		EncodingJSON,
		EncodingConsole,
	}
)

type Encoding string

func (e Encoding) String() string {
	return string(e)
}

func (e *Encoding) Set(s string) error {
	*e = Encoding(s)
	return nil
}

type CreateEncoderFunc func(config zapcore.EncoderConfig) zapcore.Encoder

type Encodings []Encoding

func (e Encodings) String() string {
	const separator = ", "
	var s string
	for _, encoding := range e {
		s = s + separator + string(encoding)
	}
	return strings.TrimPrefix(s, separator)
}

func (e Encodings) Contains(s Encoding) bool {
	for _, encoding := range e {
		if encoding == s {
			return true
		}
	}
	return false
}

type InvalidEncodingError struct {
	encoding Encoding
}

func (e InvalidEncodingError) Error() string {
	return fmt.Sprintf("invalid encoding specified(%s). Supported encodings: %s", e.encoding, SupportedEncodings.String())
}
