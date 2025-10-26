package asspb

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestUnmarshal(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		desc string
		msg  string
		want map[string]any
	}{{
		desc: "Complete",
		msg: `# This is a comment
field_string: 'asdf\n' # comment end of line
field_doublestring: "asdf\n"
field_int: 10
field_float: 10.5e13
field_true: true
field_false: false
field_nested { asdf: 10 }
field_repeated [1, 2, 3]
field_repeated: 4
field_repeated [5, 6]
`,
		want: map[string]any{
			"field_string":       "asdf\n",
			"field_doublestring": "asdf\n",
			"field_int":          10.,
			"field_float":        10.5e13,
			"field_true":         true,
			"field_false":        false,
			"field_nested":       map[string]any{"asdf": 10.},
			"field_repeated":     []any{1., 2., 3., 4., 5., 6.},
		},
	}, {
		desc: "MultilineString",
		msg: `field: "strings
can just span multiple lines"`,
		want: map[string]any{"field": "strings\ncan just span multiple lines"},
	}, {
		desc: "Zero",
		msg:  `field: 0`,
		want: map[string]any{"field": 0.},
	}, {
		desc: "Hex",
		msg:  `field: 0xff`,
		want: map[string]any{"field": 255.},
	}, {
		desc: "CapitalHex",
		msg:  `field: 0XfF`,
		want: map[string]any{"field": 255.},
	}, {
		desc: "HexLeadingZero",
		msg:  `field: 0x0f`,
		want: map[string]any{"field": 15.},
	}, {
		desc: "Float",
		msg:  `field: 1.5e10`,
		want: map[string]any{"field": 1.5e10},
	}, {
		desc: "FloatCapitalE",
		msg:  `field: 1.5E10`,
		want: map[string]any{"field": 1.5e10},
	}, {
		desc: "NegativeFloat",
		msg:  `field: -1.5e-10`,
		want: map[string]any{"field": -1.5e-10},
	}, {
		desc: "PositiveFloat",
		msg:  `field: +1.5e+10`,
		want: map[string]any{"field": 1.5e10},
	}, {
		desc: "Int",
		msg:  `field: 10`,
		want: map[string]any{"field": 10.},
	}, {
		desc: "NegativeInt",
		msg:  `field: -10`,
		want: map[string]any{"field": -10.},
	}, {
		desc: "PositiveInt",
		msg:  `field: +10`,
		want: map[string]any{"field": 10.},
	}, {
		desc: "String",
		msg:  `field: 'asdf'`,
		want: map[string]any{"field": "asdf"},
	}, {
		desc: "DoubleString",
		msg:  `field: "asdf"`,
		want: map[string]any{"field": "asdf"},
	}, {
		desc: "StringEscapeSingle",
		msg:  `you: 'ain\'t'`,
		want: map[string]any{"you": "ain't"},
	}, {
		desc: "DoubleStringEscapeSingle",
		msg:  `I: "won\'t"`,
		want: map[string]any{"I": "won't"},
	}, {
		desc: "StringEscapeDouble",
		msg:  `field: '\"'`,
		want: map[string]any{"field": `"`},
	}, {
		desc: "DoubleStringEscapeDouble",
		msg:  `field: "\""`,
		want: map[string]any{"field": `"`},
	}, {
		desc: "StringEscapeQuestionMark",
		msg:  `field: "\?"`,
		want: map[string]any{"field": "?"},
	}, {
		desc: "StringEscapeBackslash",
		msg:  `field: '\\'`,
		want: map[string]any{"field": `\`},
	}, {
		desc: "StringEscapeA",
		msg:  `field: '\a'`,
		want: map[string]any{"field": "\a"},
	}, {
		desc: "StringEscapeB",
		msg:  `field: '\b'`,
		want: map[string]any{"field": "\b"},
	}, {
		desc: "StringEscapeF",
		msg:  `field: '\f'`,
		want: map[string]any{"field": "\f"},
	}, {
		desc: "StringEscapeN",
		msg:  `field: '\n'`,
		want: map[string]any{"field": "\n"},
	}, {
		desc: "StringEscapeR",
		msg:  `field: '\r'`,
		want: map[string]any{"field": "\r"},
	}, {
		desc: "StringEscapeT",
		msg:  `field: '\t'`,
		want: map[string]any{"field": "\t"},
	}, {
		desc: "StringEscapeV",
		msg:  `field: '\v'`,
		want: map[string]any{"field": "\v"},
	}, {
		desc: "StringHex",
		msg:  `field: '\x0a'`,
		want: map[string]any{"field": "\n"},
	}, {
		desc: "StringHexHighByte",
		msg:  `field: "\xe4\xb8\x96"`,
		want: map[string]any{"field": "世"},
	}, {
		desc: "StringUnicode",
		msg:  `field: '\u2014'`,
		want: map[string]any{"field": "—"},
	}, {
		desc: "StringOctal",
		msg:  `field: '\033'`,
		want: map[string]any{"field": "\033"},
	}, {
		desc: "Message",
		msg:  `field { nested_field: 10 }`,
		want: map[string]any{"field": map[string]any{"nested_field": 10.}},
	}, {
		desc: "EmptyMessage",
		msg:  `field {}`,
		want: map[string]any{"field": map[string]any{}},
	}, {
		desc: "Repeated",
		msg: `field: 1
field: 2`,
		want: map[string]any{"field": []any{1., 2.}},
	}, {
		desc: "RepeatedList",
		msg:  `field: [1, 2]`,
		want: map[string]any{"field": []any{1., 2.}},
	}, {
		desc: "EmptyList",
		msg:  `field: []`,
		want: map[string]any{"field": []any{}},
	}, {
		desc: "RepeatedListTrailingComma",
		msg: `field: [
			1,
			2,
		]`,
		want: map[string]any{"field": []any{1., 2.}},
	}, {
		desc: "ListOfMessage",
		msg:  `field: [{}]`,
		want: map[string]any{"field": []any{map[string]any{}}},
	}, {
		desc: "CStyleComment",
		msg:  `field: /** inline comment **/ {}`,
		want: map[string]any{"field": map[string]any{}},
	}, {
		desc: "CStyleLineComment",
		msg:  `field: {} // line comment`,
		want: map[string]any{"field": map[string]any{}},
	}, {
		desc: "ConcatStrings",
		msg:  `field: 'that'"'"'s cool'`,
		want: map[string]any{"field": "that's cool"},
	}, {
		desc: "RemoveNewline",
		msg: `field: 'remove newline \
from string'`,
		want: map[string]any{"field": "remove newline from string"},
	}, {
		desc: "RemoveNewlineWindows",
		msg:  "field: 'remove newline \\\r\nfrom string'",
		want: map[string]any{"field": "remove newline from string"},
	}} {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			got := make(map[string]any)
			if err := Unmarshal([]byte(tc.msg), &got); err != nil {
				t.Fatalf("Unmarshal(%q) failed: %s\n", tc.msg, err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Unmarshal(%q) returned unexpected diff (-want +got):\n%s", tc.msg, diff)
			}
		})
	}
}

func TestUnmarshal_Invalid(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		desc string
		msg  string
	}{{
		desc: "BadNum",
		msg:  `field: .`,
	}, {
		desc: "BadStringEscape",
		msg:  `field: '\g'`,
	}, {
		desc: "BadDoubleStringEscape",
		msg:  `field: "\g"`,
	}, {
		desc: "UnterminatedString",
		msg:  `field: '`,
	}, {
		desc: "UnterminatedDoubleString",
		msg:  `field: "`,
	}, {
		desc: "NoFieldName",
		msg:  `10`,
	}, {
		desc: "MsgNoFieldName",
		msg:  `field {10}`,
	}, {
		desc: "ListMissingComma",
		msg:  `field [1 2]`,
	}, {
		desc: "ListBadVal",
		msg:  `field [asdf]`,
	}, {
		desc: "ListBadMsgVal",
		msg:  `field [{asdf}]`,
	}, {
		desc: "IntLeadingZero",
		msg:  `field: 0644`,
	}, {
		desc: "InvalidOctal",
		msg:  `field: "\777"`,
	}, {
		desc: "InvalidUTF8",
		msg:  `field: "\x80"`,
	}} {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			got := make(map[string]any)
			err := Unmarshal([]byte(tc.msg), &got)
			if err == nil {
				t.Errorf("Unmarshal(%q) returned success, want error", tc.msg)
			}
		})
	}
}

func TestUnmarshal_ErrorLineCol(t *testing.T) {
	msg := `
		###### This is a very important file please do not modify
		#########################################################
		################ The more ## I put the more secure it is######
		secret:12345; # oops typo
	`
	err := Unmarshal([]byte(msg), new(map[string]any))
	syntaxErr, ok := err.(*syntaxError)
	if !ok {
		t.Errorf("Unmarshal(%q): expected *syntaxError, got error %T %[2]v", msg, err)
	}
	want := &syntaxError{line: 5, col: 15}
	if syntaxErr.line != want.line || syntaxErr.col != want.col {
		t.Errorf("Unmarshal(%q) returned error %+v, want line %d, col %d", msg, syntaxErr, want.line, want.col)
	}
}
