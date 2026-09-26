package security

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestCensoredName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "leaves a clean name alone", in: "PlayerOne", want: "PlayerOne"},
		{name: "keeps unicode names intact", in: "Nguyễn Văn A", want: "Nguyễn Văn A"},
		{name: "keeps surrounding whitespace", in: "  Ace  ", want: "  Ace  "},

		// The first two runes of a match are replaced by the mask and the rest
		// of the word stays visible, so the name is still recognizable.
		{name: "masks a listed word", in: "fucker", want: "**cker"},
		{name: "masks inside a name", in: "Peter fucker", want: "Peter **cker"},
		{name: "masks a short word", in: "fuck", want: "**ck"},
		{name: "keeps a four-letter word's tail", in: "shit", want: "**it"},
		{name: "keeps the three-letter tail", in: "ass", want: "**s"},
		{name: "masks uppercase", in: "FUCK", want: "**CK"},
		{name: "masks a word after a space", in: "Fuck Player", want: "**ck Player"},
		{name: "masks every occurrence", in: "shit fuck", want: "**it **ck"},
		{name: "masks across punctuation", in: "shit-head", want: "**it-head"},
		{name: "masks a possessive", in: "shit's", want: "**it's"},
		{name: "masks an embedded word", in: "FuckYou", want: "**ckYou"},

		// Suffixed forms of a listed word are detected by stripping the suffix.
		{name: "masks an er suffix", in: "fuckers", want: "**ckers"},
		{name: "masks an ing suffix", in: "fucking", want: "**cking"},
		{name: "masks an ed suffix", in: "fucked", want: "**cked"},
		{name: "masks a plural", in: "shits", want: "**its"},

		// Diacritics and case fold, so accented evasions are still caught.
		{name: "folds diacritics", in: "Đéo", want: "**o"},
		{name: "folds mixed case diacritics", in: "ĐÉO", want: "**O"},
		{name: "masks an accented word mid-name", in: "Phạm Đéo", want: "Phạm **o"},

		// Multi-word slurs match as a consecutive run and are masked word by
		// word, so neither component survives in the output.
		{name: "masks a two-word slur", in: "vcl ngu", want: "**l **u"},
		{name: "masks a run inside a name", in: "vcl ngu Nguyễn", want: "**l **u Nguyễn"},

		// Whole-word matching keeps innocent names that merely contain a listed
		// short word.
		{name: "keeps a word containing ass", in: "Classy", want: "Classy"},
		{name: "keeps a word containing die", in: "Audience", want: "Audience"},
		{name: "keeps bass", in: "bass guitar", want: "bass guitar"},
		{name: "keeps a scunthorpe", in: "Scunthorpe", want: "Scunthorpe"},
		{name: "keeps assess rather than reducing to ass", in: "assess", want: "assess"},
		{name: "keeps a word plus digits", in: "cunt123", want: "cunt123"},

		{name: "handles an empty name", in: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CensoredName(tt.in); got != tt.want {
				t.Fatalf("CensoredName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestCensoredNamePreservesValidUTF8 guards the invariant the leaderboard
// depends on: masking cuts the original text at rune boundaries, so the result
// is always valid UTF-8 and never splits a multi-byte character.
func TestCensoredNamePreservesValidUTF8(t *testing.T) {
	inputs := []string{
		"Đéo", "đẹo", "Nguyễn", "shitNguyễn", "shít", "shit shit", "Phạm Đéo",
		"ĐÉOĐÉO", "vcl ngu nguyễn", strings.Repeat("đéo ", 40),
		strings.Repeat("Nguyễn ", 40),
	}
	for _, in := range inputs {
		got := CensoredName(in)
		if !utf8.ValidString(got) {
			t.Errorf("CensoredName(%q) = %q, which is not valid UTF-8", in, got)
		}
	}
}

// TestCensoredNameFitsStorageColumn ensures a masked name can never be longer
// than the raw column it would be compared against.
func TestCensoredNameFitsStorageColumn(t *testing.T) {
	long := strings.Repeat("shit ", 40)
	if got := utf8.RuneCountInString(CensoredName(long)); got > MaxCensoredNameRunes {
		t.Fatalf("CensoredName() returned %d runes, want at most %d", got, MaxCensoredNameRunes)
	}
}

// TestCensoredNameHidesEveryOffensiveWord re-runs the censoring output through
// the matcher to make sure a masked name cannot itself still match a listed
// word. This is what the mask direction protects: with the leading runes
// removed, no listed word survives in the output.
func TestCensoredNameHidesEveryOffensiveWord(t *testing.T) {
	inputs := []string{
		"shit", "fuck", "fucker", "fuckers", "cunt", "vcl ngu", "Đéo", "ass",
		"Peter fucker", "dickhead", "aces", "asses",
	}
	for _, in := range inputs {
		got := CensoredName(in)
		folded, _, _ := foldText(got)
		for _, token := range scanTokens(folded) {
			if isProfaneWord(string(folded[token.start:token.end])) {
				t.Errorf("CensoredName(%q) = %q, which still contains a listed word", in, got)
				break
			}
		}
	}
}

func TestIsProfaneWord(t *testing.T) {
	profane := []string{"shit", "fuck", "cunt", "deo", "vcl", "ngu", "fucker", "dickhead"}
	for _, word := range profane {
		if !isProfaneWord(word) {
			t.Errorf("isProfaneWord(%q) = false, want true", word)
		}
	}

	clean := []string{"", "player", "classy", "bass", "spidey", "nguyen", "dieu", "assess"}
	for _, word := range clean {
		if isProfaneWord(word) {
			t.Errorf("isProfaneWord(%q) = true, want false", word)
		}
	}
}

func TestFoldTextNormalizesEquivalentSpellings(t *testing.T) {
	same := [][2]string{
		{"Đéo", "deo"},
		{"ĐÉO", "deo"},
		{"đẹo", "deo"},
		{"Nguyễn", "nguyen"},
		{"shit", "shit"},
	}
	for _, pair := range same {
		folded, _, _ := foldText(pair[0])
		if got := string(folded); got != pair[1] {
			t.Errorf("foldText(%q) = %q, want %q", pair[0], got, pair[1])
		}
	}
}

func TestSuffixForms(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{in: "fuck", want: []string{"fuck"}},
		{in: "fucker", want: []string{"fucker", "fuck"}},
		{in: "fuckers", want: []string{"fuckers", "fucker", "fuck"}},
		{in: "fucking", want: []string{"fucking", "fuck"}},
		{in: "shit", want: []string{"shit"}},
		// A stem short enough to be an ordinary word is never produced, which
		// is what keeps "asses" from matching "ass".
		{in: "asses", want: []string{"asses"}},
		{in: "assess", want: []string{"assess"}},
		{in: "", want: []string{""}},
	}
	for _, tt := range tests {
		got := suffixForms(tt.in)
		if strings.Join(got, ",") != strings.Join(tt.want, ",") {
			t.Errorf("suffixForms(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}
