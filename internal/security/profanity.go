// Package security holds input validation and profanity filtering helpers.
package security

import (
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"threadman-api/internal/security/wordlist"
)

const (
	// MaskReplacement stands in for the masked leading runes of a censored word.
	MaskReplacement = "**"
	// MaskHiddenRunes is how many leading runes of a censored word are replaced
	// by MaskReplacement. The rest of the word stays visible so the name is still
	// recognizable, e.g. "fucker" renders as "**cker".
	MaskHiddenRunes = 2
	// MaxCensoredNameRunes is the display-width cap for a censored name. It
	// matches the VARCHAR(32) column the raw name is stored in, so masking a
	// short raw name can never produce a string the database could not hold.
	MaxCensoredNameRunes = 32
	// maxProfanityScanRunes bounds how much of a name is scanned, so a
	// pathological input cannot make the matcher walk an unbounded string.
	maxProfanityScanRunes = 512
)

// profanityIndex holds the comparison word list. The Vietnamese upstream list
// contains multi-word slurs such as "đéo cái", so phrases are indexed apart
// from single words and only match as a consecutive run.
var profanityIndex struct {
	once          sync.Once
	words         map[string]struct{}
	phrases       map[string]struct{}
	maxPhraseWord int
}

func loadProfanityIndex() {
	words := make(map[string]struct{}, 1800)
	phrases := make(map[string]struct{}, 256)
	maxPhraseWord := 0

	record := func(phrase string) {
		fields := strings.Split(phrase, " ")
		if len(fields) == 1 {
			words[fields[0]] = struct{}{}
			return
		}
		phrases[phrase] = struct{}{}
		if len(fields) > maxPhraseWord {
			maxPhraseWord = len(fields)
		}
	}

	for _, list := range []string{wordlist.English, wordlist.Vietnamese} {
		for _, phrase := range strings.Split(list, "\n") {
			if phrase = strings.TrimSpace(phrase); phrase != "" {
				record(phrase)
			}
		}
	}

	profanityIndex.words = words
	profanityIndex.phrases = phrases
	profanityIndex.maxPhraseWord = maxPhraseWord
}

// suffixForms returns the candidate stems of a folded token so suffixed forms of
// a listed word are caught, e.g. "fuckers" reduces to "fucker" then "fuck". It
// deliberately never returns a stem short enough to be an ordinary word, which
// keeps "assess" from reducing to "ass" and being masked.
func suffixForms(token string) []string {
	forms := []string{token}
	current := token
	for {
		trimmed := ""
		switch {
		case strings.HasSuffix(current, "er") && len(current) > 4:
			trimmed = strings.TrimSuffix(current, "er")
		case strings.HasSuffix(current, "ing") && len(current) > 5:
			trimmed = strings.TrimSuffix(current, "ing")
		case strings.HasSuffix(current, "ed") && len(current) > 4:
			trimmed = strings.TrimSuffix(current, "ed")
		case strings.HasSuffix(current, "es") && len(current) > 4:
			trimmed = strings.TrimSuffix(current, "es")
		case strings.HasSuffix(current, "s") && !strings.HasSuffix(current, "ss") && len(current) > 3:
			trimmed = strings.TrimSuffix(current, "s")
		default:
			return forms
		}
		if len(trimmed) < 4 {
			return forms
		}
		forms = append(forms, trimmed)
		current = trimmed
	}
}

// isProfaneWord reports whether a folded token is listed, directly or as a
// suffixed form of a listed word. Whole-word matching means a listed short word
// embedded in a longer name is left alone, so "Classy" and "bass" survive even
// though "ass" is listed.
func isProfaneWord(token string) bool {
	profanityIndex.once.Do(loadProfanityIndex)
	if token == "" {
		return false
	}
	for _, form := range suffixForms(token) {
		if _, found := profanityIndex.words[form]; found {
			return true
		}
	}
	return false
}

// foldRune lowercases a rune and strips its diacritics. Decomposing first is
// what makes a precomposed "é" and a decomposed "e"+U+0301 fold identically;
// Vietnamese đ is special-cased because it has no canonical decomposition.
func foldRune(r rune) rune {
	if r == 'Đ' || r == 'đ' {
		return 'd'
	}
	folded := norm.NFD.String(string(r))
	for _, c := range folded {
		if unicode.Is(unicode.Mn, c) {
			continue
		}
		return unicode.ToLower(c)
	}
	return r
}

// isWordRune keeps letters and digits. Everything else, including spaces and
// punctuation, separates tokens, so "shit-head" still matches "shit".
func isWordRune(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || unicode.IsLetter(r)
}

// foldText builds the comparison form of s alongside, for every rune it kept,
// the byte offset of that rune in s and its byte length. The offset table lets a
// masked name be cut from the original text so it keeps the player's exact
// spelling and capitalization. Combining marks are dropped because the base
// letter they decorate already folded to its canonical form.
func foldText(s string) (folded []rune, offsets []int, lengths []int) {
	folded = make([]rune, 0, len(s))
	offsets = make([]int, 0, len(s))
	lengths = make([]int, 0, len(s))
	for offset, r := range s {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		folded = append(folded, foldRune(r))
		offsets = append(offsets, offset)
		lengths = append(lengths, utf8.RuneLen(r))
	}
	return folded, offsets, lengths
}

// maskTail returns the visible tail of a censored word: the text from the
// word's start with its first n runes removed, stopping at the word's end so
// the rest of the name is never swallowed. The slice is taken from raw, so the
// player's own spelling and capitalization are preserved.
func maskTail(raw string, offsets, lengths []int, start, end, n int) string {
	head := offsets[start]
	last := end - 1
	wordEnd := offsets[last] + lengths[last]

	skip := 0
	for i := start; i < min(start+n, end); i++ {
		skip += lengths[i]
	}
	if head+skip >= wordEnd {
		return ""
	}
	return raw[head+skip : wordEnd]
}

// token is a whole-word run of word runes within the folded text.
type token struct{ start, end int }

// scanTokens splits the folded text into non-overlapping whole-word runs.
func scanTokens(folded []rune) []token {
	var tokens []token
	for i := 0; i < len(folded); {
		if !isWordRune(folded[i]) {
			i++
			continue
		}
		start := i
		for i < len(folded) && isWordRune(folded[i]) {
			i++
		}
		tokens = append(tokens, token{start: start, end: i})
	}
	return tokens
}

// matchAt resolves the longest censored run beginning at token index i. It
// returns the byte offset in raw where the match starts, the byte offset just
// past its last rune, and the rune bounds of each censored word. A multi-word
// slur wins over a single-word hit at the same position so the longer phrase is
// masked in full.
func matchAt(folded []rune, offsets, lengths []int, tokens []token, i int) (int, int, [][2]int) {
	profanityIndex.once.Do(loadProfanityIndex)

	// phrase reports the span a run of count tokens starting at i covers.
	phrase := func(count int) (int, int, [][2]int) {
		bounds := make([][2]int, 0, count)
		for _, t := range tokens[i : i+count] {
			bounds = append(bounds, [2]int{t.start, t.end})
		}
		last := tokens[i+count-1].end - 1
		return offsets[tokens[i].start], offsets[last] + lengths[last], bounds
	}

	if profanityIndex.maxPhraseWord > 1 {
		for count := min(i+profanityIndex.maxPhraseWord, len(tokens)) - i; count >= 2; count-- {
			fields := make([]string, 0, count)
			for _, t := range tokens[i : i+count] {
				fields = append(fields, string(folded[t.start:t.end]))
			}
			if _, found := profanityIndex.phrases[strings.Join(fields, " ")]; found {
				return phrase(count)
			}
		}
	}

	if t := tokens[i]; isProfaneWord(string(folded[t.start:t.end])) {
		return phrase(1)
	}
	return 0, 0, nil
}

// CensoredName masks offensive words for display, replacing the first
// MaskHiddenRunes runes of each match with MaskReplacement and keeping the rest,
// so "fucker" renders as "**cker" and "shit" as "**it". The raw name is never
// modified; this is applied on the way out to clients only.
//
// Matching is whole-word plus common suffixes, so a listed short word embedded
// in a longer name is left alone and "Classy" survives even though "ass" is
// listed. Comparison folds case and diacritics, so accented and upper-case
// evasions are still caught.
//
// The result is capped at MaxCensoredNameRunes so a masked name is never wider
// than the column the raw name was stored in.
func CensoredName(raw string) string {
	if raw == "" {
		return raw
	}
	if utf8.RuneCountInString(raw) > maxProfanityScanRunes {
		raw = string([]rune(raw)[:maxProfanityScanRunes])
	}

	folded, offsets, lengths := foldText(raw)
	tokens := scanTokens(folded)
	if len(tokens) == 0 {
		return raw
	}

	var out strings.Builder
	consumed := 0 // byte offset in raw up to which output is already written
	writeUntil := func(offset int) {
		if offset > consumed {
			out.WriteString(raw[consumed:offset])
			consumed = offset
		}
	}

	for i := 0; i < len(tokens); {
		maskStart, maskEnd, matched := matchAt(folded, offsets, lengths, tokens, i)
		if len(matched) == 0 {
			i++
			continue
		}
		writeUntil(maskStart)
		// Each censored word keeps its tail but loses its leading runes. A
		// multi-word slur is masked word by word, and the separator between the
		// masked words is dropped, so no listed word survives anywhere in the
		// output.
		for _, bounds := range matched {
			out.WriteString(MaskReplacement)
			out.WriteString(maskTail(raw, offsets, lengths, bounds[0], bounds[1], MaskHiddenRunes))
		}
		// Everything the match covers is now emitted in masked form, so the
		// untouched tail of raw starts after its last rune.
		consumed = maskEnd
		// Advance past every token the match covers. Counting tokens rather
		// than comparing byte offsets keeps a multi-word phrase from being
		// revisited, which would otherwise duplicate the text after it.
		for range matched {
			i++
		}
	}
	writeUntil(len(raw))
	out.WriteString(raw[consumed:])

	return truncateRunes(out.String(), MaxCensoredNameRunes)
}

// truncateRunes shortens s to at most limit runes, never splitting a rune.
func truncateRunes(s string, limit int) string {
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	return string([]rune(s)[:limit])
}
