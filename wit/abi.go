package wit

// Align aligns ptr with alignment align.
func Align(ptr, align uintptr) uintptr {
	return (ptr + align - 1) &^ (align - 1)
}

// Discriminant returns the smallest WIT integer type that can represent 0...n.
// Used by the [Canonical ABI] for [Variant] types.
//
// [Canonical ABI]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/CanonicalABI.md#alignment
func Discriminant(n int) Type {
	switch {
	case n <= 1<<8:
		return U8{}
	case n <= 1<<16:
		return U16{}
	}
	return U32{}
}

// Sized is the interface implemented by any type that reports its [ABI byte size], [alignment],
// and whether the type contains a pointer.
//
// [ABI byte size]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/CanonicalABI.md#size
// [alignment]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/CanonicalABI.md#alignment
type Sized interface {
	Size() uintptr
	Align() uintptr
	HasPointer() bool
}

// Despecializer is the interface implemented by any [TypeDefKind] that can
// [despecialize] itself into another TypeDefKind. Examples include [Result],
// which despecializes into a [Variant] with two cases, "ok" and "error".
// See the [canonical ABI documentation] for more information.
//
// [despecialize]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/CanonicalABI.md#despecialization
// [canonical ABI documentation]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/CanonicalABI.md#despecialization
type Despecializer interface {
	Despecialize() TypeDefKind
}

// Despecialize [despecializes] k if k implements [Despecializer].
// Otherwise, it returns k unmodified.
// See the [canonical ABI documentation] for more information.
//
// [despecializes]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/CanonicalABI.md#despecialization
// [canonical ABI documentation]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/CanonicalABI.md#despecialization
func Despecialize(k TypeDefKind) TypeDefKind {
	if d, ok := k.(Despecializer); ok {
		return d.Despecialize()
	}
	return k
}
