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
// an exponent written with "e". As a special case, a number prefixed with "0x"
// can be written in base 16.
//
//	100
//	-30
//	0xabc
//	-0xdef
//	13.5
//	1e100
//
// # Strings
//
// Strings are written with " or ' and any sequence of intermediate bytes (with
// the exception of escape sequences which are described below). "any bytes"
// means that strings can contain newline without needing an escape sequence
//
//	'asdf'
//	"that's cool"
//	"\tall\n\tyour\n\tfavorite\n\tescape\n\tsequences"
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
//	\Unnnnnnnn    unicode code point U+nnnnnnnn
//
// As an extension to the C11 escapes, a backslash immediately before a newline
// character (0x0a) will remove the newline character from the resulting string
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
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

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
	data []byte
	i    int
}

var spaceRE = regexp.MustCompile(`^([[:space:]\p{Zs}]|(#|//)[^\n]*|/\*([^*]|\*[^/])*\*?\*/)*`)

func (p *parser) skipSpace() {
	p.i += len(spaceRE.Find(p.data[p.i:]))
}

func (p *parser) parseLit(s string) bool {
	p.skipSpace()
	if len(p.data[p.i:]) >= len(s) && string(p.data[p.i:p.i+len(s)]) == s {
		p.i += len(s)
		return true
	}
	return false
}

var numRE = regexp.MustCompile(`^-?(0x[0-9a-fA-F]+|([0-9]+(\.[0-9]*)?|\.[0-9]+)(e-?[0-9]+)?)`)

func (p *parser) parseNum() (any, bool) {
	p.skipSpace()
	numBytes := numRE.Find(p.data[p.i:])
	if numBytes == nil {
		return nil, false
	}
	if bytes.ContainsAny(numBytes, ".e") {
		n, err := strconv.ParseFloat(string(numBytes), 64)
		if err != nil {
			return nil, false
		}
		p.i += len(numBytes)
		return n, true
	}
	if b, ok := bytes.CutPrefix(numBytes, []byte("0x")); ok {
		n, err := strconv.ParseInt(string(b), 16, 64)
		if err != nil {
			return nil, false
		}
		p.i += len(numBytes)
		return n, true
	}
	n, err := strconv.ParseInt(string(numBytes), 10, 64)
	if err != nil {
		return nil, false
	}
	p.i += len(numBytes)
	return n, true
}

var escapesRE = regexp.MustCompile(`(?s)\\(.|[0-7]{3}|x[0-9a-fA-F]{2}|u[0-9a-fA-F]{4}|U[0-9a-fA-F]{8})`)

func init() {
	escapesRE.Longest()
}

func unescape(idx int, rawStr []byte) ([]byte, error) {
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
		case "\\\n":
			return nil
		}
		if bytes.HasPrefix(escape, []byte(`\x`)) || bytes.HasPrefix(escape, []byte(`\u`)) || bytes.HasPrefix(escape, []byte(`\U`)) {
			var n int64
			if n, err = strconv.ParseInt(string(escape[2:]), 16, 32); err != nil {
				err = fmt.Errorf("%d: syntax error: invalid hex escape %q: %s", idx, escape, err)
				return nil
			}
			return utf8.AppendRune(nil, rune(n))
		}
		if len(escape) != 4 {
			err = fmt.Errorf("%d: syntax error: invalid string escape %q", idx, escape)
			return nil
		}
		var n int64
		if n, err = strconv.ParseInt(string(escape[1:]), 8, 32); err != nil {
			err = fmt.Errorf("%d: syntax error: invalid string escape %q", idx, escape)
			return nil
		}
		return utf8.AppendRune(nil, rune(n))
	})
	if err != nil {
		return nil, err
	}
	return escaped, nil
}

var (
	stringRE       = regexp.MustCompile(`(?s)^(([^'\\]|\\.)*)'`)
	doubleStringRE = regexp.MustCompile(`(?s)^(([^"\\]|\\.)*)"`)
)

func (p *parser) parseString(double bool) (string, error) {
	re := stringRE
	if double {
		re = doubleStringRE
	}
	s := new(strings.Builder)
	for {
		p.skipSpace()
		rawStr := re.FindSubmatch(p.data[p.i:])
		if rawStr == nil {
			return "", fmt.Errorf("%d: syntax error: invalid string", p.i)
		}
		ss, err := unescape(p.i, rawStr[1])
		if err != nil {
			return "", err
		}
		s.Write(ss)
		p.i += len(rawStr[0])
		switch {
		case p.parseLit("'"):
			re = stringRE
			continue
		case p.parseLit(`"`):
			re = doubleStringRE
			continue
		}
		return s.String(), nil
	}
}

var fieldRE = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z_0-9]*`)

func (p *parser) parseField() ([]byte, error) {
	p.skipSpace()
	fieldName := fieldRE.Find(p.data[p.i:])
	if fieldName == nil {
		return nil, fmt.Errorf("%d: syntax error: expecting field name", p.i)
	}
	p.i += len(fieldName)
	return fieldName, nil
}

func (p *parser) parseMessage() (map[string]any, error) {
	m := make(map[string]any)
	for {
		if p.parseLit("}") {
			return m, nil
		}
		if err := p.parseFieldVal(m); err != nil {
			return nil, err
		}
	}
}

func (p *parser) parseVal() (any, error) {
	var (
		val any
		err error
	)
	switch {
	case p.parseLit("{"):
		val, err = p.parseMessage()
	case p.parseLit("["):
		val, err = p.parseList()
	case p.parseLit("'"):
		val, err = p.parseString(false)
	case p.parseLit(`"`):
		val, err = p.parseString(true)
	case p.parseLit("true"), p.parseLit("yes"), p.parseLit("on"):
		val = true
	case p.parseLit("false"), p.parseLit("no"), p.parseLit("off"):
		val = false
	default:
		var ok bool
		val, ok = p.parseNum()
		if !ok {
			return nil, fmt.Errorf("%d: syntax error: expecting field value", p.i)
		}
	}
	return val, err
}

func (p *parser) parseList() ([]any, error) {
	l := []any{}
	for i := 0; ; i++ {
		if p.parseLit("]") {
			return l, nil
		}
		if i > 0 {
			if !p.parseLit(",") {
				return nil, fmt.Errorf("%d: syntax error: expecting comma", p.i)
			}
			if p.parseLit("]") { // allow trailing comma
				return l, nil
			}
		}
		val, err := p.parseVal()
		if err != nil {
			return nil, err
		}
		l = append(l, val)
	}
}

func (p *parser) parseFieldVal(out map[string]any) error {
	field, err := p.parseField()
	if err != nil {
		return err
	}
	var val any
	switch {
	case p.parseLit("{"):
		if val, err = p.parseMessage(); err != nil {
			return err
		}
	case p.parseLit("["):
		val, err := p.parseList()
		if err != nil {
			return err
		}
		out[string(field)] = appendAnyRepeated(out[string(field)], val...)
		return nil
	case p.parseLit(":"):
		if val, err = p.parseVal(); err != nil {
			return err
		}
		if l, ok := val.([]any); ok {
			out[string(field)] = appendAnyRepeated(out[string(field)], l...)
			return nil
		}
	default:
		return fmt.Errorf("%d: syntax error: expecting colon", p.i)
	}
	out[string(field)] = appendAny(out[string(field)], val)
	return nil
}

func (p *parser) parse() (map[string]any, error) {
	m := make(map[string]any)
	for {
		p.skipSpace()
		if p.i == len(p.data) {
			return m, nil
		}
		if err := p.parseFieldVal(m); err != nil {
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
	p := &parser{data: data}
	m, err := p.parse()
	if err != nil {
		return err
	}
	jsonBytes, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonBytes, v)
}
