package asspb

import (
	"iter"
	"regexp"
)

type token struct {
	i int
	b []byte
}

type lexer struct {
	data     []byte
	i        int
	yieldTok func(token, error) bool
}

func (l *lexer) error(reason string, args ...any) {
	l.yieldTok(token{}, newSyntaxError(l.data, l.i, reason, args...))
}

func (l *lexer) yield(n int) bool {
	if !l.yieldTok(token{l.i, l.data[l.i : l.i+n]}, nil) {
		return false
	}
	l.i += n
	return true
}

var spaceRE = regexp.MustCompile(`^([[:space:]\p{Zs}]|(#|//)[^\n]*|/\*([^*]|\*[^/])*\*?\*/)*`)

func (l *lexer) skipSpace() {
	l.i += len(spaceRE.Find(l.data[l.i:]))
}

var (
	stringRE       = regexp.MustCompile(`(?s)^(([^'\\]|\\.)*)'`)
	doubleStringRE = regexp.MustCompile(`(?s)^(([^"\\]|\\.)*)"`)
	lexNumRE       = regexp.MustCompile(`^[-+.0-9][-+.0-9a-zA-Z]*`)
	fieldRE        = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z_0-9]*`)
)

func (l *lexer) tokens() {
	for l.i = 0; ; {
		l.skipSpace()
		if l.i == len(l.data) {
			break
		}
		switch l.data[l.i] {
		case
			'{',
			'}',
			'[',
			']',
			':',
			',':

			if !l.yield(1) {
				return
			}
			continue
		case '\'':
			str := stringRE.Find(l.data[l.i+1:])
			if str == nil {
				l.error("invalid string")
				return
			}
			if !l.yield(1 + len(str)) {
				return
			}
			continue
		case '"':
			str := doubleStringRE.Find(l.data[l.i+1:])
			if str == nil {
				l.error("invalid string")
				return
			}
			if !l.yield(1 + len(str)) {
				return
			}
			continue
		}
		if n := lexNumRE.Find(l.data[l.i:]); n != nil {
			if !l.yield(len(n)) {
				return
			}
			continue
		}
		if n := fieldRE.Find(l.data[l.i:]); n != nil {
			if !l.yield(len(n)) {
				return
			}
			continue
		}
		l.error("invalid lexeme")
		return
	}
}

func tokens(data []byte) iter.Seq2[token, error] {
	return func(yield func(token, error) bool) {
		(&lexer{data: data, yieldTok: yield}).tokens()
	}
}
