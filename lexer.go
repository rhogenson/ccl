package ccl

import (
	"bytes"
	"iter"
	"unicode"
	"unicode/utf8"
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
		case '\'', '"':
			q := l.data[l.i]
			i := l.i + 1
			for ; i < len(l.data) && l.data[i] != q; i++ {
				if l.data[i] == '\\' {
					i++
				}
			}
			if i >= len(l.data) {
				l.error("unterminated string")
				return
			}
			if !l.yield(i + 1 - l.i) {
				return
			}
			continue
		}
		switch b := l.data[l.i]; {
		case numFirstByte(b):
			i := l.i + 1
			for ; i < len(l.data) && numTailByte(l.data[i]); i++ {
			}
			if !l.yield(i - l.i) {
				return
			}
			continue
		case fieldFirstByte(b):
			i := l.i + 1
			for ; i < len(l.data) && fieldTailByte(l.data[i]); i++ {
			}
			if !l.yield(i - l.i) {
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
