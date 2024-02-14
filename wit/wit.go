package wit

import (
	"fmt"
	"slices"
	"strings"

	"github.com/ydnar/wasm-tools-go/internal/codec"
)

// Node is the interface implemented by the WIT ([WebAssembly Interface Type])
// types in this package.
//
// [WebAssembly Interface Type]: https://component-model.bytecodealliance.org/design/wit.html
type Node interface {
	WITKind() string
	WIT(ctx Node, name string) string
}

func indent(s string) string {
	const ws = "\t"
	return strings.ReplaceAll(strings.TrimSuffix(ws+strings.ReplaceAll(s, "\n", "\n"+ws), ws), ws+"\n", "\n")
}

// unwrap unwraps a multiline string into a single line, if:
// 1. its length is <= 50 chars
// 2. its line count is <= 5
// This is used for single-line [Record], [Flags], [Variant], and [Enum] declarations.
func unwrap(s string) string {
	const chars = 50
	const lines = 5
	if len(s) > chars || strings.Count(s, "\n") > lines || strings.Contains(s, "//") {
		return s
	}
	var b strings.Builder
	for i, line := range strings.Split(s, "\n") {
		if i > 0 {
			b.WriteRune(' ')
		}
		b.WriteString(strings.Trim(line, " \t\r\n"))
	}
	return b.String()
}

// WITKind returns the WIT kind.
func (*Resolve) WITKind() string { return "resolve" }

// WIT returns the [WIT] text format for [Resolve] r. Note that the return value could
// represent multiple files, so may not be precisely valid WIT text.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (r *Resolve) WIT(_ Node, _ string) string {
	var b strings.Builder
	for i, p := range r.Packages {
		if i > 0 {
			b.WriteRune('\n')
			b.WriteRune('\n')
		}
		b.WriteString(p.WIT(r, ""))
	}
	return b.String()
}

// WITKind returns the WIT kind.
func (*Docs) WITKind() string { return "docs" }

// WIT returns the [WIT] text format for [Docs] d.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (d *Docs) WIT(_ Node, _ string) string {
	if d.Contents == "" {
		return ""
	}
	var b strings.Builder
	var lineLength = 0
	for _, c := range d.Contents {
		if lineLength == 0 {
			b.WriteString(DocPrefix)
			lineLength = len(DocPrefix)
		}
		switch c {
		case '\n':
			b.WriteRune('\n')
			lineLength = 0
			continue
		case ' ':
			switch {
			case lineLength == len(DocPrefix):
				// Ignore leading spaces
				continue
			case lineLength > LineLength:
				b.WriteRune('\n')
				lineLength = 0
				continue
			}
		default:
			if lineLength == len(DocPrefix) {
				b.WriteRune(' ')
				lineLength++
			}
		}
		b.WriteRune(c)
		lineLength++
	}
	if lineLength != 0 {
		b.WriteRune('\n')
	}
	return b.String()
}

const (
	DocPrefix  = "///"
	LineLength = 80
)

// WITKind returns the WIT kind.
func (*World) WITKind() string { return "world" }

// WIT returns the [WIT] text format for [World] w.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (w *World) WIT(ctx Node, name string) string {
	if name == "" {
		name = w.Name
	}
	var b strings.Builder
	b.WriteString(w.Docs.WIT(ctx, ""))
	b.WriteString("world ")
	b.WriteString(escape(name)) // TODO: compare to w.Name?
	b.WriteString(" {")
	n := 0
	w.AllImports(func(name string, i WorldItem) bool {
		if f, ok := i.(*Function); ok {
			if !f.IsFreestanding() {
				return true
			}
		}
		if n == 0 {
			b.WriteRune('\n')
		}
		b.WriteString(indent(w.itemWIT("import", name, i)))
		b.WriteRune('\n')
		n++
		return true
	})
	w.AllExports(func(name string, i WorldItem) bool {
		if n == 0 {
			b.WriteRune('\n')
		}
		b.WriteString(indent(w.itemWIT("export", name, i)))
		b.WriteRune('\n')
		n++
		return true
	})
	b.WriteRune('}')
	return b.String()
}

func (w *World) itemWIT(motion, name string, v WorldItem) string {
	switch v := v.(type) {
	case *Interface, *Function:
		return motion + " " + v.WIT(w, name) // TODO: handle resource methods?
	case *TypeDef:
		return v.WIT(w, name) // no motion, in Imports only
	}
	panic("BUG: unknown WorldItem")
}

// WITKind returns the WIT kind.
func (*Interface) WITKind() string { return "interface" }

// WIT returns the [WIT] text format for [Interface] i.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (i *Interface) WIT(ctx Node, name string) string {
	if i.Name != nil && name == "" {
		name = *i.Name
	}

	var b strings.Builder

	switch ctx := ctx.(type) {
	case *Package:
		b.WriteString(i.Docs.WIT(ctx, ""))
		b.WriteString("interface ")
		b.WriteString(escape(name))
		b.WriteRune(' ')
	case *World:
		rname := relativeName(i, ctx.Package)
		if rname != "" {
			return escape(rname) + ";"
		}

		// Otherwise, this is an inline interface decl.
		b.WriteString(i.Docs.WIT(ctx, ""))
		b.WriteString(escape(name))
		b.WriteString(": interface ")
	}

	b.WriteRune('{')
	n := 0
	keys := codec.SortedKeys(i.TypeDefs)
	// Emit use statements first
	for _, name := range keys {
		td := i.TypeDefs[name]
		if td.Root().Owner == td.Owner {
			continue // Skip declarations
		}
		if n == 0 || td.Docs.Contents != "" {
			b.WriteRune('\n')
		}
		b.WriteString(indent(td.WIT(i, name)))
		b.WriteRune('\n')
		n++
	}
	// Declarations
	for _, name := range keys {
		td := i.TypeDefs[name]
		if td.Root().Owner != td.Owner {
			continue // Skip use statements
		}
		if n == 0 || td.Docs.Contents != "" {
			b.WriteRune('\n')
		}
		b.WriteString(indent(td.WIT(i, name)))
		b.WriteRune('\n')
		n++
	}
	// Functions
	for _, name := range codec.SortedKeys(i.Functions) {
		f := i.Functions[name]
		if !f.IsFreestanding() {
			continue
		}
		if n == 0 || f.Docs.Contents != "" {
			b.WriteRune('\n')
		}
		b.WriteString(indent(f.WIT(i, name)))
		b.WriteRune('\n')
		n++
	}
	b.WriteRune('}')
	return b.String()
}

// WITKind returns the [WIT] kind.
func (t *TypeDef) WITKind() string {
	// TODO: should this be prefixed with "alias" if t.Root() != t?
	return t.Root().Kind.WITKind()
}

// WIT returns the [WIT] text format for [TypeDef] t.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (t *TypeDef) WIT(ctx Node, name string) string {
	if t.Name != nil && name == "" {
		name = *t.Name
	}
	switch ctx := ctx.(type) {
	case nil:
		return t.Kind.WIT(nil, name)

	// If context is another TypeDef, then this is an imported type.
	case *TypeDef:
		// Emit an type alias if same Owner.
		if t.Owner == ctx.Owner && t.Name != nil {
			return "type " + escape(name) + " = " + escape(*t.Name)
		}
		ownerName := relativeName(t.Owner, ctx.Package())
		if t.Name != nil && *t.Name != name {
			return fmt.Sprintf("use %s.{%s as %s};", ownerName, escape(*t.Name), escape(name))
		}
		return fmt.Sprintf("use %s.{%s};", ownerName, escape(name))

	case *World, *Interface:
		var b strings.Builder
		b.WriteString(t.Docs.WIT(ctx, ""))
		b.WriteString(t.Kind.WIT(t, name))
		constructor := t.Constructor()
		methods := t.Methods()
		statics := t.StaticFunctions()
		if constructor != nil || len(methods) > 0 || len(statics) > 0 {
			b.WriteString(" {\n")
			n := 0
			if constructor != nil {
				b.WriteString(indent(constructor.WIT(t, "constructor")))
				b.WriteRune('\n')
				n++
			}
			slices.SortFunc(methods, functionCompare)
			for _, f := range methods {
				if f.Docs.Contents != "" {
					b.WriteRune('\n')
				}
				b.WriteString(indent(f.WIT(t, "")))
				b.WriteRune('\n')
				n++
			}
			slices.SortFunc(statics, functionCompare)
			for _, f := range statics {
				if f.Docs.Contents != "" {
					b.WriteRune('\n')
				}
				b.WriteString(indent(f.WIT(t, "")))
				b.WriteRune('\n')
				n++
			}
			b.WriteRune('}')
		}
		s := b.String()
		if s[len(s)-1] != '}' && s[len(s)-1] != ';' {
			b.WriteRune(';')
		}
		return b.String()
	}
	if name != "" {
		return escape(name)
	}
	return t.Kind.WIT(ctx, name)
}

func functionCompare(a, b *Function) int {
	return strings.Compare(a.Name, b.Name)
}

func escape(name string) string {
	if witKeywords[name] {
		return "%" + name
	}
	return name
}

var witKeywords = map[string]bool{
	"enum":      true,
	"export":    true,
	"flags":     true,
	"func":      true,
	"import":    true,
	"include":   true,
	"interface": true,
	"package":   true,
	"record":    true,
	"resource":  true,
	"result":    true,
	"static":    true,
	"type":      true,
	"variant":   true,
	"world":     true,
}

func relativeName(o TypeOwner, p *Package) string {
	var op *Package
	var name string
	switch o := o.(type) {
	case *Interface:
		if o.Name == nil {
			return ""
		}
		op = o.Package
		name = *o.Name

	case *World:
		op = o.Package
		name = o.Name
	}
	if op == p {
		return name
	}
	if op == nil {
		return ""
	}
	qualifiedName := op.Name
	qualifiedName.Package += "/" + name
	return qualifiedName.String()
}

// WITKind returns the WIT kind.
func (*Record) WITKind() string { return "record" }

// WIT returns the [WIT] text format for [Record] r.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (r *Record) WIT(ctx Node, name string) string {
	var b strings.Builder
	b.WriteString("record ")
	b.WriteString(escape(name))
	b.WriteString(" {")
	if len(r.Fields) > 0 {
		b.WriteRune('\n')
		for i := range r.Fields {
			if i > 0 {
				b.WriteString(",\n")
			}
			b.WriteString(indent(r.Fields[i].WIT(ctx, "")))
		}
		b.WriteRune('\n')
	}
	b.WriteRune('}')
	if ctx == nil {
		return b.String()
	}
	return unwrap(b.String())
}

// WITKind returns the WIT kind.
func (*Field) WITKind() string { return "field" }

// WIT returns the [WIT] text format for [Field] f.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (f *Field) WIT(ctx Node, name string) string {
	if ctx == nil {
		// Omit docs
		return escape(f.Name) + ": " + f.Type.WIT(f, "")
	}
	return f.Docs.WIT(ctx, "") + escape(f.Name) + ": " + f.Type.WIT(f, "")
}

// WITKind returns the WIT kind.
func (*Resource) WITKind() string { return "resource" }

// WIT returns the [WIT] text format for [Resource] r.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (r *Resource) WIT(ctx Node, name string) string {
	return "resource " + escape(name)
}

// WITKind returns the WIT kind.
func (*Own) WITKind() string { return "owned handle" }

// WIT returns the [WIT] text format for [Own] h.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (h *Own) WIT(ctx Node, name string) string {
	var b strings.Builder
	if name != "" {
		b.WriteString("type ")
		b.WriteString(escape(name))
		b.WriteString(" = ")
	}
	b.WriteString("own<")
	b.WriteString(h.Type.WIT(h, ""))
	b.WriteRune('>')
	return b.String()
}

// WITKind returns the WIT kind.
func (*Borrow) WITKind() string { return "borrowed handle" }

// WIT returns the [WIT] text format for [Borrow] h.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (h *Borrow) WIT(ctx Node, name string) string {
	var b strings.Builder
	if name != "" {
		b.WriteString("type ")
		b.WriteString(escape(name))
		b.WriteString(" = ")
	}
	b.WriteString("borrow<")
	b.WriteString(h.Type.WIT(h, ""))
	b.WriteRune('>')
	return b.String()
}

// WITKind returns the WIT kind.
func (*Flags) WITKind() string { return "flags" }

// WIT returns the [WIT] text format for [Flags] f.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (f *Flags) WIT(ctx Node, name string) string {
	var b strings.Builder
	b.WriteString("flags ")
	b.WriteString(escape(name))
	b.WriteString(" {")
	if len(f.Flags) > 0 {
		for i := range f.Flags {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(f.Flags[i].WIT(ctx, ""))
		}
	}
	b.WriteRune('}')
	if ctx == nil {
		return b.String()
	}
	return unwrap(b.String())
}

// WITKind returns the WIT kind.
func (*Flag) WITKind() string { return "flag" }

// WIT returns the [WIT] text format for [Flag] f.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (f *Flag) WIT(ctx Node, _ string) string {
	if ctx == nil {
		// Omit docs
		return escape(f.Name)
	}
	return f.Docs.WIT(ctx, "") + escape(f.Name)
}

// WITKind returns the WIT kind.
func (*Tuple) WITKind() string { return "tuple" }

// WIT returns the [WIT] text format for [Tuple] t.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (t *Tuple) WIT(ctx Node, name string) string {
	var b strings.Builder
	if name != "" {
		b.WriteString("type ")
		b.WriteString(escape(name))
		b.WriteString(" = ")
	}
	b.WriteString("tuple<")
	for i := range t.Types {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(t.Types[i].WIT(t, ""))
	}
	b.WriteString(">")
	return b.String()
}

// WITKind returns the WIT kind.
func (*Variant) WITKind() string { return "variant" }

// WIT returns the [WIT] text format for [Variant] v.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (v *Variant) WIT(ctx Node, name string) string {
	var b strings.Builder
	b.WriteString("variant ")
	b.WriteString(escape(name))
	b.WriteString(" {")
	if len(v.Cases) > 0 {
		b.WriteRune('\n')
		for i := range v.Cases {
			if i > 0 {
				b.WriteString(",\n")
			}
			b.WriteString(indent(v.Cases[i].WIT(ctx, "")))
		}
		b.WriteRune('\n')
	}
	b.WriteRune('}')
	if ctx == nil {
		return b.String()
	}
	return unwrap(b.String())
}

// WITKind returns the WIT kind.
func (*Case) WITKind() string { return "variant case" }

// WIT returns the [WIT] text format for [Variant] [Case] c.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (c *Case) WIT(ctx Node, _ string) string {
	var b strings.Builder
	if ctx != nil {
		b.WriteString(c.Docs.WIT(ctx, ""))
	}
	b.WriteString(escape(c.Name))
	if c.Type != nil {
		b.WriteRune('(')
		b.WriteString(c.Type.WIT(c, ""))
		b.WriteRune(')')
	}
	return b.String()
}

// WITKind returns the WIT kind.
func (*Enum) WITKind() string { return "enum" }

// WIT returns the [WIT] text format for [Enum] e.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (e *Enum) WIT(ctx Node, name string) string {
	var b strings.Builder
	b.WriteString("enum ")
	b.WriteString(escape(name))
	b.WriteString(" {")
	if len(e.Cases) > 0 {
		b.WriteRune('\n')
		for i := range e.Cases {
			if i > 0 {
				b.WriteString(",\n")
			}
			b.WriteString(indent(e.Cases[i].WIT(ctx, "")))
		}
		b.WriteRune('\n')
	}
	b.WriteRune('}')
	if ctx == nil {
		return b.String()
	}
	return unwrap(b.String())
}

// WITKind returns the WIT kind.
func (*EnumCase) WITKind() string { return "enum case" }

// WIT returns the [WIT] text format for [EnumCase] c.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (c *EnumCase) WIT(ctx Node, _ string) string {
	if ctx == nil {
		// Omit docs
		return escape(c.Name)
	}
	return c.Docs.WIT(ctx, "") + escape(c.Name)
}

// WITKind returns the WIT kind.
func (*Option) WITKind() string { return "option" }

// WIT returns the [WIT] text format for [Option] o.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (o *Option) WIT(_ Node, name string) string {
	var b strings.Builder
	if name != "" {
		b.WriteString("type ")
		b.WriteString(escape(name))
		b.WriteString(" = ")
	}
	b.WriteString("option<")
	b.WriteString(o.Type.WIT(o, ""))
	b.WriteRune('>')
	return b.String()
}

// WITKind returns the WIT kind.
func (*Result) WITKind() string { return "result" }

// WIT returns the [WIT] text format for [Result] r.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (r *Result) WIT(_ Node, name string) string {
	var b strings.Builder
	if name != "" {
		b.WriteString("type ")
		b.WriteString(escape(name))
		b.WriteString(" = ")
	}
	b.WriteString("result")
	if r.OK == nil && r.Err == nil {
		return b.String()
	}
	b.WriteRune('<')
	if r.OK != nil {
		b.WriteString(r.OK.WIT(r, ""))
	} else {
		b.WriteRune('_')
	}
	if r.Err != nil {
		b.WriteString(", ")
		b.WriteString(r.Err.WIT(r, ""))
	}
	b.WriteRune('>')
	return b.String()
}

// WITKind returns the WIT kind.
func (*List) WITKind() string { return "list" }

// WIT returns the [WIT] text format for [List] l.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (l *List) WIT(_ Node, name string) string {
	var b strings.Builder
	if name != "" {
		b.WriteString("type ")
		b.WriteString(escape(name))
		b.WriteString(" = ")
	}
	b.WriteString("list<")
	b.WriteString(l.Type.WIT(l, ""))
	b.WriteRune('>')
	return b.String()
}

// WITKind returns the WIT kind.
func (*Future) WITKind() string { return "future" }

// WIT returns the [WIT] text format for [Future] f.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (f *Future) WIT(_ Node, name string) string {
	var b strings.Builder
	if name != "" {
		b.WriteString("type ")
		b.WriteString(escape(name))
		b.WriteString(" = ")
	}
	b.WriteString("future")
	if f.Type != nil {
		b.WriteRune('<')
		b.WriteString(f.Type.WIT(f, ""))
		b.WriteRune('>')
	}
	return b.String()
}

// WITKind returns the WIT kind.
func (*Stream) WITKind() string { return "stream" }

// WIT returns the [WIT] text format for [Stream] s.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (s *Stream) WIT(_ Node, name string) string {
	var b strings.Builder
	if name != "" {
		b.WriteString("type ")
		b.WriteString(escape(name))
		b.WriteString(" = ")
	}
	b.WriteString("stream")
	if s.Element == nil && s.End == nil {
		return b.String()
	}
	b.WriteRune('<')
	if s.Element != nil {
		b.WriteString(s.Element.WIT(s, ""))
	} else {
		b.WriteRune('_')
	}
	if s.End != nil {
		b.WriteString(", ")
		b.WriteString(s.End.WIT(s, ""))
	}
	b.WriteRune('>')
	return b.String()
}

// WITKind returns the WIT kind.
func (_primitive[T]) WITKind() string { return "type" }

// WIT returns the [WIT] text format for this [_primitive].
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (p _primitive[T]) WIT(_ Node, name string) string {
	if name != "" {
		return "type " + name + " = " + p.String()
	}
	return p.String()
}

// WITKind returns the WIT kind.
func (*Function) WITKind() string { return "function" }

// WIT returns the [WIT] text format for [Function] f.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (f *Function) WIT(ctx Node, name string) string {
	if name == "" {
		name = f.Name
		if _, after, found := strings.Cut(name, "."); found {
			name = after
		}
	}
	var b strings.Builder
	if ctx != nil {
		b.WriteString(f.Docs.WIT(ctx, ""))
	}
	b.WriteString(escape(name))
	var isConstructor, isMethod bool
	switch f.Kind.(type) {
	case *Constructor:
		b.WriteRune('(')
		isConstructor = true
	case *Freestanding, *Method:
		b.WriteString(": func(")
		isMethod = true
	case *Static:
		b.WriteString(": static func(")
	}
	b.WriteString(paramsWIT(f.Params, isMethod))
	b.WriteRune(')')
	if !isConstructor && len(f.Results) > 0 {
		parens := len(f.Results) > 1 || f.Results[0].Name != ""
		b.WriteString(" -> ")
		if parens {
			b.WriteRune('(')
		}
		b.WriteString(paramsWIT(f.Results, false))
		if parens {
			b.WriteRune(')')
		}
	}
	b.WriteRune(';')
	return b.String()
}

func paramsWIT(params []Param, isMethod bool) string {
	var b strings.Builder
	var i int
	for _, param := range params {
		if param.Name == "self" && isMethod {
			continue
		}
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(param.WIT(nil, ""))
		i++
	}
	return b.String()
}

// WITKind returns the WIT kind.
func (*Param) WITKind() string { return "param" }

// WIT returns the [WIT] text format of [Param] p.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (p *Param) WIT(_ Node, _ string) string {
	if p.Name == "" {
		return p.Type.WIT(p, "")
	}
	return p.Name + ": " + p.Type.WIT(p, "")
}

// WITKind returns the WIT kind.
func (*Package) WITKind() string { return "package" }

// WIT returns the [WIT] text format of [Package] p.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (p *Package) WIT(ctx Node, _ string) string {
	var b strings.Builder
	b.WriteString(p.Docs.WIT(ctx, ""))
	b.WriteString("package ")
	b.WriteString(p.Name.String())
	b.WriteString(";\n")
	if len(p.Interfaces) > 0 {
		b.WriteRune('\n')
		for i, name := range codec.SortedKeys(p.Interfaces) {
			if i > 0 {
				b.WriteRune('\n')
			}
			b.WriteString(p.Interfaces[name].WIT(p, name))
			b.WriteRune('\n')
		}
	}
	if len(p.Worlds) > 0 {
		b.WriteRune('\n')
		for i, name := range codec.SortedKeys(p.Worlds) {
			if i > 0 {
				b.WriteRune('\n')
			}
			b.WriteString(p.Worlds[name].WIT(p, name))
			b.WriteRune('\n')
		}
	}
	return b.String()
}
