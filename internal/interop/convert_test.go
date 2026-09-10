package interop

import (
	"math"
	"math/big"
	"reflect"
	"strings"
	"testing"

	"github.com/alvinjaison/pygo/internal/object"
)

// ============================================================
// GoToPython tests
// ============================================================

func TestGoToPython_Nil(t *testing.T) {
	obj, err := GoToPython(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !object.IsNone(obj) {
		t.Errorf("expected None, got %v", obj)
	}
}

func TestGoToPython_Bool(t *testing.T) {
	obj, err := GoToPython(true)
	if err != nil {
		t.Fatal(err)
	}
	b, ok := obj.(*object.BoolObject)
	if !ok {
		t.Fatalf("expected *BoolObject, got %T", obj)
	}
	if !b.Value {
		t.Error("expected true")
	}

	obj, err = GoToPython(false)
	if err != nil {
		t.Fatal(err)
	}
	b = obj.(*object.BoolObject)
	if b.Value {
		t.Error("expected false")
	}
}

func TestGoToPython_IntTypes(t *testing.T) {
	tests := []struct {
		name string
		val  interface{}
		want int64
	}{
		{"int", int(42), 42},
		{"int8", int8(-5), -5},
		{"int16", int16(1000), 1000},
		{"int32", int32(-100000), -100000},
		{"int64", int64(math.MaxInt64), math.MaxInt64},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj, err := GoToPython(tt.val)
			if err != nil {
				t.Fatal(err)
			}
			intObj, ok := obj.(*object.IntObject)
			if !ok {
				t.Fatalf("expected *IntObject, got %T", obj)
			}
			got, ok := intObj.Int64()
			if !ok {
				t.Fatal("value does not fit in int64")
			}
			if got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestGoToPython_UintTypes(t *testing.T) {
	tests := []struct {
		name string
		val  interface{}
		want int64
	}{
		{"uint", uint(42), 42},
		{"uint8", uint8(255), 255},
		{"uint16", uint16(65535), 65535},
		{"uint32", uint32(100000), 100000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj, err := GoToPython(tt.val)
			if err != nil {
				t.Fatal(err)
			}
			intObj := obj.(*object.IntObject)
			got, _ := intObj.Int64()
			if got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestGoToPython_Uint64Overflow(t *testing.T) {
	// uint64 value larger than MaxInt64.
	val := uint64(math.MaxInt64) + 1
	obj, err := GoToPython(val)
	if err != nil {
		t.Fatal(err)
	}
	intObj := obj.(*object.IntObject)
	// Should not fit in int64.
	_, fits := intObj.Int64()
	if fits {
		t.Error("expected value to not fit in int64")
	}
	// But should match the original value via big.Int.
	expected := new(big.Int).SetUint64(val)
	if intObj.Value.Cmp(expected) != 0 {
		t.Errorf("got %s, want %s", intObj.Value.String(), expected.String())
	}
}

func TestGoToPython_Float(t *testing.T) {
	obj, err := GoToPython(float64(3.14))
	if err != nil {
		t.Fatal(err)
	}
	f, ok := obj.(*object.FloatObject)
	if !ok {
		t.Fatalf("expected *FloatObject, got %T", obj)
	}
	if f.Value != 3.14 {
		t.Errorf("got %f, want 3.14", f.Value)
	}

	// float32
	obj, err = GoToPython(float32(2.5))
	if err != nil {
		t.Fatal(err)
	}
	f = obj.(*object.FloatObject)
	if f.Value != 2.5 {
		t.Errorf("got %f, want 2.5", f.Value)
	}
}

func TestGoToPython_String(t *testing.T) {
	obj, err := GoToPython("hello")
	if err != nil {
		t.Fatal(err)
	}
	s, ok := obj.(*object.StrObject)
	if !ok {
		t.Fatalf("expected *StrObject, got %T", obj)
	}
	if s.Value != "hello" {
		t.Errorf("got %q, want %q", s.Value, "hello")
	}
}

func TestGoToPython_Slice(t *testing.T) {
	obj, err := GoToPython([]int{1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}
	list, ok := obj.(*object.ListObject)
	if !ok {
		t.Fatalf("expected *ListObject, got %T", obj)
	}
	if list.Len() != 3 {
		t.Fatalf("expected len 3, got %d", list.Len())
	}
	// Check elements.
	for i, want := range []int64{1, 2, 3} {
		intObj := list.Items[i].(*object.IntObject)
		got, _ := intObj.Int64()
		if got != want {
			t.Errorf("item[%d] = %d, want %d", i, got, want)
		}
	}
}

func TestGoToPython_NestedSlice(t *testing.T) {
	obj, err := GoToPython([][]string{{"a", "b"}, {"c"}})
	if err != nil {
		t.Fatal(err)
	}
	list := obj.(*object.ListObject)
	if list.Len() != 2 {
		t.Fatalf("expected outer len 2, got %d", list.Len())
	}
	inner := list.Items[0].(*object.ListObject)
	if inner.Len() != 2 {
		t.Fatalf("expected inner len 2, got %d", inner.Len())
	}
	s := inner.Items[0].(*object.StrObject)
	if s.Value != "a" {
		t.Errorf("got %q, want %q", s.Value, "a")
	}
}

func TestGoToPython_Map(t *testing.T) {
	obj, err := GoToPython(map[string]int{"x": 10, "y": 20})
	if err != nil {
		t.Fatal(err)
	}
	d, ok := obj.(*object.DictObject)
	if !ok {
		t.Fatalf("expected *DictObject, got %T", obj)
	}
	if d.Len() != 2 {
		t.Fatalf("expected len 2, got %d", d.Len())
	}
	// Check "x" → 10.
	val, err := d.GetItem(object.StrFrom("x"))
	if err != nil {
		t.Fatal(err)
	}
	intObj := val.(*object.IntObject)
	got, _ := intObj.Int64()
	if got != 10 {
		t.Errorf("d['x'] = %d, want 10", got)
	}
}

func TestGoToPython_UnsupportedType(t *testing.T) {
	type custom struct{ X int }
	_, err := GoToPython(custom{X: 1})
	if err == nil {
		t.Fatal("expected error for unsupported type")
	}
	if !strings.Contains(err.Error(), "TypeError") {
		t.Errorf("expected TypeError, got: %v", err)
	}
}

func TestGoToPython_NilSlice(t *testing.T) {
	var s []int
	obj, err := GoToPython(s)
	if err != nil {
		t.Fatal(err)
	}
	if !object.IsNone(obj) {
		t.Errorf("expected None for nil slice, got %v", obj)
	}
}

func TestGoToPython_NilMap(t *testing.T) {
	var m map[string]int
	obj, err := GoToPython(m)
	if err != nil {
		t.Fatal(err)
	}
	if !object.IsNone(obj) {
		t.Errorf("expected None for nil map, got %v", obj)
	}
}

// ============================================================
// PythonToGo tests
// ============================================================

func TestPythonToGo_Bool(t *testing.T) {
	v, err := PythonToGo(object.PyTrue, reflect.TypeOf(false))
	if err != nil {
		t.Fatal(err)
	}
	if !v.Bool() {
		t.Error("expected true")
	}

	v, err = PythonToGo(object.PyFalse, reflect.TypeOf(false))
	if err != nil {
		t.Fatal(err)
	}
	if v.Bool() {
		t.Error("expected false")
	}
}

func TestPythonToGo_Int(t *testing.T) {
	obj := object.IntFrom(42)

	tests := []struct {
		name   string
		target reflect.Type
		want   int64
	}{
		{"int", reflect.TypeOf(int(0)), 42},
		{"int8", reflect.TypeOf(int8(0)), 42},
		{"int16", reflect.TypeOf(int16(0)), 42},
		{"int32", reflect.TypeOf(int32(0)), 42},
		{"int64", reflect.TypeOf(int64(0)), 42},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := PythonToGo(obj, tt.target)
			if err != nil {
				t.Fatal(err)
			}
			if v.Int() != tt.want {
				t.Errorf("got %d, want %d", v.Int(), tt.want)
			}
		})
	}
}

func TestPythonToGo_Uint(t *testing.T) {
	obj := object.IntFrom(100)

	tests := []struct {
		name   string
		target reflect.Type
		want   uint64
	}{
		{"uint", reflect.TypeOf(uint(0)), 100},
		{"uint8", reflect.TypeOf(uint8(0)), 100},
		{"uint16", reflect.TypeOf(uint16(0)), 100},
		{"uint32", reflect.TypeOf(uint32(0)), 100},
		{"uint64", reflect.TypeOf(uint64(0)), 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := PythonToGo(obj, tt.target)
			if err != nil {
				t.Fatal(err)
			}
			if v.Uint() != tt.want {
				t.Errorf("got %d, want %d", v.Uint(), tt.want)
			}
		})
	}
}

func TestPythonToGo_IntOverflow(t *testing.T) {
	// int8 max is 127.
	obj := object.IntFrom(200)
	_, err := PythonToGo(obj, reflect.TypeOf(int8(0)))
	if err == nil {
		t.Fatal("expected overflow error")
	}
	if !strings.Contains(err.Error(), "OverflowError") {
		t.Errorf("expected OverflowError, got: %v", err)
	}
}

func TestPythonToGo_NegativeToUint(t *testing.T) {
	obj := object.IntFrom(-1)
	_, err := PythonToGo(obj, reflect.TypeOf(uint(0)))
	if err == nil {
		t.Fatal("expected overflow error")
	}
	if !strings.Contains(err.Error(), "OverflowError") {
		t.Errorf("expected OverflowError, got: %v", err)
	}
}

func TestPythonToGo_Float(t *testing.T) {
	obj := object.FloatFrom(3.14)

	v, err := PythonToGo(obj, reflect.TypeOf(float64(0)))
	if err != nil {
		t.Fatal(err)
	}
	if v.Float() != 3.14 {
		t.Errorf("got %f, want 3.14", v.Float())
	}

	v, err = PythonToGo(obj, reflect.TypeOf(float32(0)))
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(v.Float()-3.14) > 0.001 {
		t.Errorf("got %f, want ~3.14", v.Float())
	}
}

func TestPythonToGo_IntToFloat(t *testing.T) {
	// Python int should convert to Go float.
	obj := object.IntFrom(42)
	v, err := PythonToGo(obj, reflect.TypeOf(float64(0)))
	if err != nil {
		t.Fatal(err)
	}
	if v.Float() != 42.0 {
		t.Errorf("got %f, want 42.0", v.Float())
	}
}

func TestPythonToGo_String(t *testing.T) {
	obj := object.StrFrom("hello")
	v, err := PythonToGo(obj, reflect.TypeOf(""))
	if err != nil {
		t.Fatal(err)
	}
	if v.String() != "hello" {
		t.Errorf("got %q, want %q", v.String(), "hello")
	}
}

func TestPythonToGo_NoneToPointer(t *testing.T) {
	v, err := PythonToGo(object.None, reflect.TypeOf((*int)(nil)))
	if err != nil {
		t.Fatal(err)
	}
	if !v.IsNil() {
		t.Error("expected nil pointer")
	}
}

func TestPythonToGo_NoneToInterface(t *testing.T) {
	v, err := PythonToGo(object.None, reflect.TypeOf((*interface{})(nil)).Elem())
	if err != nil {
		t.Fatal(err)
	}
	if v.IsValid() && !v.IsNil() {
		t.Error("expected nil/zero value")
	}
}

func TestPythonToGo_ListToSlice(t *testing.T) {
	items := []object.Object{
		object.IntFrom(1),
		object.IntFrom(2),
		object.IntFrom(3),
	}
	list := object.NewList(items)

	v, err := PythonToGo(list, reflect.TypeOf([]int64{}))
	if err != nil {
		t.Fatal(err)
	}
	slice := v.Interface().([]int64)
	if len(slice) != 3 {
		t.Fatalf("expected len 3, got %d", len(slice))
	}
	for i, want := range []int64{1, 2, 3} {
		if slice[i] != want {
			t.Errorf("slice[%d] = %d, want %d", i, slice[i], want)
		}
	}
}

func TestPythonToGo_TupleToSlice(t *testing.T) {
	items := []object.Object{
		object.StrFrom("a"),
		object.StrFrom("b"),
	}
	tuple := object.NewTuple(items)

	v, err := PythonToGo(tuple, reflect.TypeOf([]string{}))
	if err != nil {
		t.Fatal(err)
	}
	slice := v.Interface().([]string)
	if len(slice) != 2 || slice[0] != "a" || slice[1] != "b" {
		t.Errorf("got %v, want [a b]", slice)
	}
}

func TestPythonToGo_DictToMap(t *testing.T) {
	d := object.NewDict()
	_ = d.SetItem(object.StrFrom("x"), object.IntFrom(10))
	_ = d.SetItem(object.StrFrom("y"), object.IntFrom(20))

	v, err := PythonToGo(d, reflect.TypeOf(map[string]int64{}))
	if err != nil {
		t.Fatal(err)
	}
	m := v.Interface().(map[string]int64)
	if m["x"] != 10 || m["y"] != 20 {
		t.Errorf("got %v, want map[x:10 y:20]", m)
	}
}

func TestPythonToGo_NestedList(t *testing.T) {
	inner1 := object.NewList([]object.Object{object.IntFrom(1), object.IntFrom(2)})
	inner2 := object.NewList([]object.Object{object.IntFrom(3)})
	outer := object.NewList([]object.Object{inner1, inner2})

	v, err := PythonToGo(outer, reflect.TypeOf([][]int64{}))
	if err != nil {
		t.Fatal(err)
	}
	result := v.Interface().([][]int64)
	if len(result) != 2 {
		t.Fatalf("expected 2 inner slices, got %d", len(result))
	}
	if len(result[0]) != 2 || result[0][0] != 1 || result[0][1] != 2 {
		t.Errorf("result[0] = %v, want [1 2]", result[0])
	}
	if len(result[1]) != 1 || result[1][0] != 3 {
		t.Errorf("result[1] = %v, want [3]", result[1])
	}
}

func TestPythonToGo_TypeError(t *testing.T) {
	// Trying to convert a string to int should fail.
	obj := object.StrFrom("hello")
	_, err := PythonToGo(obj, reflect.TypeOf(int(0)))
	if err == nil {
		t.Fatal("expected TypeError")
	}
	if !strings.Contains(err.Error(), "TypeError") {
		t.Errorf("expected TypeError, got: %v", err)
	}
}

func TestPythonToGo_FloatToInt_TypeError(t *testing.T) {
	// Float → int should not be allowed (Python semantics: explicit int() required).
	obj := object.FloatFrom(3.14)
	_, err := PythonToGo(obj, reflect.TypeOf(int(0)))
	if err == nil {
		t.Fatal("expected TypeError")
	}
	if !strings.Contains(err.Error(), "TypeError") {
		t.Errorf("expected TypeError, got: %v", err)
	}
}

func TestPythonToGo_BoolToInt(t *testing.T) {
	// Python bool is a subtype of int, so bool → int should work.
	v, err := PythonToGo(object.PyTrue, reflect.TypeOf(int(0)))
	if err != nil {
		t.Fatal(err)
	}
	if v.Int() != 1 {
		t.Errorf("got %d, want 1", v.Int())
	}
}

func TestPythonToGo_BigIntOverflow(t *testing.T) {
	// A very large integer should overflow any fixed-size Go int type.
	bigVal := new(big.Int).Exp(big.NewInt(2), big.NewInt(100), nil)
	obj := object.IntFromBig(bigVal)

	_, err := PythonToGo(obj, reflect.TypeOf(int64(0)))
	if err == nil {
		t.Fatal("expected OverflowError")
	}
	if !strings.Contains(err.Error(), "OverflowError") {
		t.Errorf("expected OverflowError, got: %v", err)
	}
}

// ============================================================
// Edge case: nested dict conversion (Requirements 7.6)
// ============================================================

func TestGoToPython_NestedMap(t *testing.T) {
	nested := map[string]map[string]int{
		"a": {"x": 1, "y": 2},
		"b": {"z": 3},
	}
	obj, err := GoToPython(nested)
	if err != nil {
		t.Fatal(err)
	}
	outerDict, ok := obj.(*object.DictObject)
	if !ok {
		t.Fatalf("expected *DictObject, got %T", obj)
	}
	if outerDict.Len() != 2 {
		t.Fatalf("expected outer len 2, got %d", outerDict.Len())
	}
	// Check inner dict "a".
	valA, err := outerDict.GetItem(object.StrFrom("a"))
	if err != nil {
		t.Fatal(err)
	}
	innerA, ok := valA.(*object.DictObject)
	if !ok {
		t.Fatalf("expected inner *DictObject, got %T", valA)
	}
	if innerA.Len() != 2 {
		t.Fatalf("expected inner len 2, got %d", innerA.Len())
	}
	valX, err := innerA.GetItem(object.StrFrom("x"))
	if err != nil {
		t.Fatal(err)
	}
	intObj := valX.(*object.IntObject)
	got, _ := intObj.Int64()
	if got != 1 {
		t.Errorf("d['a']['x'] = %d, want 1", got)
	}
}

func TestPythonToGo_NestedDict(t *testing.T) {
	// Build Python dict of dicts: {"a": {"x": 1, "y": 2}, "b": {"z": 3}}
	innerA := object.NewDict()
	_ = innerA.SetItem(object.StrFrom("x"), object.IntFrom(1))
	_ = innerA.SetItem(object.StrFrom("y"), object.IntFrom(2))

	innerB := object.NewDict()
	_ = innerB.SetItem(object.StrFrom("z"), object.IntFrom(3))

	outer := object.NewDict()
	_ = outer.SetItem(object.StrFrom("a"), innerA)
	_ = outer.SetItem(object.StrFrom("b"), innerB)

	v, err := PythonToGo(outer, reflect.TypeOf(map[string]map[string]int64{}))
	if err != nil {
		t.Fatal(err)
	}
	m := v.Interface().(map[string]map[string]int64)
	if m["a"]["x"] != 1 || m["a"]["y"] != 2 {
		t.Errorf("m[\"a\"] = %v, want map[x:1 y:2]", m["a"])
	}
	if m["b"]["z"] != 3 {
		t.Errorf("m[\"b\"] = %v, want map[z:3]", m["b"])
	}
}

// ============================================================
// Edge case: empty collections (Requirements 7.6)
// ============================================================

func TestGoToPython_EmptySlice(t *testing.T) {
	// A non-nil empty slice should convert to an empty list, not None.
	obj, err := GoToPython(make([]int, 0))
	if err != nil {
		t.Fatal(err)
	}
	list, ok := obj.(*object.ListObject)
	if !ok {
		t.Fatalf("expected *ListObject, got %T", obj)
	}
	if list.Len() != 0 {
		t.Errorf("expected empty list, got len %d", list.Len())
	}
}

func TestGoToPython_EmptyMap(t *testing.T) {
	// A non-nil empty map should convert to an empty dict, not None.
	obj, err := GoToPython(make(map[string]int))
	if err != nil {
		t.Fatal(err)
	}
	d, ok := obj.(*object.DictObject)
	if !ok {
		t.Fatalf("expected *DictObject, got %T", obj)
	}
	if d.Len() != 0 {
		t.Errorf("expected empty dict, got len %d", d.Len())
	}
}

func TestPythonToGo_EmptyList(t *testing.T) {
	list := object.NewList([]object.Object{})
	v, err := PythonToGo(list, reflect.TypeOf([]int64{}))
	if err != nil {
		t.Fatal(err)
	}
	slice := v.Interface().([]int64)
	if len(slice) != 0 {
		t.Errorf("expected empty slice, got len %d", len(slice))
	}
}

func TestPythonToGo_EmptyDict(t *testing.T) {
	d := object.NewDict()
	v, err := PythonToGo(d, reflect.TypeOf(map[string]int64{}))
	if err != nil {
		t.Fatal(err)
	}
	m := v.Interface().(map[string]int64)
	if len(m) != 0 {
		t.Errorf("expected empty map, got len %d", len(m))
	}
}

func TestPythonToGo_EmptyTuple(t *testing.T) {
	tuple := object.NewTuple([]object.Object{})
	v, err := PythonToGo(tuple, reflect.TypeOf([]string{}))
	if err != nil {
		t.Fatal(err)
	}
	slice := v.Interface().([]string)
	if len(slice) != 0 {
		t.Errorf("expected empty slice, got len %d", len(slice))
	}
}

// ============================================================
// Edge case: large integer round-trip (Requirements 7.7)
// ============================================================

func TestGoToPython_LargeUint64_Roundtrip(t *testing.T) {
	// uint64 larger than MaxInt64 → Python big int → back to uint64.
	original := uint64(math.MaxUint64)
	obj, err := GoToPython(original)
	if err != nil {
		t.Fatal(err)
	}
	intObj, ok := obj.(*object.IntObject)
	if !ok {
		t.Fatalf("expected *IntObject, got %T", obj)
	}
	// Convert back.
	v, err := PythonToGo(intObj, reflect.TypeOf(uint64(0)))
	if err != nil {
		t.Fatal(err)
	}
	if v.Uint() != original {
		t.Errorf("round-trip failed: got %d, want %d", v.Uint(), original)
	}
}

func TestPythonToGo_BigIntToUint64_Overflow(t *testing.T) {
	// A big int larger than MaxUint64 should overflow uint64.
	bigVal := new(big.Int).Add(
		new(big.Int).SetUint64(math.MaxUint64),
		big.NewInt(1),
	)
	obj := object.IntFromBig(bigVal)
	_, err := PythonToGo(obj, reflect.TypeOf(uint64(0)))
	if err == nil {
		t.Fatal("expected OverflowError")
	}
	if !strings.Contains(err.Error(), "OverflowError") {
		t.Errorf("expected OverflowError, got: %v", err)
	}
}

// ============================================================
// Type cache test
// ============================================================

func TestConverterCacheIsUsed(t *testing.T) {
	// Calling PythonToGo multiple times with the same target type should
	// use the cached converter. We just verify it doesn't error.
	obj := object.IntFrom(5)
	target := reflect.TypeOf(int64(0))

	for i := 0; i < 10; i++ {
		v, err := PythonToGo(obj, target)
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		if v.Int() != 5 {
			t.Fatalf("iteration %d: got %d, want 5", i, v.Int())
		}
	}
}
