package wit

import (
	"io"

	"github.com/ydnar/wasm-tools-go/internal/codec"
	"github.com/ydnar/wasm-tools-go/internal/codec/json"
)

// DecodeJSON decodes JSON from r into a [Resolve] struct.
// It returns any error that may occur during decoding.
func DecodeJSON(r io.Reader) (*Resolve, error) {
	res := &Resolve{}
	dec := json.NewDecoder(r, res)
	err := dec.Decode(res)
	return res, err
}

// ResolveCodec implements the [codec.Resolver] interface
// translating types to decoding/encoding-aware versions.
func (res *Resolve) ResolveCodec(v any) codec.Codec {
	switch v := v.(type) {
	// References
	case **World:
		return &worldCodec{v, res}
	case **Interface:
		return &interfaceCodec{v, res}
	case **TypeDef:
		return &typeDefCodec{v, res}
	case **Package:
		return &packageCodec{v, res}

	// Allocation required
	case **Function:
		return codec.Must(v)

	// Enums
	case *FunctionKind:
		return &functionKindCodec{v}
	case *Handle:
		return &handleCodec{v}
	case *Type:
		return &typeCodec{v, res}
	case *TypeDefKind:
		return &typeDefKindCodec{v}
	case *TypeOwner:
		return &typeOwnerCodec{v}
	case *WorldItem:
		return &worldItemCodec{v}
	}

	return nil
}

func (c *Resolve) getWorld(i int) *World {
	return mustElement(&c.Worlds, i)
}

func (c *Resolve) getInterface(i int) *Interface {
	return mustElement(&c.Interfaces, i)
}

func (c *Resolve) getTypeDef(i int) *TypeDef {
	return mustElement(&c.TypeDefs, i)
}

func (c *Resolve) getPackage(i int) *Package {
	return mustElement(&c.Packages, i)
}

// DecodeField implements the [codec.FieldDecoder] interface
// to decode a struct or JSON object.
func (c *Resolve) DecodeField(dec codec.Decoder, name string) error {
	switch name {
	case "worlds":
		return codec.DecodeSlice(dec, &c.Worlds)
	case "interfaces":
		return codec.DecodeSlice(dec, &c.Interfaces)
	case "types":
		return codec.DecodeSlice(dec, &c.TypeDefs)
	case "packages":
		return codec.DecodeSlice(dec, &c.Packages)
	}
	return nil
}

// worldCodec translates WIT World references or structures into a *World.
type worldCodec struct {
	w **World
	*Resolve
}

func (c *worldCodec) DecodeInt(i int) error {
	*c.w = c.getWorld(i)
	return nil
}

func (c *worldCodec) DecodeField(dec codec.Decoder, name string) error {
	w := codec.Must(c.w)
	switch name {
	case "name":
		return dec.Decode(&w.Name)
	case "imports":
		return codec.DecodeMap(dec, &w.Imports)
	case "exports":
		return codec.DecodeMap(dec, &w.Exports)
	case "docs":
		return dec.Decode(&w.Docs)
	}
	return nil
}

// interfaceCodec translates WIT Interface references or structures into an *Interface.
type interfaceCodec struct {
	i **Interface
	*Resolve
}

func (c *interfaceCodec) DecodeInt(i int) error {
	*c.i = c.getInterface(i)
	return nil
}

func (c *interfaceCodec) DecodeField(dec codec.Decoder, name string) error {
	i := codec.Must(c.i)
	switch name {
	case "name":
		return dec.Decode(&i.Name)
	case "types":
		return codec.DecodeMap(dec, &i.TypeDefs)
	case "functions":
		return codec.DecodeMap(dec, &i.Functions)
	case "package":
		return dec.Decode(&i.Package)
	case "docs":
		return dec.Decode(&i.Docs)
	}
	return nil
}

// typeDefCodec translates WIT TypeDef references or structures into a *TypeDef.
type typeDefCodec struct {
	t **TypeDef
	*Resolve
}

func (c *typeDefCodec) DecodeInt(i int) error {
	*c.t = c.getTypeDef(i)
	return nil
}

func (c *typeDefCodec) DecodeField(dec codec.Decoder, name string) error {
	t := codec.Must(c.t)
	switch name {
	case "kind":
		return dec.Decode(&t.Kind)
	case "name":
		return dec.Decode(&t.Name)
	case "owner":
		return dec.Decode(&t.Owner)
	}
	return nil
}

// packageCodec translates WIT Package references or structures into a *Package.
type packageCodec struct {
	p **Package
	*Resolve
}

func (c *packageCodec) DecodeInt(i int) error {
	*c.p = c.getPackage(i)
	return nil
}

func (c *packageCodec) DecodeField(dec codec.Decoder, name string) error {
	p := codec.Must(c.p)
	switch name {
	case "name":
		return dec.Decode(&p.Name)
	case "interfaces":
		return codec.DecodeMap(dec, &p.Interfaces)
	case "worlds":
		return codec.DecodeMap(dec, &p.Worlds)
	case "docs":
		return dec.Decode(&p.Docs)
	}
	return nil
}

// DecodeString implements the [codec.StringDecoder] interface
// to decode a string value into a valid [PackageName].
func (pn *PackageName) DecodeString(s string) error {
	var err error
	*pn, err = ParsePackageName(s)
	return err
}

// worldItemCodec translates typed WorldItem references into a WorldItem,
// currently either an Interface or a TypeDef.
type worldItemCodec struct {
	v *WorldItem
}

func (c *worldItemCodec) DecodeField(dec codec.Decoder, name string) error {
	var err error
	switch name {
	case "interface":
		var v *Interface
		*c.v, err = v, dec.Decode(&v)
	case "function":
		var v *Function
		*c.v, err = v, dec.Decode(&v)
	case "type":
		var v *TypeDef
		*c.v, err = v, dec.Decode(&v)
	}
	return err
}

// typeCodec translates WIT type strings or reference IDs into a Type.
type typeCodec struct {
	t *Type
	*Resolve
}

// DecodeString translates s into to a primitive WIT type.
func (c *typeCodec) DecodeString(s string) error {
	var err error
	*c.t, err = ParseType(s)
	return err
}

func (c *typeCodec) DecodeInt(i int) error {
	*c.t = c.getTypeDef(i)
	return nil
}

// typeOwnerCodec translates WIT type owner enums into a [TypeOwner].
type typeOwnerCodec struct {
	v *TypeOwner
}

func (c *typeOwnerCodec) DecodeField(dec codec.Decoder, name string) error {
	var err error
	switch name {
	case "interface":
		var v *Interface
		*c.v, err = v, dec.Decode(&v)
	case "world":
		var v *World
		*c.v, err = v, dec.Decode(&v)
	}
	return err
}

// typeDefKindCodec translates WIT type owner enums into a [TypeDefKind].
type typeDefKindCodec struct {
	v *TypeDefKind
}

func (c *typeDefKindCodec) DecodeString(s string) error {
	switch s {
	case "resource":
		*c.v = &Resource{}
	}
	return nil
}

func (c *typeDefKindCodec) DecodeField(dec codec.Decoder, name string) error {
	var err error
	switch name {
	case "record":
		v := &Record{}
		*c.v, err = v, dec.Decode(v)
	case "resource": // TODO: this might not be necessary
		v := &Resource{}
		*c.v, err = v, dec.Decode(v)
	case "handle":
		var v Handle
		*c.v, err = v, dec.Decode(&v)
	case "flags":
		v := &Flags{}
		*c.v, err = v, dec.Decode(v)
	case "tuple":
		v := &Tuple{}
		*c.v, err = v, dec.Decode(v)
	case "variant":
		v := &Variant{}
		*c.v, err = v, dec.Decode(v)
	case "enum":
		v := &Enum{}
		*c.v, err = v, dec.Decode(v)
	case "option":
		v := &Option{}
		*c.v, err = v, dec.Decode(&v.Type)
	case "result":
		v := &Result{}
		*c.v, err = v, dec.Decode(v)
	case "list":
		v := &List{}
		*c.v, err = v, dec.Decode(&v.Type)
	case "future":
		v := &Future{}
		*c.v, err = v, dec.Decode(&v.Type)
	case "stream":
		v := &Stream{}
		*c.v, err = v, dec.Decode(v)
	case "type":
		var v Type
		*c.v, err = v, dec.Decode(&v)
	}
	return err
}

// DecodeField implements the [codec.FieldDecoder] interface
// to decode a struct or JSON object.
func (r *Record) DecodeField(dec codec.Decoder, name string) error {
	switch name {
	case "fields":
		return codec.DecodeSlice(dec, &r.Fields)
	}
	return nil
}

// DecodeField implements the [codec.FieldDecoder] interface
// to decode a struct or JSON object.
func (f *Field) DecodeField(dec codec.Decoder, name string) error {
	switch name {
	case "name":
		return dec.Decode(&f.Name)
	case "type":
		return dec.Decode(&f.Type)
	case "docs":
		return dec.Decode(&f.Docs)
	}
	return nil
}

// DecodeField implements the [codec.FieldDecoder] interface
// to decode a struct or JSON object.
func (f *Flags) DecodeField(dec codec.Decoder, name string) error {
	switch name {
	case "flags":
		return codec.DecodeSlice(dec, &f.Flags)
	}
	return nil
}

// DecodeField implements the [codec.FieldDecoder] interface
// to decode a struct or JSON object.
func (f *Flag) DecodeField(dec codec.Decoder, name string) error {
	switch name {
	case "name":
		return dec.Decode(&f.Name)
	case "docs":
		return dec.Decode(&f.Docs)
	}
	return nil
}

// DecodeField implements the [codec.FieldDecoder] interface
// to decode a struct or JSON object.
func (t *Tuple) DecodeField(dec codec.Decoder, name string) error {
	switch name {
	case "types":
		return codec.DecodeSlice(dec, &t.Types)
	}
	return nil
}

// DecodeField implements the [codec.FieldDecoder] interface
// to decode a struct or JSON object.
func (v *Variant) DecodeField(dec codec.Decoder, name string) error {
	switch name {
	case "cases":
		return codec.DecodeSlice(dec, &v.Cases)
	}
	return nil
}

// DecodeField implements the [codec.FieldDecoder] interface
// to decode a struct or JSON object.
func (c *Case) DecodeField(dec codec.Decoder, name string) error {
	switch name {
	case "name":
		return dec.Decode(&c.Name)
	case "type":
		return dec.Decode(&c.Type)
	case "docs":
		return dec.Decode(&c.Docs)
	}
	return nil
}

type handleCodec struct {
	v *Handle
}

func (c *handleCodec) DecodeField(dec codec.Decoder, name string) error {
	var err error
	switch name {
	case "own":
		v := &OwnedHandle{}
		*c.v, err = v, dec.Decode(&v.Type)
	case "borrow":
		v := &BorrowedHandle{}
		*c.v, err = v, dec.Decode(&v.Type)
	}
	return err
}

// DecodeField implements the [codec.FieldDecoder] interface
// to decode a struct or JSON object.
func (e *Enum) DecodeField(dec codec.Decoder, name string) error {
	switch name {
	case "cases":
		return codec.DecodeSlice(dec, &e.Cases)
	}
	return nil
}

// DecodeField implements the [codec.FieldDecoder] interface
// to decode a struct or JSON object.
func (r *Result) DecodeField(dec codec.Decoder, name string) error {
	switch name {
	case "ok":
		return dec.Decode(&r.OK)
	case "err":
		return dec.Decode(&r.Err)
	}
	return nil
}

// DecodeField implements the [codec.FieldDecoder] interface
// to decode a struct or JSON object.
func (s *Stream) DecodeField(dec codec.Decoder, name string) error {
	switch name {
	case "element":
		return dec.Decode(&s.Element)
	case "end":
		return dec.Decode(&s.End)
	}
	return nil
}

// DecodeField implements the [codec.FieldDecoder] interface
// to decode a struct or JSON object.
func (c *EnumCase) DecodeField(dec codec.Decoder, name string) error {
	switch name {
	case "name":
		return dec.Decode(&c.Name)
	case "docs":
		return dec.Decode(&c.Docs)
	}
	return nil
}

// DecodeField implements the [codec.FieldDecoder] interface
// to decode a struct or JSON object.
func (f *Function) DecodeField(dec codec.Decoder, name string) error {
	switch name {
	case "name":
		return dec.Decode(&f.Name)
	case "kind":
		return dec.Decode(&f.Kind)
	case "params":
		return codec.DecodeSlice(dec, &f.Params)
	case "results":
		return codec.DecodeSlice(dec, &f.Results)
	case "docs":
		return dec.Decode(&f.Docs)
	}
	return nil
}

type functionKindCodec struct {
	v *FunctionKind
}

func (c *functionKindCodec) DecodeString(s string) error {
	switch s {
	case "freestanding":
		*c.v = &Freestanding{}
	}
	return nil
}

func (c *functionKindCodec) DecodeField(dec codec.Decoder, name string) error {
	var err error
	switch name {
	case "method":
		v := &Method{}
		*c.v, err = v, dec.Decode(&v.Type)
	case "static":
		v := &Static{}
		*c.v, err = v, dec.Decode(&v.Type)
	case "constructor":
		v := &Constructor{}
		*c.v, err = v, dec.Decode(&v.Type)
	}
	return err
}

// DecodeField implements the [codec.FieldDecoder] interface
// to decode a struct or JSON object.
func (p *Param) DecodeField(dec codec.Decoder, name string) error {
	switch name {
	case "name":
		return dec.Decode(&p.Name)
	case "type":
		return dec.Decode(&p.Type)
	}
	return nil
}

// mustElement resizes s and allocates a new instance of T if necessary.
func mustElement[S ~[]*E, E any](s *S, i int) *E {
	if i < 0 {
		return nil
	}
	if codec.Resize(s, i) == nil {
		(*s)[i] = new(E)
	}
	return (*s)[i]
}
