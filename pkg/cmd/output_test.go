package cmd

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type rawJSONItem string

func (r rawJSONItem) RawJSON() string { return string(r) }

func TestSafeIterator(t *testing.T) {
	t.Parallel()

	t.Run("ItemWithRawJSONKeepsItsTextAndIsEscaped", func(t *testing.T) {
		t.Parallel()

		iter := &sliceIterator[rawJSONItem]{items: []rawJSONItem{`{"a":"x&y` + encodedOSC52 + `"}`}}
		safe := &safeIterator[rawJSONItem]{Iterator: iter}

		require.True(t, safe.Next())

		assert.Equal(t, `{"a":"x&y`+jsonEscapedOSC52+`"}`, safe.Current().RawJSON())
	})

	t.Run("ItemWithoutRawJSONIsEscaped", func(t *testing.T) {
		t.Parallel()

		iter := &sliceIterator[map[string]any]{items: []map[string]any{{"a": rawOSC52}}}
		safe := &safeIterator[map[string]any]{Iterator: iter}

		require.True(t, safe.Next())

		assert.Equal(t, `{"a":"`+jsonEscapedOSC52+`"}`, safe.Current().RawJSON())
	})

	t.Run("ItemThatCannotBeMarshaledEndsTheIteration", func(t *testing.T) {
		t.Parallel()

		iter := &sliceIterator[chan int]{items: []chan int{make(chan int)}}
		safe := &safeIterator[chan int]{Iterator: iter}

		next := safe.Next()

		assert.False(t, next)
		var unsupported *json.UnsupportedTypeError
		assert.ErrorAs(t, safe.Err(), &unsupported)
	})
}
