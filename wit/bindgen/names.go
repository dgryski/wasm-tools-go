package bindgen

import (
	"strings"
	"unicode"

	"github.com/ydnar/wasm-tools-go/internal/go/gen"
)

// GoPackageName generates a Go local package name (e.g. "json").
func GoPackageName(name string) string {
	return strings.Map(func(c rune) rune {
		if notLetterDigit(c) {
			return -1
		}
		return c
	}, strings.ToLower(name))
}

// ExportedName returns an idiomatic (exported CamelCase) Go name for a WIT name.
func ExportedName(name string) string {
	var b strings.Builder
	for _, word := range words(strings.ToLower(name)) {
		if s, ok := CommonWords[word]; ok {
			b.WriteString(s)
		} else if gen.Initialisms[word] {
			b.WriteString(strings.ToUpper(word))
		} else {
			runes := []rune(word)
			runes[0] = unicode.ToUpper(runes[0])
			b.WriteString(string(runes))
		}
	}
	return b.String()
}

// SnakeName returns a snake_case name for a WIT name.
func SnakeName(name string) string {
	return strings.Join(words(strings.ToLower(name)), "_")
}

func words(name string) []string {
	return strings.FieldsFunc(name, notLetterDigit)
}

func notLetterDigit(c rune) bool {
	return !unicode.IsLetter(c) && !unicode.IsDigit(c)
}

// CommonWords maps common WASI words to opinionated Go equivalents.
var CommonWords = map[string]string{
	"cabi":     "CABI",
	"datetime": "DateTime",
	"filesize": "FileSize",
	"ipv4":     "IPv4",
	"ipv6":     "IPv6",
}
