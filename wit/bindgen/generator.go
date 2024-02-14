// Package bindgen generates Go source code from a fully-resolved WIT package.
// It generates one or more Go packages, with functions, types, constants, and variables,
// along with the associated code to lift and lower Go types into Canonical ABI representation.
package bindgen

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ydnar/wasm-tools-go/internal/codec"
	"github.com/ydnar/wasm-tools-go/internal/go/gen"
	"github.com/ydnar/wasm-tools-go/wit"
)

const (
	GoSuffix  = ".wit.go"
	cmPackage = "github.com/ydnar/wasm-tools-go/cm"
	emptyAsm  = `// This file exists for testing this package without WebAssembly,
// allowing empty function bodies with a //go:wasmimport directive.
// See https://pkg.go.dev/cmd/compile for more information.
`
)

// Go generates one or more Go packages from [wit.Resolve] res.
// It returns any error that occurs during code generation.
func Go(res *wit.Resolve, opts ...Option) ([]*gen.Package, error) {
	g, err := newGenerator(res, opts...)
	if err != nil {
		return nil, err
	}
	return g.generate()
}

type generator struct {
	opts options
	res  *wit.Resolve

	// versioned is set to true if there are multiple versions of a WIT package in res,
	// which affects the generated Go package paths.
	versioned bool

	// packages are Go packages indexed on Go package paths.
	packages map[string]*gen.Package

	// witPackages map WIT identifier paths to Go packages.
	witPackages map[string]*gen.Package

	// worldPackages map [wit.World] to Go packages.
	worldPackages map[*wit.World]*gen.Package

	// interfacePackages map [wit.Interface] to Go packages.
	interfacePackages map[*wit.Interface]*gen.Package

	// typeDefs map [wit.TypeDef] to their equivalent Go identifier.
	typeDefs map[*wit.TypeDef]gen.Ident

	// funcs map [wit.Function] to their equivalent Go identifier.
	funcs map[*wit.Function]gen.Ident

	// defined represent whether a type or function has been defined.
	defined map[gen.Ident]bool
}

func newGenerator(res *wit.Resolve, opts ...Option) (*generator, error) {
	g := &generator{
		packages:          make(map[string]*gen.Package),
		witPackages:       make(map[string]*gen.Package),
		worldPackages:     make(map[*wit.World]*gen.Package),
		interfacePackages: make(map[*wit.Interface]*gen.Package),
		typeDefs:          make(map[*wit.TypeDef]gen.Ident),
		funcs:             make(map[*wit.Function]gen.Ident),
		defined:           make(map[gen.Ident]bool),
	}
	err := g.opts.apply(opts...)
	if err != nil {
		return nil, err
	}
	if g.opts.generatedBy == "" {
		_, file, _, _ := runtime.Caller(0)
		_, g.opts.generatedBy = filepath.Split(filepath.Dir(filepath.Dir(file)))
	}
	if g.opts.packageName == "" {
		g.opts.packageName = res.Packages[0].Name.Namespace
	}
	if g.opts.cmPackage == "" {
		g.opts.cmPackage = cmPackage
	}
	g.res = res
	return g, nil
}

func (g *generator) generate() ([]*gen.Package, error) {
	g.detectVersionedPackages()

	err := g.declareTypeDefs()
	if err != nil {
		return nil, err
	}

	// err := g.defineInterfaces()
	// if err != nil {
	// 	return nil, err
	// }

	err = g.defineWorlds()
	if err != nil {
		return nil, err
	}

	var packages []*gen.Package
	for _, path := range codec.SortedKeys(g.packages) {
		packages = append(packages, g.packages[path])
	}
	return packages, nil
}

func (g *generator) detectVersionedPackages() {
	if g.opts.versioned {
		g.versioned = true
		fmt.Fprintf(os.Stderr, "Generated versions for all package(s)\n")
		return
	}
	packages := make(map[string]string)
	for _, pkg := range g.res.Packages {
		id := pkg.Name
		id.Version = nil
		path := id.String()
		if packages[path] != "" && packages[path] != pkg.Name.String() {
			g.versioned = true
		} else {
			packages[path] = pkg.Name.String()
		}
	}
	if g.versioned {
		fmt.Fprintf(os.Stderr, "Multiple versions of package(s) detected\n")
	}
}

// declareTypeDefs declares all type definitions in res.
func (g *generator) declareTypeDefs() error {
	for _, t := range g.res.TypeDefs {
		err := g.declareTypeDef(t)
		if err != nil {
			return err
		}
	}
	return nil
}

func (g *generator) declareTypeDef(t *wit.TypeDef) error {
	if t.Name == nil {
		return nil
	}
	name := *t.Name

	ownerID := typeDefOwnerID(t)
	pkg := g.packageFor(ownerID)
	file := pkg.File(ownerID.Extension + GoSuffix)
	file.GeneratedBy = g.opts.generatedBy

	id := file.Declare(GoName(name, true))
	g.typeDefs[t] = id

	// fmt.Fprintf(os.Stderr, "Type:\t%s.%s\n\t%s.%s\n", ownerID.String(), name, decl.Package.Path, decl.Name)

	return nil
}

func typeDefOwnerID(t *wit.TypeDef) wit.Ident {
	var id wit.Ident
	switch owner := t.Owner.(type) {
	case *wit.World:
		id = owner.Package.Name
		id.Extension = owner.Name
	case *wit.Interface:
		id = owner.Package.Name
		if owner.Name == nil {
			id.Extension = "unknown"
		} else {
			id.Extension = *owner.Name
		}
	}
	return id
}

func (g *generator) defineInterfaces() error {
	var interfaces []*wit.Interface
	for _, i := range g.res.Interfaces {
		if i.Name != nil {
			interfaces = append(interfaces, i)
		}
	}
	fmt.Fprintf(os.Stderr, "Generating Go for %d named interface(s)\n", len(interfaces))
	for _, i := range interfaces {
		g.defineInterface(i, *i.Name)
	}
	return nil
}

// By default, each WIT interface and world maps to a single Go package.
// Options might override the Go package, including combining multiple
// WIT interfaces and/or worlds into a single Go package.
func (g *generator) defineWorlds() error {
	fmt.Fprintf(os.Stderr, "Generating Go for %d world(s)\n", len(g.res.Worlds))
	for _, w := range g.res.Worlds {
		g.defineWorld(w)
	}
	return nil
}

func (g *generator) defineWorld(w *wit.World) error {
	if g.worldPackages[w] != nil {
		return nil
	}
	id := w.Package.Name
	id.Extension = w.Name
	pkg := g.packageFor(id)
	g.worldPackages[w] = pkg

	file := pkg.File(id.Extension + GoSuffix)
	file.GeneratedBy = g.opts.generatedBy

	var b strings.Builder
	fmt.Fprintf(&b, "Package %s represents the %s \"%s\".\n", pkg.Name, w.WITKind(), id.String())
	if w.Docs.Contents != "" {
		b.WriteString("\n")
		b.WriteString(w.Docs.Contents)
	}
	file.PackageDocs = b.String()

	// fmt.Printf("// World: %s\n\n", id.String())
	for _, name := range codec.SortedKeys(w.Imports) {
		var err error
		switch v := w.Imports[name].(type) {
		case *wit.Interface:
			err = g.defineInterface(v, name)
		case *wit.TypeDef:
			err = g.defineTypeDef(v, name)
		case *wit.Function:
			err = g.defineImportedFunction(v, id)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (g *generator) defineInterface(i *wit.Interface, name string) error {
	if g.interfacePackages[i] != nil {
		return nil
	}
	if i.Name != nil {
		name = *i.Name
	}
	id := i.Package.Name
	id.Extension = name
	pkg := g.packageFor(id)
	g.interfacePackages[i] = pkg

	file := pkg.File(id.Extension + GoSuffix)
	file.GeneratedBy = g.opts.generatedBy

	var b strings.Builder
	fmt.Fprintf(&b, "Package %s represents the %s \"%s\".\n", pkg.Name, i.WITKind(), id.String())
	if i.Docs.Contents != "" {
		b.WriteString("\n")
		b.WriteString(i.Docs.Contents)
	}
	file.PackageDocs = b.String()

	for _, name := range codec.SortedKeys(i.TypeDefs) {
		g.defineTypeDef(i.TypeDefs[name], name)
	}

	for _, name := range codec.SortedKeys(i.Functions) {
		g.defineImportedFunction(i.Functions[name], id)
	}

	return nil
}

func (g *generator) defineTypeDef(t *wit.TypeDef, name string) error {
	if t.Name != nil {
		name = *t.Name
	}

	id := g.typeDefs[t]
	if id == (gen.Ident{}) {
		return fmt.Errorf("typedef %s not declared", name)
	}
	if g.defined[id] {
		return nil
	}
	// TODO: should we emit data for aliases?
	if t.Root() != t {
		return nil
	}

	ownerID := typeDefOwnerID(t)
	file := id.Package.File(ownerID.Extension + GoSuffix)
	file.GeneratedBy = g.opts.generatedBy

	fmt.Fprintf(file, "// %s represents the %s \"%s#%s\".\n", id.Name, t.WITKind(), ownerID.String(), name)
	fmt.Fprintf(file, "//\n")
	fmt.Fprint(file, gen.FormatDocComments(t.WIT(nil, ""), true))
	fmt.Fprintf(file, "//\n")
	if t.Docs.Contents != "" {
		fmt.Fprintf(file, "//\n%s", gen.FormatDocComments(t.Docs.Contents, false))
	}
	fmt.Fprintf(file, "type %s ", id.Name)
	fmt.Fprint(file, g.typeDefRep(file, id, t))
	fmt.Fprint(file, "\n\n")

	return nil
}

func (g *generator) typeDefRep(file *gen.File, typeName gen.Ident, t *wit.TypeDef) string {
	switch kind := t.Kind.(type) {
	case wit.Type:
		return g.typeRep(file, kind)
	case *wit.Record:
		return g.recordRep(file, kind)
	case *wit.Resource:
		return g.resourceRep(file, kind)
	case *wit.OwnedHandle:
		return "any /* TODO: *wit.OwnedHandle */"
	case *wit.BorrowedHandle:
		return "any /* TODO: *wit.BorrowedHandle */"
	case *wit.Flags:
		return "any /* TODO: *wit.Flags */"
	case *wit.Enum:
		return g.enumRep(file, typeName, kind)
	case *wit.Tuple:
		return "any /* TODO: *wit.Tuple */"
	case *wit.Variant:
		return "any /* TODO: *wit.Variant */"
	case *wit.Option:
		return g.optionRep(file, kind)
	case *wit.Result:
		return "any /* TODO: *wit.Result */"
	case *wit.List:
		return "any /* TODO: *wit.List */"
	case *wit.Future:
		return "any /* TODO: *wit.Future */"
	case *wit.Stream:
		return "any /* TODO: *wit.Stream */"
	default:
		panic(fmt.Sprintf("BUG: unknown wit.TypeDef %T", t)) // should never reach here
	}
}

func (g *generator) typeRep(file *gen.File, t wit.Type) string {
	switch t := t.(type) {
	case *wit.TypeDef:
		if id, ok := g.typeDefs[t]; ok {
			return file.Ident(id)
		}
		// FIXME: this is only valid for built-in WIT types.
		// User-defined types must be named, so the Ident check above must have succeeded.
		// See https://component-model.bytecodealliance.org/design/wit.html#built-in-types
		// and https://component-model.bytecodealliance.org/design/wit.html#user-defined-types.
		// TODO: add wit.Type.BuiltIn() method?
		return g.typeDefRep(file, gen.Ident{}, t)
	case wit.Primitive:
		return g.primitiveRep(file, t)
	default:
		panic(fmt.Sprintf("BUG: unknown wit.Type %T", t)) // should never reach here
	}
}

func (g *generator) primitiveRep(file *gen.File, p wit.Primitive) string {
	switch p := p.(type) {
	case wit.Bool:
		return "bool"
	case wit.S8:
		return "sint8"
	case wit.U8:
		return "uint8"
	case wit.S16:
		return "sint16"
	case wit.U16:
		return "uint16"
	case wit.S32:
		return "sint32"
	case wit.U32:
		return "uint32"
	case wit.S64:
		return "sint64"
	case wit.U64:
		return "uint64"
	case wit.Float32:
		return "float32"
	case wit.Float64:
		return "float64"
	case wit.Char:
		return "rune"
	case wit.String:
		return "string"
	default:
		panic(fmt.Sprintf("BUG: unknown wit.Primitive %T", p)) // should never reach here
	}
}

func (g *generator) recordRep(file *gen.File, r *wit.Record) string {
	var b strings.Builder
	b.WriteString("struct {")
	b.WriteString("TODO any // TODO: record fields\n")
	b.WriteString("}")
	return b.String()
}

func (g *generator) resourceRep(file *gen.File, r *wit.Resource) string {
	var b strings.Builder
	b.WriteString(file.Import(g.opts.cmPackage))
	b.WriteString(".Resource")
	b.WriteString("\n\n// TODO: resource methods")
	return b.String()
}

func (g *generator) enumRep(file *gen.File, typeName gen.Ident, e *wit.Enum) string {
	var b strings.Builder
	disc := wit.Discriminant(len(e.Cases))
	b.WriteString(g.typeRep(file, disc))
	b.WriteString("\n\n")
	b.WriteString("const (\n")
	for i, c := range e.Cases {
		b.WriteString(gen.FormatDocComments(c.Docs.Contents, false))
		caseName := file.Declare(typeName.Name + GoName(c.Name, true))
		b.WriteString(caseName.Name)
		if i == 0 {
			b.WriteRune(' ')
			b.WriteString(typeName.Name)
			b.WriteString(" = iota")
		}
		b.WriteString("\n\n")
	}
	b.WriteString(")\n")
	return b.String()
}

func (g *generator) optionRep(file *gen.File, r *wit.Option) string {
	var b strings.Builder
	b.WriteString(file.Import(g.opts.cmPackage))
	b.WriteString(".Option[any /* TODO */]")
	return b.String()
}

func (g *generator) defineImportedFunction(f *wit.Function, ownerID wit.Ident) error {
	if !f.IsFreestanding() {
		return nil
	}
	if _, ok := g.funcs[f]; ok {
		return nil
	}

	pkg := g.packageFor(ownerID)
	file := pkg.File(ownerID.Extension + GoSuffix)
	file.GeneratedBy = g.opts.generatedBy

	funcDecl := file.Declare(GoName(f.Name, true))
	g.funcs[f] = funcDecl
	snakeDecl := file.Declare(SnakeName(f.Name))

	fmt.Fprintf(file, "// %s represents the %s \"%s#%s\".\n", funcDecl.Name, f.WITKind(), ownerID.String(), f.Name)
	fmt.Fprintf(file, "//\n")
	fmt.Fprint(file, gen.FormatDocComments(f.WIT(nil, f.Name), true))
	fmt.Fprintf(file, "//\n")
	if f.Docs.Contents != "" {
		fmt.Fprintf(file, "//\n%s", gen.FormatDocComments(f.Docs.Contents, false))
	}
	fmt.Fprintf(file, "func %s()\n\n", funcDecl.Name)
	fmt.Fprintf(file, "//go:wasmimport %s %s\n", ownerID.String(), f.Name)
	fmt.Fprintf(file, "func %s()\n\n", snakeDecl.Name)

	return g.ensureEmptyAsm(pkg)
}

func (g *generator) ensureEmptyAsm(pkg *gen.Package) error {
	f := pkg.File("empty.s")
	if len(f.Content) > 0 {
		return nil
	}
	_, err := f.Write([]byte(emptyAsm))
	return err
}

func (g *generator) packageFor(id wit.Ident) *gen.Package {
	// Find existing
	pkg := g.witPackages[id.String()]
	if pkg != nil {
		return pkg
	}

	// Create a new package
	path := id.Namespace + "/" + id.Package + "/" + id.Extension
	if g.opts.packageRoot != "" && g.opts.packageRoot != "std" {
		path = g.opts.packageRoot + "/" + path
	}
	name := id.Extension
	if g.versioned && id.Version != nil {
		path += "-" + id.Version.String()
	}

	name = GoPackageName(name)
	// Ensure local name doesn’t conflict with Go keywords or predeclared identifiers
	if gen.Unique(name, gen.IsReserved) != name {
		// Try with package prefix, like error -> ioerror
		name = id.Package + name
		if gen.Unique(name, gen.IsReserved) != name {
			// Try with namespace prefix, like ioerror -> wasiioerror
			name = gen.Unique(id.Namespace+name, gen.IsReserved)
		}
	}

	pkg = gen.NewPackage(path + "#" + name)
	g.packages[pkg.Path] = pkg
	g.witPackages[id.String()] = pkg

	return pkg
}
