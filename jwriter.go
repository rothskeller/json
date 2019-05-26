package json

import (
	"bufio"
	"fmt"
	"io"
	"sync"
	"unicode/utf8"
)

var (
	openBrace      byte = '{'
	closeBrace     byte = '}'
	comma          byte = ','
	colon          byte = ':'
	openBracket    byte = '['
	closeBracket   byte = ']'
	dquote         byte = '"'
	backslash      byte = '\\'
	escapedNL           = "\\n"
	escapedCR           = "\\r"
	escapedTab          = "\\t"
	escapedUnicode      = "\\u00"
	badUnicode          = "\\ufffd"
	null                = "null"
)

var writerPool sync.Pool

// NewWriter creates a new Writer for generating JSON output.
func NewWriter(w io.Writer) Writer {
	var jw *writer
	if jwi := writerPool.Get(); jwi != nil {
		jw = jwi.(*writer)
		jw.w.Reset(w)
		jw.comma = false
		jw.inObject = false
	} else {
		jw = &writer{w: bufio.NewWriter(w)}
	}
	return jw
}

// Writer is a JSON writer.
type Writer interface {
	Close()
	Object(f func())
	Prop(name string, value interface{})
	Array(f func())
	Null()
	String(s string)
	Int(i int)
	Bool(b bool)
	Raw(s string)
	RawByte(b byte)
}

// writer is the base implementation of Writer.
type writer struct {
	w        *bufio.Writer
	comma    bool
	inObject bool
}

// Close flushes the JSON output.  The Writer must not be used again after Close
// is called.
func (jw *writer) Close() {
	jw.w.Flush()
	writerPool.Put(jw)
}

// Object writes an object to the JSON output.  The properties of the object are
// given by Prop calls made in the supplied function.
func (jw *writer) Object(f func()) {
	if jw.inObject {
		panic("Object can only contain Prop")
	}
	if jw.comma {
		jw.w.WriteByte(comma)
	}
	saveInObject := jw.inObject
	jw.comma = false
	jw.inObject = true
	jw.w.WriteByte(openBrace)
	f()
	jw.w.WriteByte(closeBrace)
	jw.comma = true
	jw.inObject = saveInObject
}

// Prop writes a property to an object definition in the JSON output.  The value
// may be either nil, a string, or a function that uses Writer calls to render
// the value.
func (jw *writer) Prop(name string, value interface{}) {
	if !jw.inObject {
		panic("Prop can only occur within Object")
	}
	jw.inObject = false
	jw.String(name)
	jw.w.WriteByte(colon)
	jw.comma = false
	switch v := value.(type) {
	case nil:
		jw.Null()
	case string:
		jw.String(v)
	case int:
		jw.Int(v)
	case bool:
		jw.Bool(v)
	case float64:
		jw.Float64(v)
	case func():
		v()
		if !jw.comma {
			panic("Prop value function did not write anything")
		}
	default:
		panic("unknown Prop value type")
	}
	jw.comma = true
	jw.inObject = true
}

// Array writes an array to the JSON output.  The contents of the array are
// given by the Writer calls made in the supplied function.
func (jw *writer) Array(f func()) {
	if jw.inObject {
		panic("Object can only contain Prop")
	}
	if jw.comma {
		jw.w.WriteByte(comma)
	}
	jw.comma = false
	jw.w.WriteByte(openBracket)
	f()
	jw.w.WriteByte(closeBracket)
	jw.comma = true
}

// Null writes a null to the JSON output.
func (jw *writer) Null() {
	if jw.inObject {
		panic("Object can only contain Prop")
	}
	if jw.comma {
		jw.w.WriteByte(comma)
	}
	jw.comma = true
	jw.w.WriteString(null)
}

// Int writes an integer to the JSON output.
func (jw *writer) Int(i int) {
	if jw.inObject {
		panic("Object can only contain Prop")
	}
	if jw.comma {
		jw.w.WriteByte(comma)
	}
	jw.comma = true
	fmt.Fprintf(jw.w, "%d", i)
}

// Float64 writes a float64 to the JSON output.
func (jw *writer) Float64(f float64) {
	if jw.inObject {
		panic("Object can only contain Prop")
	}
	if jw.comma {
		jw.w.WriteByte(comma)
	}
	jw.comma = true
	fmt.Fprintf(jw.w, "%f", f)
}

// Bool writes a boolean to the JSON output.
func (jw *writer) Bool(b bool) {
	if jw.inObject {
		panic("Object can only contain Prop")
	}
	if jw.comma {
		jw.w.WriteByte(comma)
	}
	jw.comma = true
	if b {
		jw.w.WriteString("true")
	} else {
		jw.w.WriteString("false")
	}
}

// Raw writes a string to the JSON output without encoding.
func (jw *writer) Raw(s string) {
	jw.w.WriteString(s)
	jw.comma = false
}

// RawByte writes a byte to the JSON output without encoding.
func (jw *writer) RawByte(b byte) {
	jw.w.WriteByte(b)
	jw.comma = false
}

// Everything from here on down was copied and modified from the code in the
// standard encoding/json library.

// String writes a quoted string to the JSON output, with appropriate escaping.
func (jw *writer) String(s string) {
	if jw.inObject {
		panic("Object can only contain Prop")
	}
	if jw.comma {
		jw.w.WriteByte(comma)
	}
	jw.comma = true
	jw.w.WriteByte(dquote)
	start := 0
	for i := 0; i < len(s); {
		if b := s[i]; b < utf8.RuneSelf {
			if safeSet[b] {
				i++
				continue
			}
			if start < i {
				jw.w.WriteString(s[start:i])
			}
			switch b {
			case '\\', '"':
				jw.w.WriteByte(backslash)
				jw.w.WriteByte(b)
			case '\n':
				jw.w.WriteString(escapedNL)
			case '\r':
				jw.w.WriteString(escapedCR)
			case '\t':
				jw.w.WriteString(escapedTab)
			default:
				// This encodes bytes < 0x20 except for \t, \n and \r.
				// If escapeHTML is set, it also escapes <, >, and &
				// because they can lead to security holes when
				// user-controlled strings are rendered into JSON
				// and served to some browsers.
				jw.w.WriteString(escapedUnicode)
				jw.w.WriteByte(hex[b>>4])
				jw.w.WriteByte(hex[b&0xF])
			}
			i++
			start = i
			continue
		}
		c, size := utf8.DecodeRuneInString(s[i:])
		if c == utf8.RuneError && size == 1 {
			if start < i {
				jw.w.WriteString(s[start:i])
			}
			jw.w.WriteString(badUnicode)
			i += size
			start = i
			continue
		}
		i += size
	}
	if start < len(s) {
		jw.w.WriteString(s[start:])
	}
	jw.w.WriteByte(dquote)
}

var hex = "0123456789abcdef"

// safeSet holds the value true if the ASCII character with the given array
// position can be represented inside a JSON string without any further
// escaping.
//
// All values are true except for the ASCII control characters (0-31), the
// double quote ("), and the backslash character ("\").
var safeSet = [utf8.RuneSelf]bool{
	' ':      true,
	'!':      true,
	'"':      false,
	'#':      true,
	'$':      true,
	'%':      true,
	'&':      true,
	'\'':     true,
	'(':      true,
	')':      true,
	'*':      true,
	'+':      true,
	',':      true,
	'-':      true,
	'.':      true,
	'/':      true,
	'0':      true,
	'1':      true,
	'2':      true,
	'3':      true,
	'4':      true,
	'5':      true,
	'6':      true,
	'7':      true,
	'8':      true,
	'9':      true,
	':':      true,
	';':      true,
	'<':      true,
	'=':      true,
	'>':      true,
	'?':      true,
	'@':      true,
	'A':      true,
	'B':      true,
	'C':      true,
	'D':      true,
	'E':      true,
	'F':      true,
	'G':      true,
	'H':      true,
	'I':      true,
	'J':      true,
	'K':      true,
	'L':      true,
	'M':      true,
	'N':      true,
	'O':      true,
	'P':      true,
	'Q':      true,
	'R':      true,
	'S':      true,
	'T':      true,
	'U':      true,
	'V':      true,
	'W':      true,
	'X':      true,
	'Y':      true,
	'Z':      true,
	'[':      true,
	'\\':     false,
	']':      true,
	'^':      true,
	'_':      true,
	'`':      true,
	'a':      true,
	'b':      true,
	'c':      true,
	'd':      true,
	'e':      true,
	'f':      true,
	'g':      true,
	'h':      true,
	'i':      true,
	'j':      true,
	'k':      true,
	'l':      true,
	'm':      true,
	'n':      true,
	'o':      true,
	'p':      true,
	'q':      true,
	'r':      true,
	's':      true,
	't':      true,
	'u':      true,
	'v':      true,
	'w':      true,
	'x':      true,
	'y':      true,
	'z':      true,
	'{':      true,
	'|':      true,
	'}':      true,
	'~':      true,
	'\u007f': true,
}
