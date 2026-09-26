// Command wordlist regenerates the embedded profanity word lists from their
// upstream sources.
//
// Usage:
//
//	make wordlist
//
// The generated files are committed, so this only needs to run when an
// upstream list changes. Every entry is written one per line so the runtime
// loader can split on newlines; multi-word entries survive that round trip.
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

type source struct {
	name   string
	file   string
	url    string
	doc    string
	symbol string
}

var sources = []source{
	{
		name:   "english",
		file:   "internal/security/wordlist/english.go",
		url:    "https://www.cs.cmu.edu/~biglou/resources/bad-words.txt",
		symbol: "English",
		doc: `// Code generated from the upstream English bad-words list. DO NOT EDIT BY HAND.
//
// Entries are lowercase, space-collapsed, and comment-free, one per line.
// Entries containing characters outside [a-z0-9 ] are dropped here because the
// runtime matcher only understands alphanumeric and space characters.
`,
	},
	{
		name:   "vietnamese",
		file:   "internal/security/wordlist/vietnamese.go",
		url:    "https://raw.githubusercontent.com/blue-eyes-vn/vietnamese-offensive-words/refs/heads/main/vn_offensive_words.txt",
		symbol: "Vietnamese",
		doc: `// Code generated from the upstream Vietnamese offensive-words list. DO NOT EDIT BY HAND.
//
// Entries are lowercase, space-collapsed, and comment-free, one per line.
// Entries containing characters outside the Vietnamese letter set and space are
// dropped here because the runtime matcher only understands alphanumeric and
// space characters.
`,
	},
}

// allowedEntry matches what the runtime matcher can compare: lowercase
// alphanumerics, the Vietnamese letter set, and interior spaces. Vietnamese đ
// is listed explicitly because it has no canonical Unicode decomposition.
var allowedEntry = regexp.MustCompile(`^[0-9a-zàáâãèéêìíòóôõùúýăđĩũơưạảấầẩẫậắằẳẵặẹẻẽếềểễệỉịọỏốồổỗộớờởỡợụủứừửữựỳỵỷỹ ]+$`)

var whitespace = regexp.MustCompile(`\s+`)

var hasAlnum = regexp.MustCompile(`[0-9a-z]`)

func main() {
	timeout := flag.Duration("timeout", 60*time.Second, "per-request download timeout")
	flag.Parse()

	client := &http.Client{Timeout: *timeout}
	for _, src := range sources {
		if err := regenerate(client, src); err != nil {
			log.Fatalf("wordlist %s: %v", src.name, err)
		}
	}
}

func regenerate(client *http.Client, src source) error {
	body, err := fetch(client, src.url)
	if err != nil {
		return err
	}

	seen := make(map[string]struct{})
	var words []string
	var dropped int
	for _, line := range strings.Split(string(body), "\n") {
		entry := strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if entry == "" || strings.HasPrefix(entry, "#") {
			continue
		}
		entry = whitespace.ReplaceAllString(strings.ToLower(entry), " ")
		if !allowedEntry.MatchString(entry) || !hasAlnum.MatchString(entry) {
			dropped++
			continue
		}
		if _, dup := seen[entry]; dup {
			continue
		}
		seen[entry] = struct{}{}
		words = append(words, entry)
	}
	if len(words) == 0 {
		return fmt.Errorf("no usable entries parsed from %s", src.url)
	}
	sort.Strings(words)

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "package wordlist\n\n%s// Source: %s\nconst %s = \"\" +\n", src.doc, src.url, src.symbol)
	for i, word := range words {
		if i == len(words)-1 {
			fmt.Fprintf(&buf, "\t%q\n", word)
			break
		}
		fmt.Fprintf(&buf, "\t%q +\n", word+"\n")
	}

	if err := os.WriteFile(src.file, buf.Bytes(), 0o644); err != nil {
		return err
	}
	log.Printf("%s: wrote %d entries to %s (%d unusable lines dropped)", src.name, len(words), src.file, dropped)
	return nil
}

func fetch(client *http.Client, url string) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %s", resp.Status)
	}
	return io.ReadAll(bufio.NewReader(resp.Body))
}
