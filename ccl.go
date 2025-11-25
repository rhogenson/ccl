//	Me: Mom can we have textproto?
//	Mom: no we have textproto at home
//	textproto at home:
//
// The ccl language has similar semantics to JSON, the only exception being the
// lack of null.
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
// Strings are written with " or ' and a (possibly empty) sequence of
// intervening characters. Strings must be valid UTF-8 after expanding escape
// sequences (described below).
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
// Carriage returns (0x0d) are discarded from the string value. If you need a
// string to contain carriage return, use the \r escape sequence.
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
// Bool values can be true or false (classic).
//
//	true
//	false
//
// # Lists
//
// Lists are written with square brackets and elements are separated by comma.
//
//	[1, 2, 3]
//	[{nested: "messages"}, {are: "also"}, {allowed: "yep"}]
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
package ccl

import (
	"bytes"
	"encoding"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"unicode"
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
	lexer    lexer
	tok      []byte
	err      error
	data     []byte
	i        int
	fieldMap map[structField]int
}

func (p *parser) error(reason string, args ...any) error {
	return newSyntaxError(p.data, p.i, reason, args...)
}

var errEOF = errors.New("premature EOF")

func (p *parser) peek() ([]byte, error) {
	if p.err != nil || p.tok != nil {
		return p.tok, p.err
	}
	tok, err := p.lexer.next()
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

func checkNum(b []byte) bool {
	if b[0] == '-' || b[0] == '+' {
		b = b[1:]
	}
	if bytes.Equal(b, []byte("0")) {
		return true
	}
	if len(b) == 0 || !(b[0] == '.' || '1' <= b[0] && b[0] <= '9') {
		return false
	}
	haveDigits := false
	for ; len(b) > 0 && '0' <= b[0] && b[0] <= '9'; b = b[1:] {
		haveDigits = true
	}
	if len(b) > 0 && b[0] == '.' {
		b = b[1:]
		for ; len(b) > 0 && '0' <= b[0] && b[0] <= '9'; b = b[1:] {
			haveDigits = true
		}
	}
	if !haveDigits {
		return false
	}
	if len(b) == 0 || !(b[0] == 'e' || b[0] == 'E') {
		return true
	}
	b = b[1:]
	if len(b) > 0 && (b[0] == '-' || b[0] == '+') {
		b = b[1:]
	}
	if len(b) == 0 {
		return false
	}
	for ; len(b) > 0 && '0' <= b[0] && b[0] <= '9'; b = b[1:] {
	}
	return len(b) == 0
}

type integer struct {
	n   uint64
	sgn int8
}

func (p *parser) parseInt(numBytes []byte) (integer, error) {
	n := numBytes
	var sgn int8 = 1
	switch numBytes[0] {
	case '-':
		sgn = -1
		n = numBytes[1:]
	case '+':
		n = numBytes[1:]
	}
	if len(n) > 2 && n[0] == '0' && (n[1] == 'x' || n[1] == 'X') {
		n, err := strconv.ParseUint(string(n[2:]), 16, 64)
		if err != nil {
			return integer{}, p.error("invalid hex number: %s", err)
		}
		return integer{n, sgn}, nil
	}
	if !checkNum(numBytes) {
		return integer{}, p.error("invalid number")
	}
	un, err := strconv.ParseUint(string(n), 10, 64)
	if err != nil {
		return integer{}, p.error("invalid number (unreachable)")
	}
	return integer{un, sgn}, nil
}

func (p *parser) parseFloat(nBytes []byte) (float64, error) {
	if !checkNum(nBytes) {
		return 0, p.error("invalid number")
	}
	n, err := strconv.ParseFloat(string(nBytes), 64)
	if err != nil {
		return 0, p.error("invalid number (unreachable)")
	}
	return n, nil
}

func (p *parser) unescape(rawStr []byte) ([]byte, error) {
	var escaped []byte
	for i := 0; i < len(rawStr); i++ {
		if i+1 < len(rawStr) && rawStr[i] == '\r' && rawStr[i+1] == '\n' {
			continue
		}
		if rawStr[i] != '\\' {
			r, n := utf8.DecodeRune(rawStr[i:])
			if r != '\t' && r != '\n' && unicode.IsControl(r) {
				return nil, p.error("control character %q must be escaped", r)
			}
			escaped = append(escaped, rawStr[i:i+n]...)
			i += n - 1
			continue
		}
		i++
		var b []byte
		switch rawStr[i] {
		case '\'':
			b = []byte("'")
		case '"':
			b = []byte(`"`)
		case '?':
			b = []byte("?")
		case '\\':
			b = []byte(`\`)
		case 'a':
			b = []byte("\a")
		case 'b':
			b = []byte("\b")
		case 'f':
			b = []byte("\f")
		case 'n':
			b = []byte("\n")
		case 'r':
			b = []byte("\r")
		case 't':
			b = []byte("\t")
		case 'v':
			b = []byte("\v")
		case '\n':
			b = nil
		case '\r':
			i++
			if i < len(rawStr) && rawStr[i] == '\n' {
				b = nil
			} else {
				return nil, p.error("invalid escape sequence %q", rawStr[i-2:min(i+1, len(rawStr))])
			}
		case 'x':
			i++
			if i+2 > len(rawStr) {
				return nil, p.error("invalid hex escape %q", rawStr[i-2:min(i+2, len(rawStr))])
			}
			n, err := strconv.ParseUint(string(rawStr[i:i+2]), 16, 8)
			if err != nil {
				return nil, p.error("invalid hex escape %q: %s", rawStr[i-2:i+2], err)
			}
			i++
			b = []byte{byte(n)}
		case 'u', 'U':
			nBytes := 4
			if rawStr[i] == 'U' {
				nBytes = 8
			}
			i++
			if i+nBytes > len(rawStr) {
				return nil, p.error("invalid unicode escape %q", rawStr[i-2:min(i+nBytes, len(rawStr))])
			}
			n, err := strconv.ParseUint(string(rawStr[i:i+nBytes]), 16, 31)
			if err != nil {
				return nil, p.error("invalid unicode escape %q: %s", rawStr[i-2:i+nBytes], err)
			}
			i += nBytes - 1
			b = utf8.AppendRune(nil, rune(n))
		default:
			if i+3 > len(rawStr) {
				return nil, p.error("invalid string escape %q", rawStr[i-1:i+1])
			}
			n, err := strconv.ParseUint(string(rawStr[i:i+3]), 8, 8)
			if err != nil {
				return nil, p.error("invalid octal escape %q: %s", rawStr[i-1:i+3], err)
			}
			i += 2
			b = []byte{byte(n)}
		}
		escaped = append(escaped, b...)
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

func (p *parser) parseMessage(out reflect.Value, field []byte) error {
	out = setPtr(out)
	if out.Kind() != reflect.Struct {
		return p.error("field %q should be a struct", field)
	}
	seen := make(map[string]bool)
	for {
		tok, err := p.next()
		if err != nil || tok[0] == '}' {
			return err
		}
		if err := p.parseFieldVal(out, seen, tok); err != nil {
			return err
		}
	}
}

func (p *parser) parsePossiblyRepeatedVal(fieldVal reflect.Value, parsedFields map[string]bool, tok, field []byte) error {
	if fieldVal.Kind() == reflect.Slice && fieldVal.Type() != reflect.TypeFor[[]byte]() {
		if tok[0] == '[' {
			return p.parseList(fieldVal, field)
		}
		fieldVal.Set(reflect.Append(fieldVal, reflect.Zero(fieldVal.Type().Elem())))
		return p.parseVal(fieldVal.Index(fieldVal.Len()-1), tok, field)
	}
	if parsedFields[string(field)] {
		return p.error("duplicate field %q but type is not repeated", field)
	}
	parsedFields[string(field)] = true
	return p.parseVal(fieldVal, tok, field)
}

func (p *parser) parseVal(fieldVal reflect.Value, tok, field []byte) error {
	switch tok[0] {
	case '[':
		return p.error("invalid repeated value")
	case '{':
		return p.parseMessage(fieldVal, field)
	case '\'', '"':
		s, err := p.parseString(tok)
		if err != nil {
			return err
		}
		if _, ok := fieldVal.Interface().(encoding.TextUnmarshaler); ok {
			if fieldVal.Kind() == reflect.Pointer && fieldVal.IsNil() {
				fieldVal.Set(reflect.New(fieldVal.Type().Elem()))
			}
			return fieldVal.Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(s))
		}
		if unmarshaler, ok := fieldVal.Addr().Interface().(encoding.TextUnmarshaler); ok {
			return unmarshaler.UnmarshalText([]byte(s))
		}
		fieldVal := setPtr(fieldVal)
		switch {
		case fieldVal.Kind() == reflect.String:
			fieldVal.SetString(s)
		case fieldVal.Type() == reflect.TypeFor[[]byte]():
			b, err := base64.StdEncoding.DecodeString(s)
			if err != nil {
				return p.error("field %q: bad base64", field)
			}
			fieldVal.Set(reflect.ValueOf(b))
		default:
			return p.error("field %q should have type string (got %s)", field, fieldVal.Type())
		}
		return nil
	}
	switch string(tok) {
	case "true":
		return p.unpackBool(fieldVal, true, field)
	case "false":
		return p.unpackBool(fieldVal, false, field)
	}
	if bytes.ContainsAny(tok, ".eE") {
		n, err := p.parseFloat(tok)
		if err != nil {
			return err
		}
		fieldVal := setPtr(fieldVal)
		switch fieldVal.Kind() {
		case reflect.Float32, reflect.Float64:
			fieldVal.SetFloat(n)
		default:
			return p.error("field %q should have type float64 or float32", field)
		}
		return nil
	}
	n, err := p.parseInt(tok)
	if err != nil {
		return err
	}
	fieldVal = setPtr(fieldVal)
	switch fieldVal.Kind() {
	case reflect.Float32, reflect.Float64:
		fieldVal.SetFloat(float64(n.sgn) * float64(n.n))
		return nil
	}
	min, max, ok := intLimits(fieldVal.Kind())
	if !ok {
		return p.error("field %q should have type int", field)
	}
	if n.sgn < 0 && n.n > min || n.sgn > 0 && n.n > max {
		return p.error("number %d is out of range for %s", n, fieldVal.Kind())
	}
	if min == 0 { // unsigned
		fieldVal.SetUint(n.n)
	} else {
		fieldVal.SetInt(int64(n.sgn) * int64(n.n))
	}
	return nil
}

func (p *parser) parseList(fieldVal reflect.Value, field []byte) error {
	if fieldVal.IsNil() {
		fieldVal.Set(reflect.MakeSlice(fieldVal.Type(), 0, 0))
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
		fieldVal.Set(reflect.Append(fieldVal, reflect.Zero(fieldVal.Type().Elem())))
		if err := p.parseVal(fieldVal.Index(fieldVal.Len()-1), tok, field); err != nil {
			return err
		}
	}
}

func (p *parser) parseFieldVal(out reflect.Value, parsedFields map[string]bool, field []byte) error {
	if b := field[0]; !(b == '_' || 'a' <= b && b <= 'z' || 'A' <= b && b <= 'Z') {
		return p.error("expecting field")
	}
	fieldIdx, ok := p.fieldMap[structField{out.Type(), string(field)}]
	if !ok {
		return p.error("no field named %q", field)
	}
	fieldVal := out.Field(fieldIdx)
	tok, err := p.next()
	if err != nil {
		return err
	}
	switch tok[0] {
	case '{':
		if err := p.parsePossiblyRepeatedVal(fieldVal, parsedFields, tok, field); err != nil {
			return err
		}
	case ':':
		tok, err := p.next()
		if err != nil {
			return err
		}
		if err := p.parsePossiblyRepeatedVal(fieldVal, parsedFields, tok, field); err != nil {
			return err
		}
	default:
		return p.error("expecting colon")
	}
	return nil
}

func (p *parser) parse(out reflect.Value) error {
	seen := make(map[string]bool)
	for {
		tok, err := p.next()
		if err != nil {
			if err == errEOF {
				return nil
			}
			return err
		}
		if err := p.parseFieldVal(out, seen, tok); err != nil {
			return err
		}
	}
}

func setPtr(val reflect.Value) reflect.Value {
	if val.Kind() != reflect.Pointer {
		return val
	}
	if val.IsNil() {
		val.Set(reflect.New(val.Type().Elem()))
	}
	return val.Elem()
}

func intLimits(kind reflect.Kind) (min, max uint64, ok bool) {
	switch kind {
	case reflect.Int:
		return -math.MinInt, math.MaxInt, true
	case reflect.Int8:
		return -math.MinInt8, math.MaxInt8, true
	case reflect.Int16:
		return -math.MinInt16, math.MaxInt16, true
	case reflect.Int32:
		return -math.MinInt32, math.MaxInt32, true
	case reflect.Int64:
		return -math.MinInt64, math.MaxInt64, true
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

func (p *parser) unpackBool(fieldVal reflect.Value, b bool, field []byte) error {
	fieldVal = setPtr(fieldVal)
	if fieldVal.Kind() != reflect.Bool {
		return p.error("field %q should have type bool", field)
	}
	fieldVal.SetBool(b)
	return nil
}

// Unmarshal parses a ccl message and writes the result into v. v must be a
// non-nil pointer to a struct.
//
// Unmarshal accepts a top-level message, which is equivalent to the "message"
// type described above, but without the surrounding braces. For example:
//
//	key1: "val1"
//	key2: "val2"
//
// The following rules describe how ccl types are mapped to Go types:
//
//   - For a pointer type, the field will be set to a non-nil value and the
//     value will be unmarshaled into the inner type.
//   - A number can be unmarshaled into any integral type (i.e. int, uint,
//     int8, etc.), float32 or float64. If the number has a fractional part or
//     exponent, then only float32 and float64 are allowed.
//   - A boolean must be unmarshaled as bool
//   - A list must be unmarshaled into a slice where the slice element type
//     matches the inner values inside the list.
//   - A message is unmarshaled into a struct where the fields of the struct
//     match the message fields.
//
// You can override a field's name using a struct tag "ccl", for example
//
//	type message struct {
//	    MyField int `ccl:"my_field"`
//	}
//
// This message could decode, for example `my_field:5`
//
// A ccl string field can be decoded into a string or []byte, where []byte
// expects a base64-encoded string. If a field has type T where T or *T
// implements [encoding.TextUnmarshaler], then a string value will be decoded
// by calling UnmarshalText. No other customization is supported, this
// isn't encoding/json.
func Unmarshal(data []byte, v any) error {
	val := reflect.ValueOf(v)
	if val.Kind() != reflect.Pointer || val.IsNil() || val.Type().Elem().Kind() != reflect.Struct {
		return fmt.Errorf("value must be a non-nil pointer to a struct")
	}
	fields := make(map[structField]int)
	if err := fieldMap(fields, make(map[reflect.Type]bool), val.Type().Elem()); err != nil {
		return err
	}
	return (&parser{lexer: lexer{data: data}, data: data, fieldMap: fields}).parse(val.Elem())
}
