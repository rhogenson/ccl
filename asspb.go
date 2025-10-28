//	Me: Mom can we have textproto?
//	Mom: no we have textproto at home
//	textproto at home:
//
// The asspb language has similar semantics to JSON, the only exception being the lack of null.
//
// # Comments
//
// There are two types of comments, line comments and C-style comments. Line
// comments are written with # or //, and extend from there to the end of the
// line. C-style comments are written with /* and */, and like C they may not
// be nested.
//
//	# Comments are important
//	// in a configuration language
//	/* what do I know */
//
// # Numbers
//
// Numbers are written in base 10 and can optionally have a fractional part or
// an exponent written with "e" or "E". As a special case, a number prefixed
// with "0x" or "0X" can be written in base 16.
//
//	100
//	-30
//	0xabc
//	-0xdef
//	13.5
//	1e100
//
// Leading zeros are not permitted in decimal numbers, due to potential
// confusion with octal (which is not supported).
//
// As a lexical matter, numbers must be separated from subsequent field names by
// intervening whitespace or comments:
//
//	# invalid
//	field1:10field2:20
//	# ok
//	field1:10 field2:20
//
// # Strings
//
// Strings are written with " or ' and any sequence of intermediate bytes (with
// the exception of escape sequences which are described below). Strings must be
// valid UTF-8 after escape sequences are expanded.
//
//	'asdf'
//	"that's cool"
//	"\tall\n\tyour\n\tfavorite\n\tescape\n\tsequences"
//
// Note that strings can contain newline without needing an escape sequence
//
//	'a multiline
//	string'
//
// Backslash characters inside a string are interpreted as an escape sequence.
// Any escape sequence not described below is an error. The escape sequences
// are identical to C11, with the exception that \x always takes exactly 2
// hex characters.
//
//	\'    single quote       0x27
//	\"    double quote       0x22
//	\?    question mark      0x3f (why is this in C)
//	\\    backslash          0x5c
//	\a    bell               0x07
//	\b    backspace          0x07
//	\f    form feed          0x0c
//	\n    newline            0x0a
//	\r    carriage return    0x0d
//	\t    tab                0x09
//	\v    vertical tab       0x0b
//
//	\nnn          3-digit octal value nnn
//	\xnn          2-digit hex value nn
//	\unnnn        unicode code point U+nnnn
//	\Unnnnnnnn    unicode code point U+nnnnnnnn (UTF8)
//
// As an extension to the C11 escapes, a backslash immediately before a newline
// character (0x0a) will remove the newline character from the resulting string
// (and for you Microsoft Windows users, backslash followed by \r\n is
// also removed)
//
//	'backslash also can \
//	remove newlines'
//	# equivalent to
//	'backslash also can remove newlines'
//
// If multiple string literals are written next to each other with only
// whitespace or comments in between, the result is to concatenate the strings
//
//	'multiple strings' " concatenated"
//	# equivalent to
//	'multiple strings concatenated'
//
// # Bool
//
// Bool values can be true or false (classic), and are written using one of the
// below strings.
//
//	true
//	yes
//	on
//
//	false
//	no
//	off
//
// # Lists
//
// Lists are written with square brackets and elements are separated by comma.
//
//	[1, 2, 3]
//	[{nested: "messages"}, {are: "also"}, {allowed: yes}]
//
// Trailing comma is allowed
//
//	[
//	  "suck",
//	  "it",
//	  "JSON",
//	]
//
// # Messages
//
// Messages are an unordered set of key-value pairs:
//
//	{key1: "value1" key2: "value2"}
//
// Keys can be alphanumeric or use underscore; no other characters are
// permitted. Values can be any of the value types here described. Key-value
// pairs must be written with a : between the key and value, except when the
// value is syntactically a list or a message (in that case the colon
// is optional)
//
//	{
//	  key1: "value1"
//	  key2 {}
//	  key3 []
//	}
//
// As a special case, when a key is written more than once in a message, it's
// treated the same as if the values had been written in a list. If some of the
// values are already lists, they are appended, preserving the order in which
// the values appear in the input file.
//
//	{
//	  key: [1, 2]
//	  key: 3
//	  key: [4, 5, 6]
//	}
//	# equivalent to
//	{
//	  key: [1, 2, 3, 4, 5, 6]
//	}
package asspb

import (
	"bytes"
	"errors"
	"fmt"
	"iter"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

type syntaxError struct {
	line, col int
	reason    string
}

func newSyntaxError(data []byte, idx int, reason string, args ...any) error {
	line, col := 1, 1
	for _, b := range data[:idx] {
		if b == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return &syntaxError{line, col, fmt.Sprintf(reason, args...)}
}

func (e *syntaxError) Error() string {
	return fmt.Sprintf("%d:%d syntax error: %s", e.line, e.col, e.reason)
}

func (p *parser) fieldMap(s reflect.Value) (map[string]reflect.Value, error) {
	if !(s.Kind() == reflect.Struct || s.Kind() == reflect.Pointer && s.Type().Elem().Kind() == reflect.Struct) {
		return nil, p.error("field must be a struct or pointer to struct (got %T)", s.Interface())
	}
	if s.Kind() == reflect.Pointer {
		if s.IsNil() {
			s.Set(reflect.New(s.Type().Elem()))
		}
		s = s.Elem()
	}
	m := make(map[string]reflect.Value)
	for i := range s.NumField() {
		field := s.Type().Field(i)
		if !field.IsExported() {
			continue
		}
		fieldName := field.Name
		if tag, ok := field.Tag.Lookup("ccl"); ok {
			fieldName, _, _ = strings.Cut(tag, ",")
			if fieldName == "-" {
				continue
			}
		}
		m[fieldName] = s.Field(i)
	}
	return m, nil
}

type parser struct {
	nextTok func() (token, error, bool)
	tok     []byte
	err     error
	data    []byte
	i       int
}

func (p *parser) error(reason string, args ...any) error {
	return newSyntaxError(p.data, p.i, reason, args...)
}

var errEOF = errors.New("premature EOF")

func (p *parser) peek() ([]byte, error) {
	if p.err != nil || p.tok != nil {
		return p.tok, p.err
	}
	tok, err, ok := p.nextTok()
	if !ok {
		p.err = errEOF
		return nil, p.err
	}
	if err != nil {
		p.err = err
		return nil, p.err
	}
	p.tok = tok.b
	p.i = tok.i
	return p.tok, nil
}

func (p *parser) next() ([]byte, error) {
	tok, err := p.peek()
	if err != nil {
		return nil, err
	}
	p.tok = nil
	return tok, nil
}

var numRE = regexp.MustCompile(`^[-+]?(0[xX][0-9a-fA-F]+|((0|[1-9][0-9]*)(\.[0-9]*)?|\.[0-9]+)([eE][-+]?[0-9]+)?)$`)

func (p *parser) parseNum(out reflect.Value, numBytes []byte) error {
	if !numRE.Match(numBytes) {
		return p.error("invalid number")
	}
	if bytes.HasPrefix(numBytes, []byte("0x")) || bytes.HasPrefix(numBytes, []byte("0X")) {
		if out.Kind() != reflect.Int64 {
			return p.error("field must be int64")
		}
		n, err := strconv.ParseInt(string(numBytes[2:]), 16, 64)
		if err != nil {
			return p.error("invalid number")
		}
		out.SetInt(n)
		return nil
	}
	if bytes.ContainsAny(numBytes, ".eE") {
		if out.Kind() != reflect.Float64 {
			return p.error("field must be float64")
		}
		n, err := strconv.ParseFloat(string(numBytes), 64)
		if err != nil {
			return p.error("invalid number")
		}
		out.SetFloat(n)
		return nil
	}
	if out.Kind() != reflect.Int64 {
		return p.error("field must be int64")
	}
	n, err := strconv.ParseInt(string(numBytes), 10, 64)
	if err != nil {
		return p.error("invalid number")
	}
	out.SetInt(n)
	return nil
}

var escapesRE = regexp.MustCompile(`(?s)\\(.|\r\n|[0-7]{3}|x[0-9a-fA-F]{2}|u[0-9a-fA-F]{4}|U[0-9a-fA-F]{8})`)

func init() {
	escapesRE.Longest()
}

func (p *parser) unescape(rawStr []byte) ([]byte, error) {
	var err error
	escaped := escapesRE.ReplaceAllFunc(rawStr, func(escape []byte) []byte {
		switch string(escape) {
		case `\'`:
			return []byte("'")
		case `\"`:
			return []byte(`"`)
		case `\?`:
			return []byte("?")
		case `\\`:
			return []byte(`\`)
		case `\a`:
			return []byte("\a")
		case `\b`:
			return []byte("\b")
		case `\f`:
			return []byte("\f")
		case `\n`:
			return []byte("\n")
		case `\r`:
			return []byte("\r")
		case `\t`:
			return []byte("\t")
		case `\v`:
			return []byte("\v")
		case "\\\n", "\\\r\n":
			return nil
		}
		switch {
		case bytes.HasPrefix(escape, []byte(`\x`)):
			var n uint64
			if n, err = strconv.ParseUint(string(escape[2:]), 16, 8); err != nil {
				err = p.error("invalid hex escape %q: %s", escape, err)
				return nil
			}
			return []byte{byte(n)}
		case bytes.HasPrefix(escape, []byte(`\u`)), bytes.HasPrefix(escape, []byte(`\U`)):
			var n int64
			if n, err = strconv.ParseInt(string(escape[2:]), 16, 32); err != nil {
				err = p.error("invalid unicode escape %q: %s", escape, err)
				return nil
			}
			return utf8.AppendRune(nil, rune(n))
		default:
			if len(escape) != 4 {
				err = p.error("invalid string escape %q", escape)
				return nil
			}
			var n int64
			if n, err = strconv.ParseInt(string(escape[1:]), 8, 32); err != nil {
				err = p.error("invalid string escape %q", escape)
				return nil
			}
			if n > 255 {
				err = p.error("invalid octal escape %q %d > 255", escape, n)
				return nil
			}
			return []byte{byte(n)}
		}
	})
	if err != nil {
		return nil, err
	}
	if !utf8.Valid(escaped) {
		return nil, p.error("syntax error: string %q is not UTF-8 encoded", escaped)
	}
	return escaped, nil
}

func (p *parser) parseString(out reflect.Value, tok []byte) error {
	if out.Kind() != reflect.String {
		return p.error("field must be string")
	}
	s := new(strings.Builder)
	for {
		ss, err := p.unescape(tok[1 : len(tok)-1])
		if err != nil {
			return err
		}
		s.Write(ss)
		nextTok, err := p.peek()
		if err != nil || nextTok[0] != '\'' && nextTok[0] != '"' {
			out.SetString(s.String())
			return nil
		}
		p.next()
		tok = nextTok
	}
}

func (p *parser) parseMessage(out reflect.Value) error {
	fieldMap, err := p.fieldMap(out)
	if err != nil {
		return err
	}
	for {
		tok, err := p.next()
		if err != nil || tok[0] == '}' {
			return err
		}
		if err := p.parseFieldVal(fieldMap, tok); err != nil {
			return err
		}
	}
}

func (p *parser) parseVal(out reflect.Value, tok []byte) error {
	var err error
	switch tok[0] {
	case '{':
		err = p.parseMessage(out)
	case '[':
		err = p.parseList(out)
	case '\'', '"':
		err = p.parseString(out, tok)
	default:
		switch string(tok) {
		case "true", "yes", "on":
			if out.Kind() != reflect.Bool {
				return p.error("field must be bool")
			}
			out.SetBool(true)
		case "false", "no", "off":
			if out.Kind() != reflect.Bool {
				return p.error("field must be bool")
			}
			out.SetBool(false)
		default:
			err = p.parseNum(out, tok)
		}
	}
	return err
}

func (p *parser) parseList(out reflect.Value) error {
	if out.Kind() != reflect.Slice {
		return p.error("field must be slice")
	}
	if out.IsNil() {
		// Not technically necessary since nil slices are usually treated the
		// same as any empty slice, but usually nil would mean that the user
		// didn't set the value, at least for types that have a nil.
		out.Set(reflect.MakeSlice(out.Type(), 0, 0))
	}
	for i := 0; ; i++ {
		tok, err := p.next()
		if err != nil || tok[0] == ']' {
			return err
		}
		if i > 0 {
			if tok[0] != ',' {
				return p.error("expecting comma")
			}
			tok, err = p.next()
			if err != nil || tok[0] == ']' { // allow trailing comma
				return err
			}
		}
		out.Set(reflect.Append(out, reflect.Zero(out.Type().Elem())))
		if err := p.parseVal(out.Index(out.Len()-1), tok); err != nil {
			return err
		}
	}
}

func (p *parser) appendVal(out reflect.Value, tok []byte) error {
	if tok[0] == '[' || out.Kind() != reflect.Slice {
		if err := p.parseVal(out, tok); err != nil {
			return err
		}
	} else {
		out.Set(reflect.Append(out, reflect.Zero(out.Type().Elem())))
		if err := p.parseVal(out.Index(out.Len()-1), tok); err != nil {
			return err
		}
	}
	return nil
}

func (p *parser) parseFieldVal(fieldMap map[string]reflect.Value, field []byte) error {
	if b := field[0]; !(b == '_' || 'a' <= b && b <= 'z' || 'A' <= b && b <= 'Z') {
		return p.error("expecting field")
	}
	structField, ok := fieldMap[string(field)]
	if !ok {
		return p.error("no field named %q", field)
	}
	tok, err := p.next()
	if err != nil {
		return err
	}
	switch tok[0] {
	case '{', '[':
		if err := p.appendVal(structField, tok); err != nil {
			return err
		}
	case ':':
		tok, err := p.next()
		if err != nil {
			return err
		}
		if err := p.appendVal(structField, tok); err != nil {
			return err
		}
	default:
		return p.error("expecting colon")
	}
	return nil
}

func (p *parser) parse(v any) error {
	sp := reflect.ValueOf(v)
	if sp.Kind() != reflect.Pointer || sp.IsNil() {
		return p.error("value must be a non-nil pointer")
	}
	fieldMap, err := p.fieldMap(sp.Elem())
	if err != nil {
		return err
	}
	for {
		tok, err := p.next()
		if err != nil {
			if err == errEOF {
				return nil
			}
			return err
		}
		if err := p.parseFieldVal(fieldMap, tok); err != nil {
			return err
		}
	}
}

// Unmarshal parses a ccl message and writes the result into v. Unmarshal
// internally calls json.Unmarshal for the reflection-based struct unpacking, so
// feel free to use json struct tags on v, or implement UnmarshalJSON to control
// the unmarshalling behavior.
//
// Unmarshal accepts a top-level message, which is equivalent to the "message"
// type described above, but without the surrounding braces. For example:
//
//	key1: "val1"
//	key2: "val2"
func Unmarshal(data []byte, v any) error {
	nextToken, stop := iter.Pull2(tokens(data))
	defer stop()
	return (&parser{nextTok: nextToken, data: data}).parse(v)
}
