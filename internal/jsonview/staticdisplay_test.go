package jsonview

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormatJSON_EscapesControlSequences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want string
	}{
		{"StringValue", `{"a": "x` + jsonOSC52 + `y"}`, `x\x1b]52;c;aGVsbG8=\ay`},
		{"ObjectKey", `{"a` + jsonCSI + `": 1}`, `a\x1b[31m:`},
		{"ArrayItem", `["x` + jsonOSC52 + `"]`, `x\x1b]52;c;aGVsbG8=\a`},
		{"Number", "[1\u009b]", `1\\u009b`},
		{"NewlineInStringIsKept", `{"a": "one` + jsonCSI + `\ntwo"}`, "one\\x1b[31m\ntwo"},
		{"NonASCIIIsUntouched", "{\"a\": \"héllo 世界 👨\u200d👩\u200d👧\"}", "héllo 世界 👨\u200d👩\u200d👧"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := formatJSON(safeJSON(test.body), 80)

			require.Contains(t, got, test.want)
		})
	}
}

func TestFormatJSON_FindsValuesOfKeysThatGJSONPathsTreatAsSpecial(t *testing.T) {
	t.Parallel()

	got := formatJSON(safeJSON(`{"a.b`+jsonCSI+`": "value"}`), 80)

	require.Contains(t, got, "value")
}

func TestFormatJSON_TruncatesAfterEscaping(t *testing.T) {
	t.Parallel()

	got := formatJSON(safeJSON(`"abcd`+jsonOSC52+`"`), 8)

	require.Contains(t, got, `abcd\x1…`)
}
