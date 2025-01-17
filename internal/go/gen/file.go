package gen

import (
	"bytes"
	"go/format"
	"strings"

	"github.com/ydnar/wasm-tools-go/internal/codec"
)

// File represents a generated file. It may be a Go file
type File struct {
	// Name is the short name of the file.
	// If Name ends in ".go" this file will be treated as a Go file.
	Name string

	// Build contains build tags, serialized as //go:build ...
	// Ignored if this is not a Go file.
	Build string

	// PackageDocs are doc comments that preceed the package declaration.
	// These will be wrapped and each line prefixed with // when serialized.
	// Ignored if this is not a Go file.
	PackageDocs string

	// Package this file belongs to.
	Package *Package

	// Imports maps Go package imports from package path to local name, e.g. {"encoding/json": "json"}.
	Imports map[string]string

	// Content is the file contents.
	Content []byte
}

// IsGo returns true if f represents a Go file.
func (f *File) IsGo() bool {
	return strings.HasSuffix(f.Name, ".go")
}

// Write implements io.Writer.
func (f *File) Write(content []byte) (int, error) {
	f.Content = append(f.Content, content...)
	return len(content), nil
}

// Bytes returns the byte values of this file.
func (f *File) Bytes() ([]byte, error) {
	if !f.IsGo() {
		return f.Content, nil
	}

	var b bytes.Buffer

	if f.Build != "" {
		b.WriteString("//go:build ")
		b.WriteString(f.Build)
		b.WriteString("\n\n")
	}

	if f.PackageDocs != "" {
		b.WriteString(FormatDocComments(f.PackageDocs))
	}
	b.WriteString("package ")
	b.WriteString(f.Package.Name)
	b.WriteString("\n\n")

	if len(f.Imports) > 0 {
		b.Write(Imports(f.Imports))
		b.WriteString("\n\n")
	}

	b.Write(f.Content)

	return format.Source(b.Bytes())
}

// Declare adds a package-scoped identifier to [File] f.
// It additionally checks the file-scoped declarations (local package names).
// It returns the package-unique name (which may be different than name).
func (f *File) Declare(name string) Ident {
	name = Unique(name, IsReserved, HasKey(f.Imports), HasKey(f.Package.Declared))
	f.Package.Declared[name] = true
	return Ident{
		Package: f.Package,
		Name:    name,
	}
}

// Import imports the Go package specified by path, returning the local name for the imported package.
// The path argument may have an optional "#name" suffix to specify the local name.
// The returned local name may differ from the specified local name.
func (f *File) Import(path string) string {
	path, name := ParseSelector(path)
	if f.Imports[path] != "" {
		return f.Imports[path]
	}
	name = Unique(name, IsReserved, HasKey(f.Imports), HasKey(f.Package.Declared))
	f.Imports[path] = name
	return name
}

// Imports returns Go import syntax for imports.
// The imports argument is a map of import path to local name.
func Imports(imports map[string]string) []byte {
	if len(imports) == 0 {
		return nil
	}
	var b bytes.Buffer
	b.WriteString("import (\n")
	for _, path := range codec.SortedKeys(imports) {
		name := imports[path]
		b.WriteRune('\t')
		if path != name && !strings.HasSuffix(path, "/"+name) {
			b.WriteString(name)
			b.WriteRune(' ')
		}
		b.WriteString("\"")
		b.WriteString(path)
		b.WriteString("\"\n")
	}
	b.WriteString(")")
	return b.Bytes()
}
