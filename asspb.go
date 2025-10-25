// Me: Mom can we have textproto?
//
// Mom: no we have textproto at home
//
// textproto at home:
package asspb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
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

var spaceRE = regexp.MustCompile(`^([[:space:]\p{Zs}]|#[^\n]*)*`)

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

var escapesRE = regexp.MustCompile(`\\(.|[0-7]{3}|x[0-9a-fA-F]{2}|u[0-9a-fA-F]{4}|U[0-9a-fA-F]{8})`)

func init() {
	escapesRE.Longest()
}

func unescape(idx int, rawStr []byte) (string, error) {
	var err error
	escaped := escapesRE.ReplaceAllFunc(rawStr, func(escape []byte) []byte {
		switch string(escape) {
		case `\'`:
			return []byte("'")
		case `\"`:
			return []byte(`"`)
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
		return "", err
	}
	return string(escaped), nil
}

var stringRE = regexp.MustCompile(`^(([^'\\]|\\.)*)'`)

func (p *parser) parseString() (string, error) {
	p.skipSpace()
	rawStr := stringRE.FindSubmatch(p.data[p.i:])
	if rawStr == nil {
		return "", fmt.Errorf("%d: syntax error: invalid string", p.i)
	}
	s, err := unescape(p.i, rawStr[1])
	if err != nil {
		return "", err
	}
	p.i += len(rawStr[0])
	return s, nil
}

var doubleStringRE = regexp.MustCompile(`^(([^"\\]|\\.)*)"`)

func (p *parser) parseDoubleString() (string, error) {
	p.skipSpace()
	rawStr := doubleStringRE.FindSubmatch(p.data[p.i:])
	if rawStr == nil {
		return "", fmt.Errorf("%d: syntax error: invalid string", p.i)
	}
	s, err := unescape(p.i, rawStr[1])
	if err != nil {
		return "", err
	}
	p.i += len(rawStr[0])
	return s, nil
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
		val, err = p.parseString()
	case p.parseLit(`"`):
		val, err = p.parseDoubleString()
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
func Unmarshal(data []byte, v any) error {
	p := &parser{data: data}
	m, err := p.parse()
	if err != nil {
		return err
	}
	fmt.Printf("%s: %v\n", data, m)
	jsonBytes, err := json.Marshal(m)
	if err != nil {
		return err
	}
	fmt.Printf("%s: %s\n", data, jsonBytes)
	return json.Unmarshal(jsonBytes, v)
}
