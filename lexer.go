package ccl

import (
	"bytes"
	"unicode"
	"unicode/utf8"
)

type token struct {
	i int
	b []byte
}

type lexer struct {
	data []byte
	i    int
}

func (l *lexer) error(reason string, args ...any) error {
	return newSyntaxError(l.data, l.i, reason, args...)
}

func (l *lexer) yield(n int) token {
	t := token{l.i, l.data[l.i : l.i+n]}
	l.i += n
	return t
}

func (l *lexer) skipSpace() {
	for l.i < len(l.data) {
		if bytes.HasPrefix(l.data[l.i:], []byte("#")) || bytes.HasPrefix(l.data[l.i:], []byte("//")) {
			for ; l.i < len(l.data) && l.data[l.i] != '\n'; l.i++ {
			}
			continue
		}
		if bytes.HasPrefix(l.data[l.i:], []byte("/*")) {
			for ; l.i < len(l.data) && !bytes.HasPrefix(l.data[l.i:], []byte("*/")); l.i++ {
			}
			l.i += 2
			continue
		}
		if r, n := utf8.DecodeRune(l.data[l.i:]); unicode.IsSpace(r) {
			l.i += n
			continue
		}
		break
	}
}

func numFirstByte(b byte) bool {
	return b == '-' ||
		b == '+' ||
		b == '.' ||
		'0' <= b && b <= '9'
}

func numTailByte(b byte) bool {
	return numFirstByte(b) ||
		'a' <= b && b <= 'z' ||
		'A' <= b && b <= 'Z'
}

func fieldFirstByte(b byte) bool {
	return b == '_' ||
		'a' <= b && b <= 'z' ||
		'A' <= b && b <= 'Z'
}

func fieldTailByte(b byte) bool {
	return fieldFirstByte(b) ||
		'0' <= b && b <= '9'
}

func (l *lexer) next() (token, error) {
	l.skipSpace()
	if l.i == len(l.data) {
		return token{}, errEOF
	}
	switch l.data[l.i] {
	case
		'{',
		'}',
		'[',
		']',
		':',
		',':

		return l.yield(1), nil
	case '\'', '"':
		q := l.data[l.i]
		i := l.i + 1
		for ; i < len(l.data) && l.data[i] != q; i++ {
			if l.data[i] == '\\' {
				i++
			}
		}
		if i >= len(l.data) {
			return token{}, l.error("unterminated string")
		}
		return l.yield(i + 1 - l.i), nil
	}
	switch b := l.data[l.i]; {
	case numFirstByte(b):
		i := l.i + 1
		for ; i < len(l.data) && numTailByte(l.data[i]); i++ {
		}
		return l.yield(i - l.i), nil
	case fieldFirstByte(b):
		i := l.i + 1
		for ; i < len(l.data) && fieldTailByte(l.data[i]); i++ {
		}
		return l.yield(i - l.i), nil
	}
	return token{}, l.error("invalid lexeme")
}
