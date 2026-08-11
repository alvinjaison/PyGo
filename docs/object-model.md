# PyGo — Python Object Model

## Table of Contents

1. [Core Interface](#1-core-interface)
2. [Type System Overview](#2-type-system-overview)
3. [Primitive Types](#3-primitive-types)
4. [Collection Types](#4-collection-types)
5. [Callable Types](#5-callable-types)
6. [Class and Instance Types](#6-class-and-instance-types)
7. [Iterator and Generator Types](#7-iterator-and-generator-types)
8. [Exception Types](#8-exception-types)
9. [Module Type](#9-module-type)
10. [Go Bridge Type](#10-go-bridge-type)
11. [Attribute and Method Lookup](#11-attribute-and-method-lookup)
12. [Operator Protocol](#12-operator-protocol)
13. [Hashing and Equality](#13-hashing-and-equality)
14. [Type Hierarchy](#14-type-hierarchy)
15. [Memory Model](#15-memory-model)
16. [Identity and Mutability Table](#16-identity-and-mutability-table)

---

## 1. Core Interface

Every Python value in PyGo is a Go value implementing the `Object` interface:

```go
// Object is the root interface for all Python values.
// Every value that can exist on the Python value stack must implement Object.
type Object interface {
    // Type returns the Python type object for this value.
    // Equivalent to type(obj) in Python.
    Type() *TypeObject

    // String returns a human-readable representation (used by str() and print()).
    String() string

    // Repr returns the developer representation (used by repr()).
    Repr() string
}
```

This is intentionally minimal. Additional protocols (callable, iterable, hashable, comparable) are expressed as optional Go interfaces that the VM checks at runtime via type assertions:

```go
// Callable objects can be invoked with arguments.
type Callable interface {
    Object
    Call(args []Object, kwargs map[string]Object) (Object, error)
}

// Iterable objects can produce an iterator.
type Iterable interface {
    Object
    Iter() (Iterator, error)
}

// Iterator objects support next().
type Iterator interface {
    Object
    Next() (Object, error) // returns (nil, StopIteration) when exhausted
}

// Hashable objects can be used as dict keys or set members.
type Hashable interface {
    Object
    Hash() (int64, error)
}

// Comparable supports rich comparison beyond ==.
type Comparable interface {
    Object
    RichCompare(other Object, op CompareOp) (Object, error)
}

// LenProvider supports len().
type LenProvider interface {
    Object
    Len() int
}

// Subscriptable supports obj[key].
type Subscriptable interface {
    Object
    GetItem(key Object) (Object, error)
    SetItem(key Object, value Object) error
    DelItem(key Object) error
}

// AttributeHolder supports obj.attr.
type AttributeHolder interface {
    Object
    GetAttr(name string) (Object, error)
    SetAttr(name string, value Object) error
    DelAttr(name string) error
}
```

**Why optional interfaces rather than a fat Object interface?**

A fat interface with 30+ methods would require every concrete type to implement a large set of no-op stubs. Optional interface detection via type assertions is idiomatic Go and matches how the Python data model works — not every object is callable, iterable, or subscriptable. The VM's dispatch overhead from type assertions is negligible compared to the cost of executing Python instructions.

---

## 2. Type System Overview

Python's type system is: *everything is an object, including types.*

In PyGo:

```go
// TypeObject represents a Python type (class).
// Both built-in types (int, str, list) and user-defined classes
// are represented as *TypeObject values.
type TypeObject struct {
    Name    string
    Bases   []*TypeObject  // for inheritance
    Dict    *DictObject    // type's attribute namespace
    // protocol slots — Go functions implementing the type's behaviors
    Slots   TypeSlots
}

// TypeSlots holds optional Go-level implementations of Python protocols.
// When a slot is nil, the default behavior (or an AttributeError) applies.
type TypeSlots struct {
    New        func(typ *TypeObject, args []Object, kwargs map[string]Object) (Object, error)
    Init       func(self Object, args []Object, kwargs map[string]Object) error
    Repr       func(self Object) (string, error)
    Str        func(self Object) (string, error)
    Hash       func(self Object) (int64, error)
    Call       func(self Object, args []Object, kwargs map[string]Object) (Object, error)
    GetAttr    func(self Object, name string) (Object, error)
    SetAttr    func(self Object, name string, value Object) error
    DelAttr    func(self Object, name string) error
    RichCmp    func(self, other Object, op CompareOp) (Object, error)
    Iter       func(self Object) (Object, error)
    Next       func(self Object) (Object, error)
    Len        func(self Object) (int, error)
    GetItem    func(self Object, key Object) (Object, error)
    SetItem    func(self Object, key Object, value Object) error
    DelItem    func(self Object, key Object) error
    Add        func(a, b Object) (Object, error)
    Sub        func(a, b Object) (Object, error)
    Mul        func(a, b Object) (Object, error)
    // ... other numeric slots
    Bool       func(self Object) (bool, error)
    Contains   func(self, item Object) (bool, error)
}
```

This slot-based design mirrors CPython's `PyTypeObject` and is the standard approach for high-performance Python runtimes. It allows the VM to call type operations without going through general attribute lookup for the common case.

The `TypeObject` itself implements `Object`, making types first-class Python values.

---

## 3. Primitive Types

### 3.1 Int

```go
type IntObject struct {
    Value *big.Int  // arbitrary precision
}
```

Python integers are arbitrary precision. Using `*big.Int` is correct from day one.

**Important:** For performance, small integers (e.g., -5 to 256) are cached as a fixed array of preallocated `IntObject` values. This matches CPython's small-integer cache and avoids heap allocation for the common case.

Implements: `Object`, `Hashable`, `Comparable`, `Numeric operators`

### 3.2 Float

```go
type FloatObject struct {
    Value float64
}
```

Python floats are IEEE 754 double precision. `float64` is correct and complete.

Implements: `Object`, `Hashable`, `Comparable`, `Numeric operators`

### 3.3 Str

```go
type StrObject struct {
    Value string  // Go string (UTF-8)
}
```

Python 3 strings are Unicode. Go strings are valid UTF-8 byte sequences. This mapping is correct for the common case. Operations on individual characters (`str[i]`) must yield Unicode code points, not bytes — this requires care when implementing `__getitem__`.

Implements: `Object`, `Hashable`, `Comparable`, `Iterable`, `Subscriptable` (read-only), `LenProvider`

String interning: frequently used short strings (identifiers, short literals) are interned for fast equality comparison.

### 3.4 Bool

```go
type BoolObject struct {
    Value bool
}

var (
    True  = &BoolObject{Value: true}
    False = &BoolObject{Value: false}
)
```

`bool` is a subtype of `int` in Python. In PyGo, `BoolObject` holds a `bool` but its `TypeObject` lists `IntObject`'s type as a base. Arithmetic on booleans (`True + True == 2`) must work correctly.

`True` and `False` are singletons — there is exactly one instance of each in a runtime.

### 3.5 None

```go
type NoneObject struct{}

var None = &NoneObject{}
```

`None` is a singleton. `is None` comparisons compare pointer identity.

### 3.6 Bytes

```go
type BytesObject struct {
    Value []byte
}
```

Immutable byte sequence. Initial priority: low (Phase 2+).

### 3.7 Bytearray

Mutable byte sequence. Deferred to Phase 3+.

---

## 4. Collection Types

### 4.1 List

```go
type ListObject struct {
    Items []Object
}
```

Mutable, ordered sequence. `Items` is a Go slice; append operations use `append()`. Implements `Iterable`, `Subscriptable` (read-write), `LenProvider`.

Iteration creates a `ListIterator` that holds an index into the list. Mutation during iteration is allowed (matching Python semantics) but produces CPython-consistent behavior.

### 4.2 Tuple

```go
type TupleObject struct {
    Items []Object  // fixed at creation, never modified
}
```

Immutable sequence. `Items` is allocated exactly once. Implements `Hashable` (if all elements are hashable), `Iterable`, `Subscriptable` (read-only), `LenProvider`.

Empty tuple `()` and single-element tuples are handled correctly (trailing comma in Python source).

### 4.3 Dict

```go
type DictObject struct {
    // Insertion-ordered map, matching Python 3.7+ dict behavior.
    // Using a linked hash map implementation.
    entries  []dictEntry
    indices  map[int64]int  // hash → entry index (for O(1) lookup)
    // alternatively: use a purpose-built ordered hash map
}

type dictEntry struct {
    key   Object
    value Object
    hash  int64
}
```

Insertion-ordered as of Python 3.7. `DictObject` must preserve insertion order in iteration.

The implementation choices: a flat array of entries plus a hash table mapping hash → entry index. This is a simplified version of CPython's compact dict implementation. A simpler initial implementation using Go's `map[Object]Object` is not viable because Go maps do not preserve insertion order and `Object` (an interface) is not directly usable as a map key — hashing and equality must be defined by the Python object, not Go's built-in comparison.

Correct implementation strategy for MVP: maintain an `[]dictEntry` slice for ordering and a `map[int64][]int` for hash-based lookup with collision chaining.

Implements `Iterable` (yields keys), `Subscriptable`, `LenProvider`.

### 4.4 Set / Frozenset

```go
type SetObject struct {
    items  map[int64][]Object  // hash → []Object (collision bucket)
    size   int
}

type FrozenSetObject struct {
    items  map[int64][]Object
    size   int
}
```

`FrozenSetObject` is immutable and `Hashable`. `SetObject` is mutable and not hashable.

Implements `Iterable`, `LenProvider`, set operations (`|`, `&`, `-`, `^`).

---

## 5. Callable Types

### 5.1 Function (Python-defined)

```go
type FunctionObject struct {
    Code     *bytecode.Code
    Globals  *DictObject
    Defaults []Object            // default argument values
    KwDefaults map[string]Object // keyword-only defaults
    Closure  []*CellObject       // free variables
    Name     string
    Doc      string
}
```

When called, the VM creates a new `Frame` for the function's `Code`.

### 5.2 Builtin Function

```go
type BuiltinFuncObject struct {
    Name string
    Fn   func(args []Object, kwargs map[string]Object) (Object, error)
}
```

Represents built-in functions (`print`, `len`, `range`, etc.) implemented in Go.

### 5.3 Bound Method

```go
type BoundMethodObject struct {
    Self     Object
    Function Object  // FunctionObject or BuiltinFuncObject
}
```

Created when accessing a function attribute on an instance (`obj.method`). The `Self` is prepended to args on call.

### 5.4 Cell

```go
type CellObject struct {
    Value Object  // nil if not yet bound
}
```

Cells implement closure variable sharing between nested functions. Both the outer function's local scope and the inner function's free variable list point to the same `*CellObject`.

---

## 6. Class and Instance Types

### 6.1 User-Defined Type (Class)

User-defined classes are `*TypeObject` values where `Slots` fields are populated by the class's `__dunder__` methods rather than hardcoded Go functions.

```go
// When the compiler emits LOAD_BUILD_CLASS + CALL_FUNCTION,
// the runtime creates a TypeObject from:
//   - class name
//   - base classes
//   - class body's namespace dict (from executing the class body as a code object)
```

Attribute lookup on a class follows MRO (Method Resolution Order). PyGo must implement the C3 linearization algorithm for MRO computation.

### 6.2 Instance

```go
type InstanceObject struct {
    Type_  *TypeObject
    Dict   *DictObject  // instance's __dict__
}
```

Instances are created by calling a type object. Attribute lookup: check `instance.__dict__` first, then `type(instance).__mro__` chain.

Descriptor protocol: if a name resolves to a descriptor (an object with `__get__`), the descriptor is invoked rather than returning the raw value. This supports `property`, `classmethod`, `staticmethod`, and custom descriptors.

---

## 7. Iterator and Generator Types

### 7.1 Generic Iterator

Many types produce iterators. Built-in iterators hold a reference to the underlying container and a current position.

```go
type ListIterator struct {
    List  *ListObject
    Index int
}

func (it *ListIterator) Next() (Object, error) {
    if it.Index >= len(it.List.Items) {
        return nil, ErrStopIteration
    }
    v := it.List.Items[it.Index]
    it.Index++
    return v, nil
}
```

### 7.2 Generator

Generators are functions containing `yield`. They execute lazily.

```go
type GeneratorObject struct {
    Frame     *vm.Frame  // suspended execution frame
    Started   bool
    Finished  bool
    Name      string
}
```

When a generator function is called, instead of immediately executing the body, PyGo creates a `GeneratorObject` with a fresh `Frame` pointing at the function's code. The frame is not yet running.

On each `next(gen)` call:
1. Resume the frame from its saved `PC`.
2. Execute until `YIELD_VALUE` opcode.
3. Pop the yield value from the stack, save frame state, return the value.
4. On subsequent `next()`, continue from saved `PC`.

This is implemented by giving the VM a "run until yield or return" execution mode. No goroutine is allocated per generator. The frame's stack and local variables persist between `next()` calls in the `GeneratorObject`.

**This is the correct approach.** Using a goroutine per generator (goroutine-per-fiber) would be tempting in Go but creates significant overhead (each goroutine has a minimum stack), complicates cancellation, and makes it impossible to share generators across goroutines safely.

---

## 8. Exception Types

```go
type ExceptionObject struct {
    Type_    *TypeObject
    Message  string
    Args     *TupleObject
    Traceback *TracebackObject
    Cause    *ExceptionObject  // __cause__ (from x from y)
    Context  *ExceptionObject  // __context__ (implicit chaining)
}

type TracebackObject struct {
    Frames []TracebackFrame
}

type TracebackFrame struct {
    Filename string
    FuncName string
    Line     int
}
```

Built-in exception hierarchy mirrors Python's:

```
BaseException
├── SystemExit
├── KeyboardInterrupt
├── GeneratorExit
└── Exception
    ├── StopIteration
    ├── ArithmeticError
    │   ├── ZeroDivisionError
    │   └── OverflowError
    ├── LookupError
    │   ├── IndexError
    │   └── KeyError
    ├── NameError
    │   └── UnboundLocalError
    ├── TypeError
    ├── ValueError
    ├── AttributeError
    ├── ImportError
    │   └── ModuleNotFoundError
    ├── OSError
    ├── RuntimeError
    │   └── RecursionError
    ├── NotImplementedError
    └── SyntaxError
```

Go errors crossing into Python are wrapped as `RuntimeError` by default, with the Go error message preserved.

---

## 9. Module Type

```go
type ModuleObject struct {
    Name    string
    Dict    *DictObject  // module's global namespace (__dict__)
    Loader  string       // for __spec__.__loader__ representation
}
```

Modules are plain objects whose attributes are stored in `Dict`. Accessing `module.attr` is equivalent to `module.__dict__['attr']`.

---

## 10. Go Bridge Type

```go
type GoObject struct {
    Value reflect.Value  // the wrapped Go value
    Type_ *TypeObject    // a dynamically constructed TypeObject
}
```

When a Go value is registered with the runtime (via `RegisterFunction` or `Register`), it is wrapped in a `GoObject`. Method calls on a `GoObject` use `reflect` to invoke the corresponding Go method.

See `docs/interop.md` for the full design.

---

## 11. Attribute and Method Lookup

Python's attribute lookup is deliberately complex. PyGo must implement it correctly to support user-defined classes, descriptors, and the standard library.

### Instance attribute lookup (`obj.name`)

```
1. type(obj).__mro__ — search each type in MRO order
   for each base_type:
       if name in base_type.__dict__:
           descriptor = base_type.__dict__[name]
           if descriptor has __get__ AND __set__ (data descriptor):
               return descriptor.__get__(obj, type(obj))  ← data descriptor wins

2. obj.__dict__[name]  ← instance dict

3. type(obj).__mro__ — search again for non-data descriptors
   for each base_type:
       if name in base_type.__dict__:
           descriptor = base_type.__dict__[name]
           if descriptor has __get__ (non-data descriptor):
               return descriptor.__get__(obj, type(obj))
           else:
               return descriptor  ← plain class attribute

4. AttributeError
```

This is the standard CPython lookup order. It must be implemented correctly for `property`, `classmethod`, `staticmethod`, and any custom descriptor to work.

### Type attribute lookup (`Type.name`)

```
1. type(Type).__mro__ — metatype chain (usually just type and object)
2. Type.__mro__ — the type's own MRO
3. AttributeError
```

---

## 12. Operator Protocol

Binary operations (`a + b`) follow Python's left-operand-first, then right-operand protocol:

```
1. If type(b) is a strict subtype of type(a):
       try b.__radd__(a) first
2. Try a.__add__(b)
3. If result is NotImplemented: try b.__radd__(a)
4. If both return NotImplemented: raise TypeError
```

This must be implemented correctly — it affects all numeric and string operations.

Augmented assignment (`a += b`) checks `__iadd__` first, then falls back to `__add__`.

---

## 13. Hashing and Equality

Rules:
- Objects that compare equal must have the same hash: `a == b → hash(a) == hash(b)`.
- Mutable objects (`list`, `dict`, `set`) are not hashable.
- User-defined classes are hashable by default (hash of object identity) unless they define `__eq__` without `__hash__`.

```go
// Hash computation for built-in types:
// int:    hash of the integer value (must match float hash for equal values: hash(1) == hash(1.0))
// float:  hash following CPython's algorithm (sys.hash_info)
// str:    FNV or SipHash of UTF-8 bytes (seeded per-runtime for hash randomization)
// tuple:  combination of element hashes
// bool:   hash(True) == hash(1), hash(False) == hash(0)
// None:   constant (e.g., 0)
```

Hash randomization (seeded at runtime creation) is important for security (hash-flooding prevention) but can be disabled for testing reproducibility.

---

## 14. Type Hierarchy

```
object
├── NoneType         (None)
├── bool             (True, False) — subtype of int
├── int
├── float
├── complex          (Phase 2+)
├── str
├── bytes
├── bytearray        (Phase 3+)
├── list
├── tuple
├── dict
├── set
├── frozenset
├── type             (metatype — type of all types)
├── function
├── builtin_function_or_method
├── method
├── cell
├── module
├── iterator types   (listiterator, rangeiterator, etc.)
├── generator
├── coroutine        (Phase 4+)
├── BaseException
│   └── Exception ...
└── GoObject         (PyGo extension — wraps Go values)
```

`type` is the metatype: `type(int) == type`. `type` is its own type: `type(type) == type`. This circular relationship is handled by initializing the `TypeObject` for `type` with itself as its own type — a special case at runtime initialization, not a general language mechanism.

---

## 15. Memory Model

PyGo uses the Go garbage collector for all memory management.

**Rationale:**
- Implementing reference counting in Go is possible but creates significant complexity: every assignment to an `Object` field requires inc/dec calls, and reference cycles require a backup cycle collector anyway.
- Go's GC handles cycles correctly and has improved substantially in recent versions (sub-millisecond pause times in Go 1.17+).
- The added complexity of reference counting is not justified until performance profiling shows GC pressure is a real problem.

**Implications:**
- Python finalizers (`__del__`) cannot be guaranteed to run at a deterministic time. This is documented behavior.
- `weakref` support requires Go's `runtime.SetFinalizer` — feasible but deferred to Phase 3+.
- Large numbers of short-lived Python objects (tight loops creating temporary values) may create GC pressure. The small-int cache and string interning mitigate the most common cases. Object pooling for the value stack is a future optimization.

---

## 16. Identity and Mutability Table

| Type | Mutable | Hashable | Identity |
|------|---------|----------|----------|
| `int` | No | Yes | By value |
| `float` | No | Yes | By value |
| `str` | No | Yes | By value (interned for short strings) |
| `bool` | No | Yes | Singleton (True, False) |
| `None` | No | Yes | Singleton |
| `bytes` | No | Yes | By value |
| `tuple` | No | Yes (if elements hashable) | By pointer |
| `list` | Yes | No | By pointer |
| `dict` | Yes | No | By pointer |
| `set` | Yes | No | By pointer |
| `frozenset` | No | Yes (if elements hashable) | By pointer |
| `function` | No | Yes (by identity) | By pointer |
| `type` | Restricted | Yes (by identity) | By pointer |
| `instance` | Yes | Yes (by identity, unless `__hash__` defined) | By pointer |
| `module` | Yes | No | By pointer |
| `GoObject` | Depends on Go type | No (initially) | By pointer |
