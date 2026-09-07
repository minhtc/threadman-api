package security

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var invisibleRunes = map[rune]bool{
	'\u200B': true, '\u200C': true, '\u200D': true, '\u200E': true, '\u200F': true,
	'\u2028': true, '\u2029': true, '\u202A': true, '\u202B': true, '\u202C': true,
	'\u202D': true, '\u202E': true, '\u2060': true, '\uFEFF': true,
}

func ValidateScore(score int64, durationMS int, timestamp int64, now time.Time) error {
	drift := now.Unix() - timestamp
	if drift < -120 || drift > 120 {
		return errors.New("timestamp drift excessive; check device time")
	}
	if score < 0 {
		return errors.New("score cannot be negative")
	}
	if durationMS < 3000 || durationMS > 3600000 {
		return errors.New("invalid game duration")
	}

	maxAllowed := int64(durationMS/1000) * 200
	if score > maxAllowed {
		return fmt.Errorf("score anomaly: %d exceeds maximum theoretical rate (%d)", score, maxAllowed)
	}
	return nil
}

func SanitizePlayerName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if !utf8.ValidString(name) {
		return "", errors.New("player name must be valid UTF-8")
	}
	length := utf8.RuneCountInString(name)
	if length < 2 || length > 24 {
		return "", errors.New("player name must be between 2 and 24 characters")
	}
	for _, r := range name {
		if unicode.IsControl(r) || !utf8.ValidRune(r) || invisibleRunes[r] {
			return "", errors.New("player name contains forbidden invisible or control characters")
		}
	}
	return name, nil
}
