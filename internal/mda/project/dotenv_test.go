package project

import (
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

// dotenvResult is one entry of a parsed `.env`, or "error" for a malformed
// one.
type dotenvResult struct {
	key, value string
}

var dotenvError = dotenvResult{key: "error"}

func parseDotenv(contents string, env map[string]string) []dotenvResult {
	parser := newDotenvParser(contents, func(name string) (string, bool) {
		value, ok := env[name]
		return value, ok
	})
	var results []dotenvResult
	for {
		key, value, err := parser.next()
		switch {
		case errors.Is(err, io.EOF):
			return results
		case err != nil:
			results = append(results, dotenvError)
		default:
			results = append(results, dotenvResult{key, value})
		}
	}
}

// The cases below are dotenvy 0.15.7's own tests.

func TestDotenvParsesKeysValuesAndWhitespace(t *testing.T) {
	// Note 5 spaces after 'KEY8=' below.
	assert.Equal(t, []dotenvResult{
		{"KEY", "1"},
		{"KEY2", "2"},
		{"KEY3", "3"},
		{"KEY4", "fo ur"},
		{"KEY5", "fi ve"},
		{"KEY6", "s ix"},
		{"KEY7", ""},
		{"KEY8", ""},
		{"KEY9", ""},
		{"KEY10", "whitespace before ="},
		{"KEY11", "whitespace after ="},
		{"export", "export as key"},
		{"SHELL_LOVER", "1"},
	}, parseDotenv(`
KEY=1
KEY2="2"
KEY3='3'
KEY4='fo ur'
KEY5="fi ve"
KEY6=s\ ix
KEY7=
KEY8=
KEY9=   # foo
KEY10  ="whitespace before ="
KEY11=    "whitespace after ="
export="export as key"
export   SHELL_LOVER=1
`, nil))
}

func TestDotenvSkipsComments(t *testing.T) {
	assert.Empty(t, parseDotenv("\n# foo=bar\n#    ", nil))
}

func TestDotenvRejectsInvalidLines(t *testing.T) {
	// Note 4 spaces after 'invalid' below.
	assert.Equal(t, []dotenvResult{dotenvError, dotenvError, dotenvError},
		parseDotenv("\n  invalid    \nvery bacon = yes indeed\n=value", nil))
}

func TestDotenvParsesEscapes(t *testing.T) {
	assert.Equal(t, []dotenvResult{
		{"KEY", `my cool value`},
		{"KEY2", `$sweet`},
		{"KEY3", `awesome stuff "mang"`},
		{"KEY4", `sweet $\fgs'fds`},
		{"KEY5", `'"yay\ stuff`},
		{"KEY6", "lol"},
		{"KEY7", "line 1\nline 2"},
	}, parseDotenv(`
KEY=my\ cool\ value
KEY2=\$sweet
KEY3="awesome stuff \"mang\""
KEY4='sweet $\fgs'\''fds'
KEY5="'\"yay\\"\ "stuff"
KEY6="lol" #well you see when I say lol wh
KEY7="line 1\nline 2"
`, nil))
}

func TestDotenvRejectsInvalidValues(t *testing.T) {
	// The unterminated quote on KEY2 runs to the end of the file, taking KEY3
	// and KEY4 with it.
	assert.Equal(t, []dotenvResult{dotenvError, dotenvError}, parseDotenv(`
KEY=my uncool value
KEY2="why
KEY3='please stop''
KEY4=h\8u
`, nil))
}

func TestDotenvJoinsQuotedValuesAcrossLines(t *testing.T) {
	assert.Equal(t, []dotenvResult{{"KEY", "line 1\nline 2"}, {"NEXT", "1"}},
		parseDotenv("KEY=\"line 1\nline 2\"\r\nNEXT=1\n", nil))
}

func TestDotenvRejectsTrailingWhitespaceAtTheEndOfTheFile(t *testing.T) {
	assert.Equal(t, []dotenvResult{{"KEY", "1"}, dotenvError}, parseDotenv("KEY=1\nOTHER=2 ", nil))
}

func TestDotenvSubstitutesVariables(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
		env   map[string]string
		want  []dotenvResult
	}{
		{
			name:  "variable in braces surrounded by quotes",
			input: "KEY=test\nKEY1=\"${KEY}\"",
			want:  []dotenvResult{{"KEY", "test"}, {"KEY1", "test"}},
		},
		{
			name:  "undefined variables become empty",
			input: `KEY=">$KEY1<>${KEY2}<"`,
			want:  []dotenvResult{{"KEY", "><><"}},
		},
		{
			name:  "escaped dollar is not substituted",
			input: `KEY=>\$KEY1<>\${KEY2}<`,
			want:  []dotenvResult{{"KEY", ">$KEY1<>${KEY2}<"}},
		},
		{
			name:  "escaped dollar in weak quotes is not substituted",
			input: `KEY=">\$KEY1<>\${KEY2}<"`,
			want:  []dotenvResult{{"KEY", ">$KEY1<>${KEY2}<"}},
		},
		{
			name:  "strong quotes are not substituted",
			input: `KEY='>${KEY1}<>$KEY2<'`,
			want:  []dotenvResult{{"KEY", ">${KEY1}<>$KEY2<"}},
		},
		{
			name:  "same variable reused",
			input: "KEY=VALUE\nKEY1=$KEY$KEY",
			want:  []dotenvResult{{"KEY", "VALUE"}, {"KEY1", "VALUEVALUE"}},
		},
		{
			name:  "key with a dot",
			input: "KEY.Value=VALUE",
			want:  []dotenvResult{{"KEY.Value", "VALUE"}},
		},
		{
			name:  "recursive substitution",
			input: "KEY=${KEY1}+KEY_VALUE\nKEY1=${KEY}+KEY1_VALUE",
			want:  []dotenvResult{{"KEY", "+KEY_VALUE"}, {"KEY1", "+KEY_VALUE+KEY1_VALUE"}},
		},
		{
			name:  "unbraced variable ends before separators",
			input: "KEY1=test_user\nKEY1_1=test_user_with_separator\nKEY=\">$KEY1_1<>$KEY1}<>$KEY1{<\"",
			want: []dotenvResult{
				{"KEY1", "test_user"},
				{"KEY1_1", "test_user_with_separator"},
				{"KEY", ">test_user_1<>test_user}<>test_user{<"},
			},
		},
		{
			name:  "process environment",
			input: `KEY=">${KEY11}<"`,
			env:   map[string]string{"KEY11": "test_user_env"},
			want:  []dotenvResult{{"KEY", ">test_user_env<"}},
		},
		{
			name:  "process environment overrides the file",
			input: "KEY11=test_user\nKEY=\">${KEY11}<\"",
			env:   map[string]string{"KEY11": "test_user_env"},
			want:  []dotenvResult{{"KEY11", "test_user"}, {"KEY", ">test_user_env<"}},
		},
		{
			name:  "consequent substitutions",
			input: "KEY1=test_user\nKEY2=$KEY1_2\nKEY=>${KEY1}<>${KEY2}<",
			want:  []dotenvResult{{"KEY1", "test_user"}, {"KEY2", "test_user_2"}, {"KEY", ">test_user<>test_user_2<"}},
		},
		{
			name:  "consequent substitutions with one missing",
			input: "KEY2=$KEY1_2\nKEY=>${KEY1}<>${KEY2}<",
			want:  []dotenvResult{{"KEY2", "_2"}, {"KEY", "><>_2<"}},
		},
		{
			name:  "unfinished substitution",
			input: "KEY=VALUE\nKEY1=>${KEY{<",
			want:  []dotenvResult{{"KEY", "VALUE"}, dotenvError},
		},
		{
			name:  "dot as the first character of a key",
			input: ".Key=VALUE",
			want:  []dotenvResult{dotenvError},
		},
		{
			name:  "illegal format",
			input: "<><><>",
			want:  []dotenvResult{dotenvError},
		},
		{
			name:  "illegal escape",
			input: `VALUE=>\f<`,
			want:  []dotenvResult{dotenvError},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, parseDotenv(tc.input, tc.env))
		})
	}
}
