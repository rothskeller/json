package json

import (
	"bufio"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// A Handlers structure defines how parsed JSON is handled.  Any field which is
// nil implies that JSON element is not valid in the current parsing context.
type Handlers struct {
	// Null is called when a JSON null is encountered.
	Null func() error
	// Int is called when a JSON number without a fractional part is
	// encountered.
	Int func(int) error
	// Float is called when a JSON number is encountered (unless Int is
	// non-nil and the number has no fractional part).
	Float func(float64) error
	// String is called when a JSON string is encountered (unless Time is
	// non-nil and the string looks like an RFC3339 timestamp).
	String func(string) error
	// Time is called when a JSON string is encountered that can be parsed
	// as an RFC3339 timestamp.
	Time func(time.Time) error
	// Bool is called when a JSON boolean is encountered.
	Bool func(bool) error
	// Object is called when a JSON object is encountered.
	Object func() KeyHandler
	// Array is called when a JSON array is encountered.  It should return
	// the handlers used to parse the array elements.
	Array func() Handlers
}

func (h Handlers) empty() bool {
	return h.Null == nil && h.Int == nil && h.Float == nil &&
		h.String == nil && h.Time == nil && h.Bool == nil &&
		h.Object == nil && h.Array == nil
}

// A KeyHandler is a function that maps an object key to the handlers used to
// parse its value.
type KeyHandler func(string) Handlers

// Parse reads the input stream until EOF, and uses the supplied handlers to
// parse it.  It returns an error if it hits a JSON syntax error, if it hits a
// JSON element for which no handler was provided, or if a handler returns an
// error.
func Parse(reader io.Reader, handlers Handlers) (err error) {
	var (
		br *bufio.Reader
		r  rune
	)
	br = bufio.NewReader(reader)
	if err = parseOne(br, handlers); err != nil {
		return err
	}
	for {
		r, _, err = br.ReadRune()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if !unicode.IsSpace(r) {
			return errors.New("extra text after JSON")
		}
	}
}

func parseOne(reader *bufio.Reader, handlers Handlers) (err error) {
	var r rune

	skipWhitespace(reader)
	if r, _, err = reader.ReadRune(); err != nil {
		return err
	}
	switch {
	case r == '{':
		return parseObject(reader, handlers)
	case r == '[':
		return parseArray(reader, handlers)
	case r >= '0' && r <= '9', r == '-':
		reader.UnreadRune()
		return parseNumber(reader, handlers)
	case r == 't', r == 'f', r == 'n':
		reader.UnreadRune()
		return parseKeyword(reader, handlers)
	case r == '"':
		return parseString(reader, handlers)
	default:
		return errors.New("JSON syntax error")
	}
}

func skipWhitespace(reader *bufio.Reader) {
	for {
		r, _, err := reader.ReadRune()
		if err != nil {
			return
		}
		if r != ' ' && r != '\t' && r != '\r' && r != '\n' {
			reader.UnreadRune()
			return
		}
	}
}

func parseObject(reader *bufio.Reader, handlers Handlers) (err error) {
	var (
		hf        KeyHandler
		vhandlers Handlers
		r         rune
	)
	if handlers.Object == nil {
		return errors.New("unexpected '{' in JSON")
	}
	hf = handlers.Object()
	skipWhitespace(reader)
	if r, _, err = reader.ReadRune(); err != nil {
		return err
	}
	if r == '}' {
		return nil
	}
	reader.UnreadRune()
	for {
		if err = parseOne(reader, Handlers{String: func(key string) error {
			vhandlers = hf(key)
			if vhandlers.empty() {
				return errors.New("unexpected key \"" + key + "\" in JSON")
			}
			skipWhitespace(reader)
			if r, _, err = reader.ReadRune(); err != nil {
				return err
			}
			if r != ':' {
				return errors.New("expected ':' in JSON")
			}
			if err = parseOne(reader, vhandlers); err != nil {
				return err
			}
			return nil
		}}); err != nil {
			return err
		}
		skipWhitespace(reader)
		if r, _, err = reader.ReadRune(); err != nil {
			return err
		}
		if r == '}' {
			return nil
		}
		if r != ',' {
			return errors.New("expected '}' or ',' in JSON")
		}
	}
}

func parseArray(reader *bufio.Reader, handlers Handlers) (err error) {
	var (
		vhandlers Handlers
		r         rune
	)
	if handlers.Array == nil {
		return errors.New("unexpected '[' in JSON")
	}
	vhandlers = handlers.Array()
	skipWhitespace(reader)
	if r, _, err = reader.ReadRune(); err != nil {
		return err
	}
	if r == ']' {
		return nil
	}
	reader.UnreadRune()
	for {
		if err = parseOne(reader, vhandlers); err != nil {
			return err
		}
		skipWhitespace(reader)
		if r, _, err = reader.ReadRune(); err != nil {
			return err
		}
		if r == ']' {
			return nil
		}
		if r != ',' {
			return errors.New("expected ']' or ',' in JSON")
		}
	}
}

func parseNumber(reader *bufio.Reader, handlers Handlers) (err error) {
	var (
		buf [64]byte
		b   byte
		num = buf[:0]
	)
	for {
		b, err = reader.ReadByte()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if strings.IndexByte("0123456789-+eE.", b) < 0 {
			reader.UnreadByte()
			break
		}
		num = append(num, b)
	}
	if handlers.Int != nil {
		if i, err := strconv.Atoi(string(num)); err == nil {
			return handlers.Int(i)
		}
	}
	if handlers.Float != nil {
		if f, err := strconv.ParseFloat(string(num), 64); err == nil {
			return handlers.Float(f)
		}
		return errors.New("invalid JSON number")
	}
	return errors.New("unexpected number in JSON")
}

func parseKeyword(reader *bufio.Reader, handlers Handlers) (err error) {
	var (
		buf [5]byte
		b   byte
		kw  = buf[:0]
	)
	for {
		b, err = reader.ReadByte()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if strings.IndexByte("aeflnrstu", b) < 0 {
			reader.UnreadByte()
			break
		}
		kw = append(kw, b)
	}
	switch string(kw) {
	case "true":
		if handlers.Bool != nil {
			return handlers.Bool(true)
		}
		return errors.New("unexpected Boolean in JSON")
	case "false":
		if handlers.Bool != nil {
			return handlers.Bool(false)
		}
		return errors.New("unexpected Boolean in JSON")
	case "null":
		if handlers.Null != nil {
			return handlers.Null()
		}
		return errors.New("unexpected null in JSON")
	}
	return errors.New("unquoted string in JSON")
}

func parseString(reader *bufio.Reader, handlers Handlers) (err error) {
	var (
		sb strings.Builder
		s  string
		r  rune
		h  [4]byte
	)
	for {
		if r, _, err = reader.ReadRune(); err != nil {
			return err
		}
		if r == '"' {
			break
		}
		if r < 32 {
			return errors.New("unexpected control character in JSON string")
		}
		if r != '\\' {
			sb.WriteRune(r)
			continue
		}
		if r, _, err = reader.ReadRune(); err != nil {
			return err
		}
		switch r {
		case '"', '\\', '/':
			sb.WriteRune(r)
		case 'b':
			sb.WriteByte('\b')
		case 'f':
			sb.WriteByte('\f')
		case 'n':
			sb.WriteByte('\n')
		case 'r':
			sb.WriteByte('\r')
		case 't':
			sb.WriteByte('\t')
		case 'u':
			if n, err := reader.Read(h[:]); err != nil {
				return err
			} else if n != 4 {
				return errors.New("invalid Unicode escape in JSON string")
			}
			if i, err := strconv.ParseInt(string(h[:]), 16, 32); err != nil {
				return errors.New("invalid Unicode escape in JSON string")
			} else {
				sb.WriteRune(rune(i))
			}
		default:
			return errors.New("unexpected escape sequence in JSON string")
		}
	}
	s = sb.String()
	if handlers.Time != nil {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return handlers.Time(t)
		}
	}
	if handlers.String != nil {
		return handlers.String(s)
	}
	return errors.New("unexpected string in JSON")
}
