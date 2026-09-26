package security

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	// MaxTimestampDriftSeconds bounds how far a client timestamp may differ from server time.
	MaxTimestampDriftSeconds = 120
	// MinDurationMS / MaxDurationMS bound a plausible single game session.
	MinDurationMS = 3000
	MaxDurationMS = 3600000
	// MaxScorePointsPerSecond is the upper bound used to reject fabricated scores.
	MaxScorePointsPerSecond = 200
	// MinNameRunes / MaxNameRunes bound the displayed player name.
	MinNameRunes = 2
	MaxNameRunes = 24
)

var invisibleRunes = map[rune]bool{
	'\u200B': true, '\u200C': true, '\u200D': true, '\u200E': true, '\u200F': true,
	'\u2028': true, '\u2029': true, '\u202A': true, '\u202B': true, '\u202C': true,
	'\u202D': true, '\u202E': true, '\u2060': true, '\uFEFF': true,
}

func ValidateScore(score int64, durationMS int, timestamp int64, now time.Time) error {
	drift := now.Unix() - timestamp
	if drift < -MaxTimestampDriftSeconds || drift > MaxTimestampDriftSeconds {
		return errors.New("timestamp drift excessive; check device time")
	}
	if score < 0 {
		return errors.New("score cannot be negative")
	}
	if durationMS < MinDurationMS || durationMS > MaxDurationMS {
		return errors.New("invalid game duration")
	}

	maxAllowed := int64(durationMS/1000) * MaxScorePointsPerSecond
	if score > maxAllowed {
		return fmt.Errorf("score anomaly: %d exceeds maximum theoretical rate (%d)", score, maxAllowed)
	}
	return nil
}

// SanitizePlayerName validates a player name. The returned name is the raw
// player-supplied text and is what gets stored; profanity is not stripped here
// because CensoredName is applied on the way out to clients instead, which
// keeps the stored value faithful to what the player typed.
func SanitizePlayerName(raw string) (string, error) {
	if !utf8.ValidString(raw) {
		return "", errors.New("player name must be valid UTF-8")
	}
	for _, r := range raw {
		if unicode.IsControl(r) || invisibleRunes[r] {
			return "", errors.New("player name contains forbidden invisible or control characters")
		}
	}

	// TrimSpace only removes outer space; remaining runes already passed the checks above.
	name := strings.TrimSpace(raw)
	if length := utf8.RuneCountInString(name); length < MinNameRunes || length > MaxNameRunes {
		return "", fmt.Errorf("player name must be between %d and %d characters", MinNameRunes, MaxNameRunes)
	}
	return name, nil
}
