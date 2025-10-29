//	Me: Mom can we have textproto?
//	Mom: no we have textproto at home
//	textproto at home:
//
// The asspb language has similar semantics to JSON, the only exception being
// the lack of null.
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
// value is syntactically a message (in that case the colon is optional)
//
//	{
//	  key1: "value1"
//	  key2 {}
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
//
// # Disclaimer
//
// This package is still experimental, expect breaking changes.
package asspb

import (
	"bytes"
	"encoding"
	"encoding/base64"
	"errors"
	"fmt"
	"iter"
	"math"
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

type structField struct {
	ty   reflect.Type
	name string
}

func fieldMap(out map[structField]int, types map[reflect.Type]bool, s reflect.Type) error {
	if types[s] {
		// Already processed
		return nil
	}
	types[s] = true
	for i := range s.NumField() {
		field := s.Field(i)
		if !field.IsExported() {
			continue
		}
		fieldName := field.Name
		if tag, ok := field.Tag.Lookup("ccl"); ok {
			var opts string
			fieldName, opts, _ = strings.Cut(tag, ",")
			if fieldName == "-" {
				continue
			}
			for opt := range strings.FieldsFuncSeq(opts, func(r rune) bool { return r == ',' }) {
				return fmt.Errorf("unknown option %q", opt)
			}
		}
		if _, ok := out[structField{s, fieldName}]; ok {
			return fmt.Errorf("multiple fields with name %q", fieldName)
		}
		out[structField{s, fieldName}] = i
		if field.Type.Kind() == reflect.Struct {
			if err := fieldMap(out, types, field.Type); err != nil {
				return err
			}
		} else if (field.Type.Kind() == reflect.Pointer || field.Type.Kind() == reflect.Slice) && field.Type.Elem().Kind() == reflect.Struct {
			if err := fieldMap(out, types, field.Type.Elem()); err != nil {
				return err
			}
		} else if field.Type.Kind() == reflect.Slice && field.Type.Elem().Kind() == reflect.Pointer && field.Type.Elem().Elem().Kind() == reflect.Struct {
			if err := fieldMap(out, types, field.Type.Elem().Elem()); err != nil {
				return err
			}
		}
	}
	return nil
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

var (
	numRE = regexp.MustCompile(`^[-+]?(0[xX][0-9a-fA-F]+|((0|[1-9][0-9]*)(\.[0-9]*)?|\.[0-9]+)([eE][-+]?[0-9]+)?)$`)
	hexRE = regexp.MustCompile(`^([-+]?)0[xX]`)
)

func (p *parser) parseNum(numBytes []byte) (any, error) {
	if !numRE.Match(numBytes) {
		return nil, p.error("invalid number")
	}
	if hex := hexRE.FindSubmatch(numBytes); hex != nil {
		if string(hex[1]) == "-" {
			n, err := strconv.ParseInt(string(numBytes[len(hex[0]):]), 16, 64)
			if err != nil {
				return nil, p.error("invalid number")
			}
			return -n, nil
		} else {
			n, err := strconv.ParseUint(string(numBytes[len(hex[0]):]), 16, 64)
			if err != nil {
				return nil, p.error("invalid number")
			}
			return n, nil
		}
	}
	if bytes.ContainsAny(numBytes, ".eE") {
		n, err := strconv.ParseFloat(string(numBytes), 64)
		if err != nil {
			return nil, p.error("invalid number")
		}
		return n, nil
	}
	if bytes.HasPrefix(numBytes, []byte("-")) {
		n, err := strconv.ParseInt(string(numBytes), 10, 64)
		if err != nil {
			return nil, p.error("invalid number")
		}
		return n, nil
	} else {
		n, err := strconv.ParseUint(string(bytes.TrimPrefix(numBytes, []byte("+"))), 10, 64)
		if err != nil {
			return nil, p.error("invalid number")
		}
		return n, nil
	}
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

func (p *parser) parseMessage() (map[string][]any, error) {
	m := make(map[string][]any)
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

func (p *parser) parseVal(tok []byte) ([]any, error) {
	switch tok[0] {
	case '{':
		m, err := p.parseMessage()
		if err != nil {
			return nil, err
		}
		return []any{m}, nil
	case '[':
		return p.parseList()
	case '\'', '"':
		s, err := p.parseString(tok)
		if err != nil {
			return nil, err
		}
		return []any{s}, nil
	default:
		switch string(tok) {
		case "true", "yes", "on":
			return []any{true}, nil
		case "false", "no", "off":
			return []any{false}, nil
		default:
			n, err := p.parseNum(tok)
			if err != nil {
				return nil, err
			}
			return []any{n}, nil
		}
	}
}

func (p *parser) parseList() ([]any, error) {
	var l []any
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
		vs, err := p.parseVal(tok)
		if err != nil {
			return nil, err
		}
		l = append(l, vs...)
	}
}

func (p *parser) parseFieldVal(m map[string][]any, field []byte) error {
	if b := field[0]; !(b == '_' || 'a' <= b && b <= 'z' || 'A' <= b && b <= 'Z') {
		return p.error("expecting field")
	}
	tok, err := p.next()
	if err != nil {
		return err
	}
	switch tok[0] {
	case '{':
		vs, err := p.parseVal(tok)
		if err != nil {
			return err
		}
		m[string(field)] = append(m[string(field)], vs...)
	case ':':
		tok, err := p.next()
		if err != nil {
			return err
		}
		vs, err := p.parseVal(tok)
		if err != nil {
			return err
		}
		m[string(field)] = append(m[string(field)], vs...)
	default:
		return p.error("expecting colon")
	}
	return nil
}

func (p *parser) parse() (map[string][]any, error) {
	m := make(map[string][]any)
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

func intLimits(kind reflect.Kind) (min int64, max uint64, ok bool) {
	switch kind {
	case reflect.Int:
		return math.MinInt, math.MaxInt, true
	case reflect.Int8:
		return math.MinInt8, math.MaxInt8, true
	case reflect.Int16:
		return math.MinInt16, math.MaxInt16, true
	case reflect.Int32:
		return math.MinInt32, math.MaxInt32, true
	case reflect.Int64:
		return math.MinInt64, math.MaxInt64, true
	case reflect.Uint:
		return 0, math.MaxUint, true
	case reflect.Uint8:
		return 0, math.MaxUint8, true
	case reflect.Uint16:
		return 0, math.MaxUint16, true
	case reflect.Uint32:
		return 0, math.MaxUint32, true
	case reflect.Uint64:
		return 0, math.MaxUint64, true
	default:
		return 0, 0, false
	}
}

func unpackVal(fieldVal reflect.Value, fieldMap map[structField]int, val any, field string) error {
	switch val := val.(type) {
	case bool:
		switch fieldVal.Kind() {
		case reflect.Bool:
			fieldVal.SetBool(val)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if val {
				fieldVal.SetInt(1)
			} else {
				fieldVal.SetInt(0)
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if val {
				fieldVal.SetUint(1)
			} else {
				fieldVal.SetUint(0)
			}
		default:
			return fmt.Errorf("field %q should have type bool", field)
		}
	case uint64:
		switch fieldVal.Kind() {
		case reflect.Float32, reflect.Float64:
			fieldVal.SetFloat(float64(val))
			return nil
		}
		min, max, ok := intLimits(fieldVal.Kind())
		if !ok {
			return fmt.Errorf("field %q should have type int", field)
		}
		if val > max {
			return fmt.Errorf("number %d is out of range for %s", val, fieldVal.Kind())
		}
		if min == 0 { // unsigned
			fieldVal.SetUint(val)
		} else {
			fieldVal.SetInt(int64(val))
		}
	case int64:
		switch fieldVal.Kind() {
		case reflect.Float32, reflect.Float64:
			fieldVal.SetFloat(float64(val))
			return nil
		}
		min, max, ok := intLimits(fieldVal.Kind())
		if !ok {
			return fmt.Errorf("field %q should have type int", field)
		}
		if val < min || val > 0 && uint64(val) > max {
			return fmt.Errorf("number %d is out of range for %s", val, fieldVal.Kind())
		}
		if min == 0 { // unsigned
			fieldVal.SetUint(uint64(val))
		} else {
			fieldVal.SetInt(val)
		}
	case float64:
		switch fieldVal.Kind() {
		case reflect.Float32, reflect.Float64:
			fieldVal.SetFloat(float64(val))
		default:
			return fmt.Errorf("field %q should have type float64 or float32", field)
		}
	case string:
		if _, ok := fieldVal.Interface().(encoding.TextUnmarshaler); ok {
			if fieldVal.Kind() == reflect.Pointer && fieldVal.IsNil() {
				fieldVal.Set(reflect.New(fieldVal.Type().Elem()))
			}
			return fieldVal.Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(val))
		}
		if unmarshaler, ok := fieldVal.Addr().Interface().(encoding.TextUnmarshaler); ok {
			return unmarshaler.UnmarshalText([]byte(val))
		}
		switch {
		case fieldVal.Kind() == reflect.String:
			fieldVal.SetString(val)
		case fieldVal.Kind() == reflect.Slice && fieldVal.Type().Elem() == reflect.TypeFor[byte]():
			b, err := base64.StdEncoding.DecodeString(val)
			if err != nil {
				return fmt.Errorf("field %q: bad base64", field)
			}
			fieldVal.Set(reflect.ValueOf(b))
		default:
			return fmt.Errorf("field %q should have type string (got %s)", field, fieldVal.Type())
		}
	case map[string][]any:
		if !(fieldVal.Kind() == reflect.Struct || fieldVal.Kind() == reflect.Pointer && fieldVal.Type().Elem().Kind() == reflect.Struct) {
			return fmt.Errorf("field %q should have type struct (got %s)", field, fieldVal.Type())
		}
		if fieldVal.Kind() == reflect.Pointer {
			if fieldVal.IsNil() {
				fieldVal.Set(reflect.New(fieldVal.Type().Elem()))
			}
			fieldVal = fieldVal.Elem()
		}
		if err := unpackStruct(fieldVal, fieldMap, val); err != nil {
			return err
		}
	}
	return nil
}

func unpackStruct(out reflect.Value, fieldMap map[structField]int, msg map[string][]any) error {
	for field, vals := range msg {
		fieldIdx, ok := fieldMap[structField{out.Type(), field}]
		if !ok {
			return fmt.Errorf("no field named %q", field)
		}
		fieldVal := out.Field(fieldIdx)
		if fieldVal.Kind() == reflect.Slice && fieldVal.Type().Elem() != reflect.TypeFor[byte]() {
			l := reflect.MakeSlice(fieldVal.Type(), len(vals), len(vals))
			for i, val := range vals {
				if err := unpackVal(l.Index(i), fieldMap, val, field); err != nil {
					return err
				}
			}
			if fieldVal.IsNil() {
				fieldVal.Set(reflect.MakeSlice(fieldVal.Type(), 0, 0))
			}
			fieldVal.Set(reflect.AppendSlice(fieldVal, l))
			continue
		}
		if len(vals) != 1 {
			return fmt.Errorf("field %q is not repeated", field)
		}
		if err := unpackVal(fieldVal, fieldMap, vals[0], field); err != nil {
			return err
		}
	}
	return nil
}

// Unmarshal parses a asspb message and writes the result into v. v must be a
// non-nil pointer to a struct.
//
// Unmarshal accepts a top-level message, which is equivalent to the "message"
// type described above, but without the surrounding braces. For example:
//
//	key1: "val1"
//	key2: "val2"
//
// The exact semantics of which asspb types map to which Go types is a bit
// complicated and I don't feel like writing out all the rules, so suffice it
// to say that the usual stuff should work. As a special case, a []byte field
// expects a base64-encoded string.
//
// You can override a field's name using a struct tag "ccl", for example
//
//	type message struct {
//	    MyField int `ccl:"my_field"`
//	}
//
// This message could decode, for example `my_field:5`
//
// If a field has type T where T or *T implements [encoding.TextUnmarshaler],
// then a string value will be decoded by calling UnmarshalText. No other
// customization is supported, this isn't encoding/json.
func Unmarshal(data []byte, v any) error {
	val := reflect.ValueOf(v)
	if val.Kind() != reflect.Pointer || val.IsNil() || val.Type().Elem().Kind() != reflect.Struct {
		return fmt.Errorf("value must be a non-nil pointer to a struct")
	}
	fields := make(map[structField]int)
	if err := fieldMap(fields, make(map[reflect.Type]bool), val.Type().Elem()); err != nil {
		return err
	}
	nextToken, stop := iter.Pull2(tokens(data))
	defer stop()
	msg, err := (&parser{nextTok: nextToken, data: data}).parse()
	if err != nil {
		return err
	}
	return unpackStruct(val.Elem(), fields, msg)
}
