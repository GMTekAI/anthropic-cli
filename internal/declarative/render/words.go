package render

import (
	"strings"
	"unicode"
)

// A word-level diff for prose fields. A system prompt is the value most often
// edited and least usefully shown as `"…" → "…"`; what the reader wants is the
// sentence that changed, in place, with a little context either side.

// wordOp says what an edit script does with a run: keep, delete or insert it.
type wordOp int

const (
	wordSame wordOp = iota
	wordDel
	wordIns
)

// wordRun is a stretch of text that an edit script treats as one unit.
type wordRun struct {
	op   wordOp
	text string
}

// maxWordCells bounds the LCS table. Past it the strings are too long to diff
// interactively and the caller falls back to a summary.
const maxWordCells = 4_000_000

// diffWords returns the edit script turning before into after as runs of
// unchanged, deleted and inserted text, or false when the inputs are too large.
func diffWords(before, after string) ([]wordRun, bool) {
	a, b := splitWords(before), splitWords(after)
	if (len(a)+1)*(len(b)+1) > maxWordCells {
		return nil, false
	}

	// lcs[i][j] is the length of the longest common subsequence of a[i:], b[j:].
	lcs := make([][]int32, len(a)+1)
	for i := range lcs {
		lcs[i] = make([]int32, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			switch {
			case a[i] == b[j]:
				lcs[i][j] = lcs[i+1][j+1] + 1
			case lcs[i+1][j] >= lcs[i][j+1]:
				lcs[i][j] = lcs[i+1][j]
			default:
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}

	var runs []wordRun
	emit := func(op wordOp, text string) {
		if n := len(runs); n > 0 && runs[n-1].op == op {
			runs[n-1].text += text
			return
		}
		runs = append(runs, wordRun{op, text})
	}
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] == b[j]:
			emit(wordSame, a[i])
			i, j = i+1, j+1
		case lcs[i+1][j] >= lcs[i][j+1]:
			emit(wordDel, a[i])
			i++
		default:
			emit(wordIns, b[j])
			j++
		}
	}
	for ; i < len(a); i++ {
		emit(wordDel, a[i])
	}
	for ; j < len(b); j++ {
		emit(wordIns, b[j])
	}
	return tidyRuns(runs), true
}

// tidyRuns merges edits that the LCS split around shared whitespace, so
// replacing "quick brown" with "slow red" reads as one deletion and one
// insertion rather than four edits threaded through two spaces. The spaces
// go into both sides, which keeps each side's text intact.
func tidyRuns(runs []wordRun) []wordRun {
	var out []wordRun
	for i := 0; i < len(runs); {
		if runs[i].op == wordSame {
			out = append(out, runs[i])
			i++
			continue
		}
		var del, ins strings.Builder
		j := i
	scan:
		for ; j < len(runs); j++ {
			run := runs[j]
			switch {
			case run.op == wordDel:
				del.WriteString(run.text)
			case run.op == wordIns:
				ins.WriteString(run.text)
			case strings.TrimSpace(run.text) == "" && j+1 < len(runs) && runs[j+1].op != wordSame:
				del.WriteString(run.text)
				ins.WriteString(run.text)
			default:
				break scan
			}
		}
		// Whitespace both sides share at either end is not part of the edit.
		deleted, inserted := del.String(), ins.String()
		lead := commonSpacePrefix(deleted, inserted)
		deleted, inserted = deleted[len(lead):], inserted[len(lead):]
		trail := commonSpaceSuffix(deleted, inserted)
		deleted, inserted = deleted[:len(deleted)-len(trail)], inserted[:len(inserted)-len(trail)]
		if lead != "" {
			out = append(out, wordRun{wordSame, lead})
		}
		if deleted != "" {
			out = append(out, wordRun{wordDel, deleted})
		}
		if inserted != "" {
			out = append(out, wordRun{wordIns, inserted})
		}
		if trail != "" {
			out = append(out, wordRun{wordSame, trail})
		}
		i = j
	}
	return out
}

func commonSpacePrefix(a, b string) string {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] && isSpaceByte(a[i]) {
		i++
	}
	return a[:i]
}

func commonSpaceSuffix(a, b string) string {
	i := 0
	for i < len(a) && i < len(b) {
		ca, cb := a[len(a)-1-i], b[len(b)-1-i]
		if ca != cb || !isSpaceByte(ca) {
			break
		}
		i++
	}
	return a[len(a)-i:]
}

// isSpaceByte reports whether c is one of the ASCII whitespace bytes the word
// diff folds into a shared lead or trail.
func isSpaceByte(c byte) bool { return c == ' ' || c == '\t' || c == '\n' }

// splitWords tokenizes into alternating runs of word and non-word characters,
// keeping every byte so the runs concatenate back to the input.
func splitWords(s string) []string {
	var out []string
	start := 0
	var inWord, started bool
	for i, r := range s {
		w := isWordRune(r)
		if started && w != inWord {
			out = append(out, s[start:i])
			start = i
		}
		inWord, started = w, true
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

// isWordRune reports whether r belongs to a word rather than to the separators
// between words.
func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' }

// isWordToken reports whether a token from splitWords is a word run. Tokens
// are homogeneous, so the first rune decides.
func isWordToken(token string) bool {
	for _, r := range token {
		return isWordRune(r)
	}
	return false
}

// elideContext shortens long unchanged runs to their first and last few words,
// so a one-word edit in a long prompt shows the sentence around it and not the
// whole prompt. keep is the number of words of context to leave on each side.
func elideContext(runs []wordRun, keep int, elided func(words int) string) []wordRun {
	out := make([]wordRun, 0, len(runs))
	for i, run := range runs {
		if run.op != wordSame {
			out = append(out, run)
			continue
		}
		tokens := splitWords(run.text)
		// Count only real words toward the budget; separators ride along.
		var wordPositions []int
		for k, t := range tokens {
			if isWordToken(t) {
				wordPositions = append(wordPositions, k)
			}
		}
		// The first run has no edit before it to give context to, and the last
		// none after it, so those sides keep nothing.
		head, tail := keep, keep
		if i == 0 {
			head = 0
		}
		if i == len(runs)-1 {
			tail = 0
		}
		// The "…N words…" marker reads as about two words, so eliding two or
		// fewer would not shorten the line.
		if len(wordPositions) <= head+tail+2 {
			out = append(out, run)
			continue
		}
		cutFrom, cutTo := 0, len(tokens) // token indices [cutFrom, cutTo) are dropped
		if head > 0 {
			cutFrom = wordPositions[head-1] + 1
		}
		if tail > 0 {
			cutTo = wordPositions[len(wordPositions)-tail]
		}
		dropped := len(wordPositions) - head - tail
		headText, tailText := strings.Join(tokens[:cutFrom], ""), strings.Join(tokens[cutTo:], "")
		text := elided(dropped)
		if headText != "" {
			text = headText + " " + text
		}
		if tailText != "" {
			text = text + " " + tailText
		}
		out = append(out, wordRun{wordSame, text})
	}
	return out
}
