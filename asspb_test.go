package asspb

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestUnmarshal(t *testing.T) {
	t.Parallel()

	type nestedMessage struct {
		Field int64 `ccl:"field"`
	}
	type message struct {
		String          string           `ccl:"string"`
		String2         string           `ccl:"string2"`
		Int             int64            `ccl:"int"`
		Float           float64          `ccl:"float"`
		Bool            bool             `ccl:"bool"`
		Bool2           bool             `ccl:"bool2"`
		Message         *nestedMessage   `ccl:"message"`
		Repeated        []int64          `ccl:"repeated"`
		RepeatedMessage []*nestedMessage `ccl:"repeated_message"`

		Ignore     map[int]int `ccl:"-,"` // unlike JSON this also means ignore
		unexported int64
	}

	for _, tc := range []struct {
		desc string
		msg  string
		want message
	}{{
		desc: "Complete",
		msg: `# This is a comment
string: 'asdf\n' # comment end of line
string2: "asdf\n"
int: 10
float: 10.5e13
bool: true
bool2: false
message { field: 10 }
repeated [1, 2, 3]
repeated: 4
repeated [5, 6]
`,
		want: message{
			String:   "asdf\n",
			String2:  "asdf\n",
			Int:      10,
			Float:    10.5e13,
			Bool:     true,
			Bool2:    false,
			Message:  &nestedMessage{Field: 10},
			Repeated: []int64{1, 2, 3, 4, 5, 6},
		},
	}, {
		desc: "MultilineString",
		msg: `string: "strings
can just span multiple lines"`,
		want: message{String: "strings\ncan just span multiple lines"},
	}, {
		desc: "Zero",
		msg:  `int: 0`,
		want: message{Int: 0},
	}, {
		desc: "Hex",
		msg:  `int: 0xff`,
		want: message{Int: 255},
	}, {
		desc: "CapitalHex",
		msg:  `int: 0XfF`,
		want: message{Int: 255},
	}, {
		desc: "HexLeadingZero",
		msg:  `int: 0x0f`,
		want: message{Int: 15},
	}, {
		desc: "Float",
		msg:  `float: 1.5e10`,
		want: message{Float: 1.5e10},
	}, {
		desc: "FloatCapitalE",
		msg:  `float: 1.5E10`,
		want: message{Float: 1.5e10},
	}, {
		desc: "NegativeFloat",
		msg:  `float: -1.5e-10`,
		want: message{Float: -1.5e-10},
	}, {
		desc: "PositiveFloat",
		msg:  `float: +1.5e+10`,
		want: message{Float: 1.5e10},
	}, {
		desc: "Int",
		msg:  `int: 10`,
		want: message{Int: 10},
	}, {
		desc: "NegativeInt",
		msg:  `int: -10`,
		want: message{Int: -10},
	}, {
		desc: "PositiveInt",
		msg:  `int: +10`,
		want: message{Int: 10},
	}, {
		desc: "String",
		msg:  `string: 'asdf'`,
		want: message{String: "asdf"},
	}, {
		desc: "DoubleString",
		msg:  `string: "asdf"`,
		want: message{String: "asdf"},
	}, {
		desc: "StringEscapeSingle",
		msg:  `string: 'ain\'t'`,
		want: message{String: "ain't"},
	}, {
		desc: "DoubleStringEscapeSingle",
		msg:  `string: "won\'t"`,
		want: message{String: "won't"},
	}, {
		desc: "StringEscapeDouble",
		msg:  `string: '\"'`,
		want: message{String: `"`},
	}, {
		desc: "DoubleStringEscapeDouble",
		msg:  `string: "\""`,
		want: message{String: `"`},
	}, {
		desc: "StringEscapeQuestionMark",
		msg:  `string: "\?"`,
		want: message{String: "?"},
	}, {
		desc: "StringEscapeBackslash",
		msg:  `string: '\\'`,
		want: message{String: `\`},
	}, {
		desc: "StringEscapeA",
		msg:  `string: '\a'`,
		want: message{String: "\a"},
	}, {
		desc: "StringEscapeB",
		msg:  `string: '\b'`,
		want: message{String: "\b"},
	}, {
		desc: "StringEscapeF",
		msg:  `string: '\f'`,
		want: message{String: "\f"},
	}, {
		desc: "StringEscapeN",
		msg:  `string: '\n'`,
		want: message{String: "\n"},
	}, {
		desc: "StringEscapeR",
		msg:  `string: '\r'`,
		want: message{String: "\r"},
	}, {
		desc: "StringEscapeT",
		msg:  `string: '\t'`,
		want: message{String: "\t"},
	}, {
		desc: "StringEscapeV",
		msg:  `string: '\v'`,
		want: message{String: "\v"},
	}, {
		desc: "StringHex",
		msg:  `string: '\x0a'`,
		want: message{String: "\n"},
	}, {
		desc: "StringHexHighByte",
		msg:  `string: "\xe4\xb8\x96"`,
		want: message{String: "世"},
	}, {
		desc: "StringUnicode",
		msg:  `string: '\u2014'`,
		want: message{String: "—"},
	}, {
		desc: "StringOctal",
		msg:  `string: '\033'`,
		want: message{String: "\033"},
	}, {
		desc: "Message",
		msg:  `message { field: 10 }`,
		want: message{Message: &nestedMessage{Field: 10}},
	}, {
		desc: "EmptyMessage",
		msg:  `message {}`,
		want: message{Message: &nestedMessage{}},
	}, {
		desc: "Repeated",
		msg: `
					repeated: 1
					repeated: 2`,
		want: message{Repeated: []int64{1, 2}},
	}, {
		desc: "RepeatedList",
		msg:  `repeated: [1, 2]`,
		want: message{Repeated: []int64{1, 2}},
	}, {
		desc: "EmptyList",
		msg:  `repeated: []`,
		want: message{Repeated: []int64{}},
	}, {
		desc: "RepeatedListTrailingComma",
		msg: `repeated: [
					1,
					2,
				]`,
		want: message{Repeated: []int64{1, 2}},
	}, {
		desc: "ListOfMessage",
		msg:  `repeated_message: [{}]`,
		want: message{RepeatedMessage: []*nestedMessage{{}}},
	}, {
		desc: "CStyleComment",
		msg:  `message: /** inline comment **/ {}`,
		want: message{Message: &nestedMessage{}},
	}, {
		desc: "CStyleLineComment",
		msg:  `message: {} // line comment`,
		want: message{Message: &nestedMessage{}},
	}, {
		desc: "ConcatStrings",
		msg:  `string: 'that'"'"'s cool'`,
		want: message{String: "that's cool"},
	}, {
		desc: "RemoveNewline",
		msg: `string: 'remove newline \
from string'`,
		want: message{String: "remove newline from string"},
	}, {
		desc: "RemoveNewlineWindows",
		msg:  "string: 'remove newline \\\r\nfrom string'",
		want: message{String: "remove newline from string"},
	}} {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			var got message
			if err := Unmarshal([]byte(tc.msg), &got); err != nil {
				t.Fatalf("Unmarshal(%q) failed: %s\n", tc.msg, err)
			}
			if diff := cmp.Diff(tc.want, got, cmp.AllowUnexported(message{})); diff != "" {
				t.Errorf("Unmarshal(%q) returned unexpected diff (-want +got):\n%s", tc.msg, diff)
			}
		})
	}
}

func TestUnmarshal_Invalid(t *testing.T) {
	t.Parallel()

	type nestedMessage struct {
		Int int `ccl:"int"`
	}
	type message struct {
		Int         int64           `ccl:"int"`
		String      string          `ccl:"string"`
		Msg         nestedMessage   `ccl:"msg"`
		Repeated    []int64         `ccl:"repeated"`
		RepeatedMsg []nestedMessage `ccl:"repeated_msg"`
	}

	for _, tc := range []struct {
		desc string
		msg  string
	}{{
		desc: "BadNum",
		msg:  `int: .`,
	}, {
		desc: "BadStringEscape",
		msg:  `string: '\g'`,
	}, {
		desc: "BadDoubleStringEscape",
		msg:  `string: "\g"`,
	}, {
		desc: "UnterminatedString",
		msg:  `string: '`,
	}, {
		desc: "UnterminatedDoubleString",
		msg:  `string: "`,
	}, {
		desc: "NoFieldName",
		msg:  `10`,
	}, {
		desc: "MsgNoFieldName",
		msg:  `msg {10}`,
	}, {
		desc: "ListMissingComma",
		msg:  `repeated [1 2]`,
	}, {
		desc: "ListBadVal",
		msg:  `repeated [asdf]`,
	}, {
		desc: "ListBadMsgVal",
		msg:  `repeated_msg [{asdf}]`,
	}, {
		desc: "IntLeadingZero",
		msg:  `int: 0644`,
	}, {
		desc: "InvalidOctal",
		msg:  `string: "\777"`,
	}, {
		desc: "InvalidUTF8",
		msg:  `string: "\x80"`,
	}, {
		desc: "FieldMissingVal",
		msg:  `string`,
	}, {
		desc: "FieldMissingColon",
		msg:  `string "abc"`,
	}} {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			var got message
			err := Unmarshal([]byte(tc.msg), &got)
			if err == nil {
				t.Errorf("Unmarshal(%q) returned success, want error", tc.msg)
			}
		})
	}
}

func TestUnmarshal_InvalidType(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		desc string
		msg  string
		out  any
	}{{
		desc: "Nil",
		msg:  `field {}`,
		out:  (*struct{})(nil),
	}, {
		desc: "Struct",
		msg:  `field {}`,
		out:  new(int),
	}, {
		desc: "Int",
		msg:  `Field: 123`,
		out:  new(struct{ Field string }),
	}, {
		desc: "IntHex",
		msg:  `Field: 0x123`,
		out:  new(struct{ Field string }),
	}, {
		desc: "Float",
		msg:  `Field: 123.`,
		out:  new(struct{ Field int64 }),
	}, {
		desc: "NestedMessage",
		msg:  `Field {Field {}}`,
		out:  new(struct{ Field int64 }),
	}, {
		desc: "True",
		msg:  `F:on`,
		out:  new(struct{ F int64 }),
	}, {
		desc: "False",
		msg:  `F:no`,
		out:  new(struct{ F int64 }),
	}, {
		desc: "List",
		msg:  `F:[]`,
		out:  new(struct{ F int64 }),
	}, {
		desc: "RepeatedBool",
		msg:  `F:on F:no`,
		out:  new(struct{ F []int64 }),
	}, {
		desc: "String",
		msg:  `F:"abc"`,
		out:  new(struct{ F int64 }),
	}} {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			err := Unmarshal([]byte(tc.msg), tc.out)
			if err == nil {
				t.Errorf("Unmarshal(%+v) returned success, want error", tc.out)
			}
		})
	}
}

func TestUnmarshal_ErrorLineCol(t *testing.T) {
	t.Parallel()

	type message struct {
		Secret int64 `ccl:"secret"`
	}

	msg := `
		###### This is a very important file please do not modify
		#########################################################
		################ The more ## I put the more secure it is######
		secret:12345; # oops typo
	`
	err := Unmarshal([]byte(msg), new(message))
	syntaxErr, ok := err.(*syntaxError)
	if !ok {
		t.Errorf("Unmarshal(%q): expected *syntaxError, got error %T %[2]v", msg, err)
	}
	want := &syntaxError{line: 5, col: 15}
	if syntaxErr.line != want.line || syntaxErr.col != want.col {
		t.Errorf("Unmarshal(%q) returned error %+v, want line %d, col %d", msg, syntaxErr, want.line, want.col)
	}
}
