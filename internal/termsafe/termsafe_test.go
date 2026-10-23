package termsafe

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestEscape(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"BackslashAndQuoteAloneAreUntouched", `a\b"c`, `a\b"c`},
		{"NonASCIIIsUntouched", "héllo → 世界 🎉", "héllo → 世界 🎉"},
		{"OSC52EndedByBEL", "name\x1b]52;c;aGVsbG8=\x07rest", `name\x1b]52;c;aGVsbG8=\arest`},
		{"OSC52EndedByST", "\x1b]52;c;aGVsbG8=\x1b\\", `\x1b]52;c;aGVsbG8=\x1b\`},
		{"CSI", "\x1b[31mred", `\x1b[31mred`},
		{"LoneEscape", "a\x1bb", `a\x1bb`},
		{"C1AsUTF8", "a\u009b31mb", `a\u009b31mb`},
		{"C1AsStrayByte", "a\x9b31mb", `a\x9b31mb`},
		{"Delete", "a\x7fb", `a\x7fb`},
		{"CarriageReturn", "real\rfake", `real\rfake`},
		{"BidiOverride", "a\u202eb", `a\u202eb`},
		{"TabAndNewline", "a\tb\nc", `a\tb\nc`},
		{"QuotesBesideAControl", "\"a\"\x1b", `"a"\x1b`},
		{"BackslashBesideAControl", "a\\b\x1b", `a\b\x1b`},
		{"ZWJEmojiSequence", "👨\u200d👩\u200d👧", "👨\u200d👩\u200d👧"},
		{"ZWNJWord", "می\u200cخواهم", "می\u200cخواهم"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := Escape(test.input)

			require.Equal(t, test.want, got)
			require.Equal(t, got, Escape(got), "Escape must be idempotent")
		})
	}
}

func TestLines(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"ControlSequenceBesideNewlines", "first\x1b[2J line\n\nthird\n", `first\x1b[2J line` + "\n\nthird\n"},
		{"Tab", "a\tb", "a\tb"},
		{"ZWJEmojiSequence", "👨\u200d👩\u200d👧", "👨\u200d👩\u200d👧"},
		{"ZWNJWord", "می\u200cخواهم", "می\u200cخواهم"},
		{"QuotesOnALineWithAnEscapedRune", "say \"hi\"\x1b[2J", `say "hi"\x1b[2J`},
		{"BidiOverride", "a\u202eb", `a\u202eb`},
		{"CarriageReturn", "real\rfake", `real\rfake`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := Lines(test.input)

			require.Equal(t, test.want, got)
			require.Equal(t, got, Lines(got), "Lines must be idempotent")
		})
	}
}

func TestWriter(t *testing.T) {
	t.Parallel()

	var out strings.Builder
	input := "one\x1b[2J\ntwo"

	n, err := Writer{W: &out}.Write([]byte(input))

	require.NoError(t, err)
	require.Equal(t, len(input), n)
	require.Equal(t, `one\x1b[2J`+"\ntwo", out.String())
}

func TestStdoutFollowsOSStdout(t *testing.T) {
	r, w, err := os.Pipe()
	require.NoError(t, err)
	realStdout := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = realStdout })

	_, err = fmt.Fprint(Stdout, "a\x1b[2J")

	require.NoError(t, err)
	require.NoError(t, w.Close())
	got, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, `a\x1b[2J`, string(got))
}

func TestJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"CleanDocumentIsUntouched", "{\n\t\"a\": \"世界\\n\\t\\\"\\\\\\/\\u00e9\\ud83d\\ude00\"\n}", "{\n\t\"a\": \"世界\\n\\t\\\"\\\\\\/\\u00e9\\ud83d\\ude00\"\n}"},
		{"RawEscape", "{\"a\":\"\x1b[2J\"}", `{"a":"\\x1b[2J"}`},
		{"EncodedEscape", `{"a":"\u001b[2J"}`, `{"a":"\\x1b[2J"}`},
		{"EncodedKey", `{"\u001b[2J":1}`, `{"\\x1b[2J":1}`},
		{"RawBEL", "{\"a\":\"\x07\"}", `{"a":"\\a"}`},
		{"Delete", "{\"a\":\"\x7f\"}", `{"a":"\\x7f"}`},
		{"C1AsUTF8", "{\"a\":\"\u009b\"}", `{"a":"\\u009b"}`},
		{"EncodedC1", `{"a":"\u009b"}`, `{"a":"\\u009b"}`},
		{"BidiOverride", "{\"a\":\"\u202e\"}", `{"a":"\\u202e"}`},
		{"EncodedBidiOverride", `{"a":"\u202e"}`, `{"a":"\\u202e"}`},
		{"ShortEscapes", `{"a":"\b\f\r"}`, `{"a":"\\b\\f\\r"}`},
		{"JoinersAreUntouched", "{\"a\":\"👨\u200d👩 می\u200cخواهم\\u200d\"}", "{\"a\":\"👨\u200d👩 می\u200cخواهم\\u200d\"}"},
		{"FormatRuneAboveBMP", "{\"a\":\"\U000e0001\"}", `{"a":"\\U000e0001"}`},
		{"EncodedFormatRuneAboveBMP", `{"a":"\udb40\udc01"}`, `{"a":"\\U000e0001"}`},
		{"LoneSurrogate", `{"a":"\ud83d"}`, `{"a":"\ud83d"}`},
		{"LoneSurrogateBeforeEscape", `{"a":"\ud83d\u001b"}`, `{"a":"\ud83d\\x1b"}`},
		{"StrayByte", "{\"a\":\"\x9b\"}", `{"a":"\\x9b"}`},
		{"BackslashPairBeforeEscape", `{"a":"\\\u001b"}`, `{"a":"\\\\x1b"}`},
		{"CRLFWhitespace", "{\"a\":1,\r\n\"b\":2}", "{\"a\":1, \n\"b\":2}"},
		{"LoneCRWhitespace", "{\"a\":1,\r\"b\":2}", `{"a":1, "b":2}`},
		{"InvalidEscape", "{\"a\":\"\\q\x1b\"}", `{"a":"\\q\\x1b"}`},
		{"InvalidUnicodeEscape", `{"a":"\uZZZZ"}`, `{"a":"\\uZZZZ"}`},
		{"UnterminatedEscape", `{"a":"\`, `{"a":"\\`},
		{"UnterminatedString", "{\"a\":\"\x1b", `{"a":"\\x1b`},
		{"EscapeOutsideAString", `[1"a,"\u001b"]`, `[1"a,"\\x1b"]`},
		{"NotJSON", "<html>\x1b[2J\n</html>", "<html>\\\\x1b[2J\n</html>"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := JSON(test.input)

			require.Equal(t, test.want, got)
			require.Equal(t, got, JSON(got), "JSON must be idempotent")
		})
	}
}

func TestJSON_LeavesNothingToDecodeToAControlCharacter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		doc  string
		path string
	}{
		{"InvalidHexDecodesToNUL", `["\uZZZZ"]`, "0"},
		{"EscapeAfterAnInvalidEscape", `["\q\u001b"]`, "0"},
		{"StringAfterAMalformedNumber", `[1"a,"\u001b"]`, "1"},
		{"RawControlInAStringWithEscapes", "[\"\\n\x1b\"]", "0"},
		{"EscapeInAKey", `{"\u001b":"\u001b"}`, `\\x1b`},
		{"SurrogatesSplitByARawCR", "[\"\\udb40\r\\udc01\"]", "0"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := gjson.Get(JSON(test.doc), test.path)

			require.True(t, got.Exists())
			require.Equal(t, Lines(got.String()), got.String())
		})
	}
}

func TestJSON_ValidJSONStaysValid(t *testing.T) {
	t.Parallel()

	input := "{\"a\\u001b\":\"x\u009by\u202ez\U000e0001\\u0000\\r\\n\",\r\n\"b\":[1,true,null]}"

	got := JSON(input)

	require.True(t, json.Valid([]byte(got)))
	var decoded map[string]any
	require.NoError(t, json.Unmarshal([]byte(got), &decoded))
	require.Equal(t, `x\u009by\u202ez\U000e0001\x00\r`+"\n", decoded[`a\x1b`])
}

// unescapedOutputFiles are custom code files that still print directly. Remove a file once it moves to this package.
var unescapedOutputFiles = []string{
	"pkg/cmd/apply.go",
	"pkg/cmd/cmd_auth.go",
	"pkg/cmd/cmd_profile.go",
	"pkg/cmd/cmdutil.go",
	"pkg/cmd/otel.go",
	"pkg/cmd/sessionconnect.go",
}

func TestOutputGoesThroughTermsafe(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..")
	var found []string
	for _, dir := range []string{"cmd", "internal", "pkg"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			relative, err := filepath.Rel(root, path)
			if err != nil || slices.Contains(unescapedOutputFiles, filepath.ToSlash(relative)) {
				return err
			}
			source, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, call := range directOutputCalls(t, string(source)) {
				found = append(found, relative+": "+call)
			}
			return nil
		})
		require.NoError(t, err)
	}

	require.Empty(t, found, "Print through termsafe.Stdout or termsafe.Stderr, and show responses with cmd.ShowJSON")
}

func TestDirectOutputCalls(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		call string
		want []string
	}{
		{"Print", `fmt.Print(x)`, []string{"3: fmt.Print"}},
		{"Printf", `fmt.Printf("%s", x)`, []string{"3: fmt.Printf"}},
		{"Println", `fmt.Println(x)`, []string{"3: fmt.Println"}},
		{"FprintfToStdout", `fmt.Fprintf(os.Stdout, "%s", x)`, []string{"3: fmt.Fprintf"}},
		{"FprintlnToStderr", `fmt.Fprintln(os.Stderr, x)`, []string{"3: fmt.Fprintln"}},
		{"WriteStringToStdout", `io.WriteString(os.Stdout, x)`, []string{"3: io.WriteString"}},
		{"StdoutWrite", `os.Stdout.Write(x)`, []string{"3: os.Stdout.Write"}},
		{"StderrWriteString", `os.Stderr.WriteString(x)`, []string{"3: os.Stderr.WriteString"}},
		{"FprintfToTermsafe", `fmt.Fprintf(termsafe.Stderr, "%s", x)`, nil},
		{"FprintfToAnotherWriter", `fmt.Fprintf(w, "%s", x)`, nil},
		{"Sprintf", `fmt.Sprintf("%s", x)`, nil},
		{"OtherWrite", `w.Write(x)`, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := directOutputCalls(t, "package p\nfunc f() {\n"+test.call+"\n}\n")

			require.Equal(t, test.want, got)
		})
	}
}

// directOutputCalls returns each call in source that writes straight to stdout or stderr, as "line: call".
func directOutputCalls(t *testing.T, source string) []string {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", source, 0)
	require.NoError(t, err)

	isStdStream := func(expr ast.Expr) bool {
		name := dottedName(expr)
		return name == "os.Stdout" || name == "os.Stderr"
	}
	var calls []string
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		name := dottedName(call.Fun)
		var direct bool
		switch name {
		case "fmt.Print", "fmt.Printf", "fmt.Println",
			"os.Stdout.Write", "os.Stdout.WriteString", "os.Stderr.Write", "os.Stderr.WriteString":
			direct = true
		case "fmt.Fprint", "fmt.Fprintf", "fmt.Fprintln", "io.WriteString":
			direct = len(call.Args) > 0 && isStdStream(call.Args[0])
		}
		if direct {
			calls = append(calls, fmt.Sprintf("%d: %s", fset.Position(call.Pos()).Line, name))
		}
		return true
	})
	return calls
}

func dottedName(expr ast.Expr) string {
	switch expr := expr.(type) {
	case *ast.Ident:
		return expr.Name
	case *ast.SelectorExpr:
		return dottedName(expr.X) + "." + expr.Sel.Name
	default:
		return ""
	}
}
