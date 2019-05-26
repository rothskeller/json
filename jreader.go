package json

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// A Handlers structure defines how parsed JSON is handled.  Any field which is
// nil implies that JSON element is not valid in the current parsing context.
type Handlers struct {
	// Ignore causes the value to be ignored.  None of the rest of the
	// Handlers fields are meaningful when this is set.
	Ignore bool
	// Null is called when a JSON null is encountered.
	Null func()
	// Int is called when a JSON number without a fractional part is
	// encountered.
	Int func(int)
	// Float is called when a JSON number is encountered (unless Int is
	// non-nil and the number has no fractional part).
	Float func(float64)
	// String is called when a JSON string is encountered (unless Time is
	// non-nil and the string looks like an RFC3339 timestamp).
	String func(string)
	// Time is called when a JSON string is encountered that can be parsed
	// as an RFC3339 timestamp.
	Time func(time.Time)
	// Bool is called when a JSON boolean is encountered.
	Bool func(bool)
	// Object is called when a JSON object is encountered.
	Object func(key string) Handlers
	// Array is called when a JSON array is encountered.  It should return
	// the handlers used to parse the array elements.
	Array func() Handlers
}

func (h Handlers) empty() bool {
	return !h.Ignore && h.Null == nil && h.Int == nil && h.Float == nil &&
		h.String == nil && h.Time == nil && h.Bool == nil &&
		h.Object == nil && h.Array == nil
}

// NewReader returns a Reader reading the provided stream.
func NewReader(reader io.Reader) *Reader {
	return &Reader{r: bufio.NewReader(reader), line: 1, col: 1}
}

// A Reader is a JSON stream reader and parser.
type Reader struct {
	r    *bufio.Reader
	line int
	col  int
	err  error
}

// Raise raises an error in the reader, causing its Read method to return the
// error.
func (h *Reader) Raise(err string) {
	if h.err == nil {
		h.err = fmt.Errorf("%s at %d:%d", err, h.line, h.col)
	}
}

// Read reads the input stream until EOF, and uses the supplied handlers to
// parse it.  It returns an error if it hits a JSON syntax error, if it hits a
// JSON element for which no handler was provided, or if a handler called Raise.
func (h *Reader) Read(handlers Handlers) (err error) {
	var (
		r rune
	)
	h.parseOne(handlers)
	if h.err != nil {
		return h.err
	}
	for {
		if r = h.readRune(true); r == 0 {
			return h.err
		}
		if r != ' ' && r != '\t' && r != '\r' && r != '\n' {
			h.Raise("extra text after JSON")
			break
		}
	}
	return h.err
}

func (h *Reader) readRune(allowEOF bool) (r rune) {
	var err error

	r, _, err = h.r.ReadRune()
	if err == io.EOF && allowEOF {
		return 0
	}
	if err != nil {
		h.err = err
		return 0
	}
	if r == '\n' {
		h.line++
		h.col = 1
	} else {
		h.col++
	}
	return r
}

func (h *Reader) unreadRune() {
	h.r.UnreadRune()
	h.col--
}

func (h *Reader) skipWhitespace() {
	for {
		switch r := h.readRune(false); r {
		case 0:
			return
		case ' ', '\t', '\r', '\n':
			h.unreadRune()
			return
		}
	}
}

func (h *Reader) parseOne(handlers Handlers) {
	var r rune

	h.skipWhitespace()
	r = h.readRune(false)
	switch {
	case r == 0:
		break
	case r == '{':
		h.parseObject(handlers)
	case r == '[':
		h.parseArray(handlers)
	case r >= '0' && r <= '9', r == '-':
		h.unreadRune()
		h.parseNumber(handlers)
	case r == 't', r == 'f', r == 'n':
		h.unreadRune()
		h.parseKeyword(handlers)
	case r == '"':
		h.parseString(handlers)
	default:
		h.Raise("syntax error in JSON")
	}
}

func (h *Reader) parseObject(handlers Handlers) {
	var (
		vhandlers Handlers
		r         rune
	)
	if handlers.Object == nil && !handlers.Ignore {
		h.Raise("unexpected '{' in JSON")
		return
	}
	h.skipWhitespace()
	if r = h.readRune(false); r == 0 || r == '}' {
		return
	}
	h.unreadRune()
	for h.err == nil {
		h.parseOne(Handlers{String: func(key string) {
			if handlers.Ignore {
				vhandlers = handlers
			} else {
				vhandlers = handlers.Object(key)
			}
			if vhandlers.empty() {
				h.Raise("unexpected key \"" + key + "\" in JSON")
				return
			}
			h.skipWhitespace()
			if r = h.readRune(false); r == 0 {
				return
			}
			if r != ':' {
				h.Raise("expected ':' in JSON")
				return
			}
			h.parseOne(vhandlers)
			if h.err != nil {
				return
			}
		}})
		if h.err != nil {
			return
		}
		h.skipWhitespace()
		if r = h.readRune(false); r == 0 || r == '}' {
			return
		}
		if r != ',' {
			h.Raise("expected '}' or ',' in JSON")
			return
		}
	}
}

func (h *Reader) parseArray(handlers Handlers) {
	var (
		vhandlers Handlers
		r         rune
	)
	if handlers.Array == nil && !handlers.Ignore {
		h.Raise("unexpected '[' in JSON")
		return
	}
	if handlers.Ignore {
		vhandlers = handlers
	} else {
		vhandlers = handlers.Array()
	}
	h.skipWhitespace()
	if r = h.readRune(false); r == 0 || r == ']' {
		return
	}
	h.unreadRune()
	for {
		h.parseOne(vhandlers)
		if h.err != nil {
			return
		}
		h.skipWhitespace()
		if r = h.readRune(false); r == 0 || r == ']' {
			return
		}
		if r != ',' {
			h.Raise("expected ']' or ',' in JSON")
			return
		}
	}
}

func (h *Reader) parseNumber(handlers Handlers) {
	var (
		buf [64]byte
		r   rune
		num = buf[:0]
	)
	for {
		if r = h.readRune(true); r == 0 {
			break
		}
		if strings.IndexRune("0123456789-+eE.", r) < 0 {
			h.unreadRune()
			break
		}
		num = append(num, byte(r))
	}
	if h.err != nil || handlers.Ignore {
		return
	}
	if handlers.Int != nil {
		if i, err := strconv.Atoi(string(num)); err == nil {
			handlers.Int(i)
			return
		} else if handlers.Float == nil {
			h.Raise("JSON number is not an integer")
			return
		}
	}
	if handlers.Float != nil {
		if f, err := strconv.ParseFloat(string(num), 64); err == nil {
			handlers.Float(f)
			return
		}
		h.Raise("invalid JSON number")
		return
	}
	h.Raise("unexpected number in JSON")
}

func (h *Reader) parseKeyword(handlers Handlers) {
	var (
		buf [5]byte
		r   rune
		kw  = buf[:0]
	)
	for {
		if r = h.readRune(true); r == 0 {
			break
		}
		if strings.IndexRune("aeflnrstu", r) < 0 {
			h.unreadRune()
			break
		}
		kw = append(kw, byte(r))
	}
	switch string(kw) {
	case "true":
		if handlers.Ignore {
			return
		}
		if handlers.Bool != nil {
			handlers.Bool(true)
			return
		}
		h.Raise("unexpected Boolean in JSON")
		return
	case "false":
		if handlers.Ignore {
			return
		}
		if handlers.Bool != nil {
			handlers.Bool(false)
			return
		}
		h.Raise("unexpected Boolean in JSON")
		return
	case "null":
		if handlers.Ignore {
			return
		}
		if handlers.Null != nil {
			handlers.Null()
			return
		}
		h.Raise("unexpected null in JSON")
		return
	}
	h.Raise("unquoted string in JSON")
}

func (h *Reader) parseString(handlers Handlers) {
	var (
		sb strings.Builder
		s  string
		r  rune
		u  [4]byte
	)
	for {
		if r = h.readRune(false); r == 0 {
			return
		}
		if r == '"' {
			break
		}
		if r < 32 {
			h.Raise("unexpected control character in JSON string")
			return
		}
		if r != '\\' {
			sb.WriteRune(r)
			continue
		}
		if r = h.readRune(false); r == 0 {
			return
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
			if n, err := h.r.Read(u[:]); err != nil {
				h.err = err
				return
			} else if n != 4 {
				h.Raise("invalid Unicode escape in JSON string")
				return
			}
			h.col += 4
			if i, err := strconv.ParseInt(string(u[:]), 16, 32); err != nil {
				h.Raise("invalid Unicode escape in JSON string")
				return
			} else {
				sb.WriteRune(rune(i))
			}
		default:
			h.Raise("unexpected escape sequence in JSON string")
			return
		}
	}
	s = sb.String()
	if handlers.Time != nil {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			handlers.Time(t)
			return
		} else if handlers.String == nil {
			h.Raise("invalid time string in JSON")
			return
		}
	}
	if handlers.String != nil {
		handlers.String(s)
		return
	}
	h.Raise("unexpected string in JSON")
}

// Shortcut functions to generate handlers for common cases.
func RejectHandler() Handlers                        { return Handlers{} }
func IgnoreHandler() Handlers                        { return Handlers{Ignore: true} }
func NullHandler(f func()) Handlers                  { return Handlers{Null: f} }
func IntHandler(f func(int)) Handlers                { return Handlers{Int: f} }
func FloatHandler(f func(float64)) Handlers          { return Handlers{Float: f} }
func StringHandler(f func(string)) Handlers          { return Handlers{String: f} }
func TimeHandler(f func(time.Time)) Handlers         { return Handlers{Time: f} }
func BoolHandler(f func(bool)) Handlers              { return Handlers{Bool: f} }
func ObjectHandler(f func(string) Handlers) Handlers { return Handlers{Object: f} }
func ArrayHandler(f func() Handlers) Handlers        { return Handlers{Array: f} }
func IntNullHandler(f func(int)) Handlers            { return Handlers{Int: f, Null: func() { f(0) }} }
func FloatNullHandler(f func(float64)) Handlers      { return Handlers{Float: f, Null: func() { f(0.0) }} }
func StringNullHandler(f func(string)) Handlers      { return Handlers{String: f, Null: func() { f("") }} }
func TimeNullHandler(f func(time.Time)) Handlers {
	return Handlers{Time: f, Null: func() { f(time.Time{}) }}
}
func BoolNullHandler(f func(bool)) Handlers              { return Handlers{Bool: f, Null: func() { f(false) }} }
func ObjectNullHandler(f func(string) Handlers) Handlers { return Handlers{Object: f, Null: func() {}} }
func ArrayNullHandler(f func() Handlers) Handlers {
	return Handlers{Array: f, Null: func() {}}
}
