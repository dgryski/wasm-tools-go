package gen

import (
	"go/ast"
)

// Unique tests name against filters and modifies name until all filters return false.
// Use IsReserved to filter out Go keywords and predeclared identifiers.
//
// Exported names that start with a capital letter will be appended with an underscore.
// Non-exported names are prefixed with an underscore.
func Unique(name string, filters ...func(string) bool) string {
	isExported := ast.IsExported(name)
	filter := func(name string) bool {
		for _, f := range filters {
			if f(name) {
				return true
			}
		}
		return false
	}
	for filter(name) {
		if isExported {
			name += "_"
		} else {
			name = "_" + name
		}
	}
	return name
}

// HasKey returns a function for map m that tests presence of key k.
// The function returns true if m contains k, and false otherwise.
func HasKey[M ~map[K]V, K comparable, V any](m M) func(k K) bool {
	return func(k K) bool {
		_, ok := m[k]
		return ok
	}
}

// IsReserved returns true for any name that is a Go keyword or predeclared identifier.
func IsReserved(name string) bool {
	return reserved[name]
}

var reserved = mapWords(
	// Keywords
	"break",
	"case",
	"chan",
	"const",
	"continue",
	"default",
	"defer",
	"else",
	"fallthrough",
	"for",
	"func",
	"go",
	"goto",
	"if",
	"import",
	"interface",
	"map",
	"package",
	"range",
	"return",
	"select",
	"struct",
	"switch",
	"type",
	"var",

	// Types
	"any",
	"bool",
	"byte",
	"comparable",
	"complex64",
	"complex128",
	"error",
	"float32",
	"float64",
	"int",
	"int8",
	"int16",
	"int32",
	"int64",
	"rune",
	"string",
	"uint",
	"uint8",
	"uint16",
	"uint32",
	"uint64",
	"uintptr",

	// Constants
	"true",
	"false",
	"iota",

	// Zero value
	"nil",

	// Functions
	"append",
	"cap",
	"clear",
	"close",
	"complex",
	"copy",
	"delete",
	"imag",
	"len",
	"make",
	"max",
	"min",
	"new",
	"panic",
	"print",
	"println",
	"real",
	"recover",
)

// Initialisms is a set of common initialisms.
var Initialisms = mapWords(
	"acl",
	"abi",
	"api",
	"ascii",
	"cabi",
	"cpu",
	"css",
	"cwd",
	"dns",
	"eof",
	"fifo",
	"guid",
	"html",
	"http",
	"https",
	"id",
	"imap",
	"io",
	"ip",
	"js",
	"json",
	"lhs",
	"mime",
	"posix",
	"qps",
	"ram",
	"rhs",
	"rpc",
	"sla",
	"smtp",
	"sql",
	"ssh",
	"tcp",
	"tls",
	"ttl",
	"tty",
	"udp",
	"ui",
	"uid",
	"uuid",
	"uri",
	"url",
	"utf8",
	"vm",
	"xml",
	"xmpp",
	"xsrf",
	"xss",
)

func mapWords(words ...string) map[string]bool {
	m := make(map[string]bool, len(words))
	for _, word := range words {
		m[word] = true
	}
	return m
}
