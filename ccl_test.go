package ccl

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func ptr[T any](v T) *T {
	return &v
}

func TestUnmarshal(t *testing.T) {
	t.Parallel()

	type byteSliceWrapper []byte
	type nestedMessage struct {
		Field int64 `ccl:"field"`
	}
	type message struct {
		String          string           `ccl:"string"`
		String2         string           `ccl:"string2"`
		Int             int              `ccl:"int"`
		Int8            int8             `ccl:"int8"`
		Int16           int16            `ccl:"int16"`
		Int32           int32            `ccl:"int32"`
		Int64           int64            `ccl:"int64"`
		Uint            uint             `ccl:"uint"`
		Uint8           uint8            `ccl:"uint8"`
		Uint16          uint16           `ccl:"uint16"`
		Uint32          uint32           `ccl:"uint32"`
		Uint64          uint64           `ccl:"uint64"`
		Float           float64          `ccl:"float"`
		Bool            bool             `ccl:"bool"`
		Bool2           bool             `ccl:"bool2"`
		Message         *nestedMessage   `ccl:"message"`
		Repeated        []int64          `ccl:"repeated"`
		RepeatedMessage []*nestedMessage `ccl:"repeated_message"`
		Bytes           []byte           `ccl:"bytes"`
		BytesWrapper    byteSliceWrapper `ccl:"bytes_wrapper"`
		Time            time.Time        `ccl:"time"`
		TimePointer     *time.Time       `ccl:"time_pointer"`
		IntPointer      *int             `ccl:"int_pointer"`
		RepeatedPointer []*int           `ccl:"repeated_pointer"`

		Ignore     map[int]int `ccl:"-,"` // unlike JSON this also means ignore
		unexported int64
	}

	for _, tc := range []struct {
		desc string
		msg  string
		want message
	}{{
		desc: "Complete",
		msg: `
			# This is a comment
			string: 'asdf\n' # comment end of line
			string2: "asdf\n"
			int: 10
			float: 10.5e13
			bool: true
			bool2: false
			message { field: 10 }
			repeated: [1, 2, 3]
			repeated: 4
			repeated: [5, 6]
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
		desc: "NegativeHex",
		msg:  `int: -0x0f`,
		want: message{Int: -15},
	}, {
		desc: "PositiveHex",
		msg:  `int: +0x0f`,
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
		desc: "Int8",
		msg:  `int8:1`,
		want: message{Int8: 1},
	}, {
		desc: "Int16",
		msg:  `int16:1`,
		want: message{Int16: 1},
	}, {
		desc: "Int32",
		msg:  `int32:1`,
		want: message{Int32: 1},
	}, {
		desc: "Int64",
		msg:  `int64:1`,
		want: message{Int64: 1},
	}, {
		desc: "Uint",
		msg:  `uint:1`,
		want: message{Uint: 1},
	}, {
		desc: "Uint8",
		msg:  `uint8:1`,
		want: message{Uint8: 1},
	}, {
		desc: "Uint16",
		msg:  `uint16:1`,
		want: message{Uint16: 1},
	}, {
		desc: "Uint32",
		msg:  `uint32:1`,
		want: message{Uint32: 1},
	}, {
		desc: "Uint64",
		msg:  `uint64:1`,
		want: message{Uint64: 1},
	}, {
		desc: "IntFloat",
		msg:  `float:-1`,
		want: message{Float: -1},
	}, {
		desc: "UintFloat",
		msg:  `float:1`,
		want: message{Float: 1},
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
		desc: "StringBigUnicode",
		msg:  `string: '\U0001f600'`,
		want: message{String: "😀"},
	}, {
		desc: "StringOctal",
		msg:  `string: '\033'`,
		want: message{String: "\033"},
	}, {
		desc: "StringOctalOne",
		msg:  `string:'\0asdf'`,
		want: message{String: "\x00asdf"},
	}, {
		desc: "StringOctalTwo",
		msg:  `string:'\33['`,
		want: message{String: "\033["},
	}, {
		desc: "StringOctalFour",
		msg:  `string:'\1234'`,
		want: message{String: "\1234"},
	}, {
		desc: "StringHexOne",
		msg:  `string:'\xfhello'`,
		want: message{String: "\x0fhello"},
	}, {
		desc: "StringHexThree",
		msg:  `string:'\x0ff'`,
		want: message{String: "\x0ff"},
	}, {
		desc: "StringStripCarriageReturn",
		msg:  "string:'a\r\nb'",
		want: message{String: "a\nb"},
	}, {
		desc: "StringTab",
		msg:  "string:'\t'",
		want: message{String: "\t"},
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
		desc: "RepeatedSingle",
		msg:  `repeated: 1`,
		want: message{Repeated: []int64{1}},
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
	}, {
		desc: "Base64",
		msg:  `bytes:"dGVzdA=="`,
		want: message{Bytes: []byte("test")},
	}, {
		desc: "NotBase64",
		msg:  `bytes_wrapper: [1, 2, 3]`,
		want: message{BytesWrapper: byteSliceWrapper{1, 2, 3}},
	}, {
		desc: "TextUnmarshaler",
		msg:  `time:"2025-10-28T07:41:47Z"`,
		want: message{Time: time.Date(2025, time.October, 28, 7, 41, 47, 0, time.UTC)},
	}, {
		desc: "TextUnmarshalerPointer",
		msg:  `time_pointer:"2025-10-28T07:41:47Z"`,
		want: message{TimePointer: ptr(time.Date(2025, time.October, 28, 7, 41, 47, 0, time.UTC))},
	}, {
		desc: "IntPointer",
		msg:  `int_pointer: 5`,
		want: message{IntPointer: ptr(5)},
	}, {
		desc: "RepeatedPointer",
		msg:  `repeated_pointer: [1, 2, 3]`,
		want: message{RepeatedPointer: []*int{ptr(1), ptr(2), ptr(3)}},
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
		Int            int64             `ccl:"int"`
		Int8           int8              `ccl:"int8"`
		Float          float64           `ccl:"float"`
		String         string            `ccl:"string"`
		Msg            nestedMessage     `ccl:"msg"`
		Repeated       []int64           `ccl:"repeated"`
		RepeatedMsg    []nestedMessage   `ccl:"repeated_msg"`
		Bytes          []byte            `ccl:"bytes"`
		NestedRepeated [][]nestedMessage `ccl:"nested_repeated"`
	}

	for _, tc := range []struct {
		desc string
		msg  string
		want *syntaxError
	}{{
		desc: "BadNum",
		msg:  `int: .`,
		want: &syntaxError{line: 1, col: 6},
	}, {
		desc: "BadHex",
		msg:  `int:0xgg`,
		want: &syntaxError{line: 1, col: 5},
	}, {
		desc: "BadStringEscape",
		msg:  `string: '\g'`,
		want: &syntaxError{line: 1, col: 10},
	}, {
		desc: "BadDoubleStringEscape",
		msg:  `string: "\g"`,
		want: &syntaxError{line: 1, col: 10},
	}, {
		desc: "StringBadReturnEscape",
		msg:  "string:'\\\r'",
		want: &syntaxError{line: 1, col: 9},
	}, {
		desc: "StringBadHex",
		msg:  `string:"\xgg"`,
		want: &syntaxError{line: 1, col: 9},
	}, {
		desc: "StringShortUnicode",
		msg:  `string:"\u001"`,
		want: &syntaxError{line: 1, col: 9},
	}, {
		desc: "StringBadUnicode",
		msg:  `string:"\ugggg"`,
		want: &syntaxError{line: 1, col: 9},
	}, {
		desc: "StringControlCharacter",
		msg:  "string:'\a'",
		want: &syntaxError{line: 1, col: 9},
	}, {
		desc: "StringCarriageReturnNotFollowedByNewline",
		msg:  "string:'\r'",
		want: &syntaxError{line: 1, col: 9},
	}, {
		desc: "UnterminatedString",
		msg:  `string: '`,
		want: &syntaxError{line: 1, col: 9},
	}, {
		desc: "UnterminatedDoubleString",
		msg:  `string: "`,
		want: &syntaxError{line: 1, col: 9},
	}, {
		desc: "NoFieldName",
		msg:  `10`,
		want: &syntaxError{line: 1, col: 1},
	}, {
		desc: "MsgNoFieldName",
		msg:  `msg {10}`,
		want: &syntaxError{line: 1, col: 6},
	}, {
		desc: "ListMissingColon",
		msg:  `repeated []`,
		want: &syntaxError{line: 1, col: 10},
	}, {
		desc: "ListMissingComma",
		msg:  `repeated: [1 2]`,
		want: &syntaxError{line: 1, col: 14},
	}, {
		desc: "ListBadVal",
		msg:  `repeated: [asdf]`,
		want: &syntaxError{line: 1, col: 12},
	}, {
		desc: "ListBadMsgVal",
		msg:  `repeated_msg: [{asdf}]`,
		want: &syntaxError{line: 1, col: 17},
	}, {
		desc: "IntLeadingZero",
		msg:  `int: 0644`,
		want: &syntaxError{line: 1, col: 6},
	}, {
		desc: "InvalidOctal",
		msg:  `string: "\777"`,
		want: &syntaxError{line: 1, col: 10},
	}, {
		desc: "InvalidUTF8",
		msg:  `string: "\x80"`,
		want: &syntaxError{line: 1, col: 9},
	}, {
		desc: "FieldMissingVal",
		msg:  `string`,
		want: &syntaxError{line: 1, col: 7},
	}, {
		desc: "FieldMissingColon",
		msg:  `string "abc"`,
		want: &syntaxError{line: 1, col: 8},
	}, {
		desc: "Repeated",
		msg:  `int:5 int:6`,
		want: &syntaxError{line: 1, col: 7},
	}, {
		desc: "IntOutOfRange",
		msg:  `int8:512`,
		want: &syntaxError{line: 1, col: 6},
	}, {
		desc: "IntOutOfRangeNegative",
		msg:  `int8:-512`,
		want: &syntaxError{line: 1, col: 6},
	}, {
		desc: "Base64",
		msg:  `bytes:"dGVzdAo"`,
		want: &syntaxError{line: 1, col: 7},
	}, {
		desc: "NotBase64",
		msg:  `bytes:[1,2,3]`,
		want: &syntaxError{line: 1, col: 7},
	}, {
		desc: "BadField",
		msg:  `asdfasdfasdf:"asdf"`,
		want: &syntaxError{line: 1, col: 1},
	}, {
		desc: "NestedRepeated",
		msg:  `repeated: [[1]]`,
		want: &syntaxError{line: 1, col: 12},
	}, {
		desc: "NestedRepeatedNestedType",
		msg:  `nested_repeated: [[{}]]`,
		want: &syntaxError{line: 1, col: 19},
	}, {
		desc: "FloatMissingExponent",
		msg:  `float:1e`,
		want: &syntaxError{line: 1, col: 7},
	}, {
		desc: "FloatPositiveMissingExponent",
		msg:  `float:1e+`,
		want: &syntaxError{line: 1, col: 7},
	}, {
		desc: "UnterminatedComment",
		msg:  `/*`,
		want: &syntaxError{line: 1, col: 1},
	}, {
		desc: "BadToken",
		msg: `###### This is a very important file please do not modify
#########################################################
################ The more ## I put the more secure it is######
int:12345; # oops typo
`,
		want: &syntaxError{line: 4, col: 10},
	}, {
		desc: "OutOfRange",
		msg:  `int:20000000000000000000`,
		want: &syntaxError{line: 1, col: 5},
	}, {
		desc: "FloatRange",
		msg:  `float:1e309`,
		want: &syntaxError{line: 1, col: 7},
	}, {
		desc: "IntLetter",
		msg:  `int: 1A`,
		want: &syntaxError{line: 1, col: 6},
	}} {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			err := Unmarshal([]byte(tc.msg), new(message))
			got, ok := err.(*syntaxError)
			if !ok {
				t.Fatalf("Unmarshal(%q): expected *syntaxError, got error %T %[2]v", tc.msg, err)
			}
			if diff := cmp.Diff(tc.want, got, cmp.AllowUnexported(syntaxError{}), cmpopts.IgnoreFields(syntaxError{}, "reason")); diff != "" {
				t.Errorf("Unmarshal(%q) returned unexpected error diff (-want +got):\n%s", tc.msg, diff)
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
		desc: "NilMap",
		msg:  `field {}`,
		out:  map[string]any(nil),
	}, {
		desc: "Struct",
		msg:  `field {}`,
		out:  new(int),
	}, {
		desc: "Int",
		msg:  `Field: 123`,
		out:  new(struct{ Field string }),
	}, {
		desc: "NegativeInt",
		msg:  `Field: -123`,
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
		out:  new(struct{ Field struct{ Field int64 } }),
	}, {
		desc: "True",
		msg:  `F:true`,
		out:  new(struct{ F string }),
	}, {
		desc: "False",
		msg:  `F:false`,
		out:  new(struct{ F string }),
	}, {
		desc: "List",
		msg:  `F:[]`,
		out:  new(struct{ F int64 }),
	}, {
		desc: "RepeatedBool",
		msg:  `F:true F:false`,
		out:  new(struct{ F []string }),
	}, {
		desc: "String",
		msg:  `F:"abc"`,
		out:  new(struct{ F int64 }),
	}, {
		desc: "Option",
		msg:  `F:"abc"`,
		out: new(struct {
			F string `ccl:",asdf"`
		}),
	}, {
		desc: "OptionNested",
		msg:  `F{F:"abc"}`,
		out: new(struct {
			F struct {
				F string `ccl:",asdf"`
			}
		}),
	}, {
		desc: "OptionNestedPointer",
		msg:  `F{F:"abc"}`,
		out: new(struct {
			F *struct {
				F string `ccl:",asdf"`
			}
		}),
	}, {
		desc: "OptionSlicePointer",
		msg:  `F{F:"abc"}`,
		out: new(struct {
			F []*struct {
				F string `ccl:",asdf"`
			}
		}),
	}, {
		desc: "RepeatedTagName",
		msg:  `F:"abc"`,
		out: new(struct {
			F string
			G string `ccl:"F"`
		}),
	}, {
		desc: "RepeatedSingular",
		msg:  `F:[1]`,
		out:  new(struct{ F int }),
	}, {
		desc: "IntTrue",
		msg:  `int:true`,
		out: new(struct {
			F int `ccl:"int"`
		}),
	}, {
		desc: "IntFalse",
		msg:  `int:false`,
		out: new(struct {
			F int `ccl:"int"`
		}),
	}, {
		desc: "UintTrue",
		msg:  `uint:true`,
		out: new(struct {
			F uint `ccl:"uint"`
		}),
	}, {
		desc: "UintFalse",
		msg:  `uint:false`,
		out: new(struct {
			F uint `ccl:"uint"`
		}),
	}} {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			err := Unmarshal([]byte(tc.msg), tc.out)
			if err == nil {
				t.Errorf("Unmarshal(%q) returned %+v, want error", tc.msg, tc.out)
			}
		})
	}
}

func ExampleUnmarshal() {
	// Pretend this was loaded from a file
	msg := []byte(`
		server {
		    listen: "0.0.0.0:80"
		    listen: "[::0]:80"
		    location {
		        path: "/"
		        return: "301 https://$host$request_uri" # redirect to https
		    }
		    location {
		        path: "/.well-known/acme-challenge/"
		        root: "/var/lib/acme/acme-challenge"
		        auth_basic: false
		        auth_request: false
		    }
		}
	`)
	var serverConfig struct {
		Server struct {
			Listen   []string `ccl:"listen"`
			Location []struct {
				Path        string `ccl:"path"`
				Root        string `ccl:"root"`
				AuthBasic   bool   `ccl:"auth_basic"`
				AuthRequest bool   `ccl:"auth_request"`
				Return      string `ccl:"return"`
			} `ccl:"location"`
		} `ccl:"server"`
	}
	if err := Unmarshal(msg, &serverConfig); err != nil {
		panic(err)
	}
	fmt.Println(serverConfig.Server.Location[0].Return)
	// Output:
	// 301 https://$host$request_uri
}

func BenchmarkLex(b *testing.B) {
	msg := []byte(`
		# This is a comment
		string: 'asdf\n' # comment end of line
		string2: "asdf\n"
		int: 10
		float: 10.5e13
		bool: true
		bool2: false
		message { field: 10 }
		repeated: [1, 2, 3]
		repeated: 4
		repeated: [5, 6]
	`)
	for b.Loop() {
		l := &lexer{data: msg}
		for {
			_, _, err := l.next()
			if err != nil {
				if err == errEOF {
					break
				}
				b.Fatal(err)
			}
		}
	}
}

func BenchmarkParse(b *testing.B) {
	msg := []byte(`
		# This is a comment
		string: 'asdf\n' # comment end of line
		string2: "asdf\n"
		int: 10
		float: 10.5e13
		bool: true
		bool2: false
		message { field: 10 }
		repeated: [1, 2, 3]
		repeated: 4
		repeated: [5, 6]
	`)
	type message struct {
		String  string  `ccl:"string"`
		String2 string  `ccl:"string2"`
		Int     int     `ccl:"int"`
		Float   float64 `ccl:"float"`
		Bool    bool    `ccl:"bool"`
		Bool2   bool    `ccl:"bool2"`
		Message struct {
			Field int `ccl:"field"`
		} `ccl:"message"`
		Repeated []int `ccl:"repeated"`
	}
	for b.Loop() {
		var m message
		if err := Unmarshal(msg, &m); err != nil {
			b.Fatal(err)
		}
	}
}

func FuzzUnmarshal(f *testing.F) {
	for _, tc := range []string{
		`
			# This is a comment
			string: 'asdf\n' # comment end of line
			string2: "asdf\n"
			int: 10
			float: 10.5e13
			bool: true
			bool2: false
			message { field: 10 }
			repeated: [1, 2, 3]
			repeated: 4
			repeated: [5, 6]
		`,
		`string: "strings
can just span multiple lines"`,
		`int: 0`,
		`int: 0xff`,
		`int: 0XfF`,
		`int: 0x0f`,
		`int: -0x0f`,
		`int: +0x0f`,
		`float: 1.5e10`,
		`float: 1.5E10`,
		`float: -1.5e-10`,
		`float: +1.5e+10`,
		`int: 10`,
		`int: -10`,
		`int: +10`,
		`int8:1`,
		`int16:1`,
		`int32:1`,
		`int64:1`,
		`uint:1`,
		`uint8:1`,
		`uint16:1`,
		`uint32:1`,
		`uint64:1`,
		`int:on`,
		`int:no`,
		`uint:on`,
		`uint:no`,
		`float:-1`,
		`float:1`,
		`string: 'asdf'`,
		`string: "asdf"`,
		`string: 'ain\'t'`,
		`string: "won\'t"`,
		`string: '\"'`,
		`string: "\""`,
		`string: "\?"`,
		`string: '\\'`,
		`string: '\a'`,
		`string: '\b'`,
		`string: '\f'`,
		`string: '\n'`,
		`string: '\r'`,
		`string: '\t'`,
		`string: '\v'`,
		`string: '\x0a'`,
		`string: "\xe4\xb8\x96"`,
		`string: '\u2014'`,
		`string: '\U0001f600'`,
		`string: '\033'`,
		`message { field: 10 }`,
		`message {}`,
		`
			repeated: 1
			repeated: 2`,
		`repeated: [1, 2]`,
		`repeated: []`,
		`repeated: 1`,
		`repeated: [
			1,
			2,
		]`,
		`repeated_message: [{}]`,
		`message: /** inline comment **/ {}`,
		`message: {} // line comment`,
		`string: 'that'"'"'s cool'`,
		`string: 'remove newline \
from string'`,
		"string: 'remove newline \\\r\nfrom string'",
		`bytes:"dGVzdA=="`,
		`bytes_wrapper: [1, 2, 3]`,
		`time:"2025-10-28T07:41:47Z"`,
		`time_pointer:"2025-10-28T07:41:47Z"`,
		`int_pointer: 5`,
		`repeated_pointer: [1, 2, 3]`,
		`int: .`,
		`float:1e+`,
		`int:0xgg`,
		`string: '\g'`,
		`string: "\g"`,
		"string:'\\\r'",
		`string:"\x1"`,
		`string:"\xgg"`,
		`string:"\u001"`,
		`string:"\ugggg"`,
		`string: '`,
		`string: "`,
		`10`,
		`msg {10}`,
		`repeated []`,
		`repeated: [1 2]`,
		`repeated: [asdf]`,
		`repeated_msg: [{asdf}]`,
		`int: 0644`,
		`string: "\777"`,
		`string: "\x80"`,
		`string`,
		`string "abc"`,
		`int:5 int:6`,
		`int8:512`,
		`int8:-512`,
		`bytes:"dGVzdAo"`,
		`bytes:[1,2,3]`,
		`asdfasdfasdf:"asdf"`,
		`repeated: [[1]]`,
		`nested_repeated: [[1]]`,
		`float:1e`,
		`/*`,
		`bytes:100000000000000000000`,
		`float:1e700`,
		`float:1A000`,
	} {
		f.Add([]byte(tc))
	}
	f.Fuzz(func(t *testing.T, input []byte) {
		type byteSliceWrapper []byte
		type nestedMessage struct {
			Field int64 `ccl:"field"`
		}
		var message struct {
			String          string           `ccl:"string"`
			String2         string           `ccl:"string2"`
			Int             int              `ccl:"int"`
			Int8            int8             `ccl:"int8"`
			Int16           int16            `ccl:"int16"`
			Int32           int32            `ccl:"int32"`
			Int64           int64            `ccl:"int64"`
			Uint            uint             `ccl:"uint"`
			Uint8           uint8            `ccl:"uint8"`
			Uint16          uint16           `ccl:"uint16"`
			Uint32          uint32           `ccl:"uint32"`
			Uint64          uint64           `ccl:"uint64"`
			Float           float64          `ccl:"float"`
			Bool            bool             `ccl:"bool"`
			Bool2           bool             `ccl:"bool2"`
			Message         *nestedMessage   `ccl:"message"`
			Repeated        []int64          `ccl:"repeated"`
			RepeatedMessage []*nestedMessage `ccl:"repeated_message"`
			Bytes           []byte           `ccl:"bytes"`
			BytesWrapper    byteSliceWrapper `ccl:"bytes_wrapper"`
			Time            time.Time        `ccl:"time"`
			TimePointer     *time.Time       `ccl:"time_pointer"`
			IntPointer      *int             `ccl:"int_pointer"`
			RepeatedPointer []*int           `ccl:"repeated_pointer"`

			Ignore     map[int]int `ccl:"-,"` // unlike JSON this also means ignore
			unexported int64
		}
		Unmarshal(input, &message)
	})
}
