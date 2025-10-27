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
	"encoding/json"
	"errors"
	"fmt"
	"iter"
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

func appendAnyRepeated(prev any, new ...any) []any {
	if prev == nil {
		return new
	}
	if prevList, ok := prev.([]any); ok {
		return append(prevList, new...)
	}
	return append([]any{prev}, new...)
}

func appendAny(prev any, new any) any {
	if prev == nil {
		return new
	}
	return appendAnyRepeated(prev, new)
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

func parseNum(numBytes []byte) (any, bool) {
	if !numRE.Match(numBytes) {
		return nil, false
	}
	if bytes.HasPrefix(numBytes, []byte("0x")) || bytes.HasPrefix(numBytes, []byte("0X")) {
		n, err := strconv.ParseInt(string(numBytes[2:]), 16, 64)
		if err != nil {
			return nil, false
		}
		return n, true
	}
	if bytes.ContainsAny(numBytes, ".eE") {
		n, err := strconv.ParseFloat(string(numBytes), 64)
		if err != nil {
			return nil, false
		}
		return n, true
	}
	n, err := strconv.ParseInt(string(numBytes), 10, 64)
	if err != nil {
		return nil, false
	}
	return n, true
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

func (p *parser) parseString(tok []byte) (string, error) {
	s := new(strings.Builder)
	for {
		ss, err := p.unescape(tok[1 : len(tok)-1])
		if err != nil {
			return "", err
		}
		s.Write(ss)
		nextTok, err := p.peek()
		if err != nil || nextTok[0] != '\'' && nextTok[0] != '"' {
			return s.String(), nil
		}
		p.next()
		tok = nextTok
	}
}

func (p *parser) parseMessage() (map[string]any, error) {
	m := make(map[string]any)
	for {
		tok, err := p.next()
		if err != nil || tok[0] == '}' {
			return m, err
		}
		if err := p.parseFieldVal(m, tok); err != nil {
			return nil, err
		}
	}
}

func (p *parser) parseVal(tok []byte) (any, error) {
	var (
		val any
		err error
	)
	switch tok[0] {
	case '{':
		val, err = p.parseMessage()
	case '[':
		val, err = p.parseList()
	case '\'', '"':
		val, err = p.parseString(tok)
	default:
		switch string(tok) {
		case "true", "yes", "on":
			val = true
		case "false", "no", "off":
			val = false
		default:
			var ok bool
			val, ok = parseNum(tok)
			if !ok {
				return nil, p.error("expecting field value")
			}
		}
	}
	return val, err
}

func (p *parser) parseList() ([]any, error) {
	l := []any{}
	for i := 0; ; i++ {
		tok, err := p.next()
		if err != nil || tok[0] == ']' {
			return l, err
		}
		if i > 0 {
			if tok[0] != ',' {
				return nil, p.error("expecting comma")
			}
			tok, err = p.next()
			if err != nil || tok[0] == ']' { // allow trailing comma
				return l, err
			}
		}
		val, err := p.parseVal(tok)
		if err != nil {
			return nil, err
		}
		l = append(l, val)
	}
}

func (p *parser) parseFieldVal(out map[string]any, field []byte) error {
	if b := field[0]; !(b == '_' || 'a' <= b && b <= 'z' || 'A' <= b && b <= 'Z') {
		return p.error("expecting field")
	}
	tok, err := p.next()
	if err != nil {
		return err
	}
	var val any
	switch tok[0] {
	case '{':
		if val, err = p.parseMessage(); err != nil {
			return err
		}
	case '[':
		val, err := p.parseList()
		if err != nil {
			return err
		}
		out[string(field)] = appendAnyRepeated(out[string(field)], val...)
		return nil
	case ':':
		tok, err := p.next()
		if err != nil {
			return err
		}
		if val, err = p.parseVal(tok); err != nil {
			return err
		}
		if l, ok := val.([]any); ok {
			out[string(field)] = appendAnyRepeated(out[string(field)], l...)
			return nil
		}
	default:
		return p.error("expecting colon")
	}
	out[string(field)] = appendAny(out[string(field)], val)
	return nil
}

func (p *parser) parse() (map[string]any, error) {
	m := make(map[string]any)
	for {
		tok, err := p.next()
		if err != nil {
			if err == errEOF {
				return m, nil
			}
			return nil, err
		}
		if err := p.parseFieldVal(m, tok); err != nil {
			return nil, err
		}
	}
}

// Unmarshal parses an asspb message and writes the result into v. Unmarshal
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
	m, err := (&parser{nextTok: nextToken, data: data}).parse()
	if err != nil {
		return err
	}
	jsonBytes, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonBytes, v)
}
