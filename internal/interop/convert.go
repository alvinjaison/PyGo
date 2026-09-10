// Package interop bridges Go values to/from Python objects using reflection.
// It provides the core type conversion layer for the PyGo Go↔Python boundary.
package interop

import (
	"fmt"
	"math"
	"math/big"
	"reflect"
	"sync"

	"github.com/alvinjaison/pygo/internal/object"
)

// ============================================================
// Type conversion cache
// ============================================================

// converterCache maps reflect.Type → conversion function for PythonToGo.
// This avoids repeated type-switch logic for the same target type.
var converterCache sync.Map // map[reflect.Type]converterFunc

// converterFunc converts a Python object to a Go reflect.Value of a specific type.
type converterFunc func(obj object.Object) (reflect.Value, error)

// ============================================================
// GoToPython: Go values → Python objects
// ============================================================

// GoToPython converts a Go value to a Python object.
// Supported types: nil, bool, int types, uint types, float32/64, string,
// []T (recursive), map[K]V (recursive).
// Returns an error for unsupported types.
func GoToPython(v interface{}) (object.Object, error) {
	if v == nil {
		return object.None, nil
	}

	// Check for nil interface wrapping a typed nil pointer/slice/map.
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Slice, reflect.Map:
		if rv.IsNil() {
			return object.None, nil
		}
	}

	switch val := v.(type) {
	case bool:
		return object.BoolFrom(val), nil

	case int:
		return object.IntFrom(int64(val)), nil
	case int8:
		return object.IntFrom(int64(val)), nil
	case int16:
		return object.IntFrom(int64(val)), nil
	case int32:
		return object.IntFrom(int64(val)), nil
	case int64:
		return object.IntFrom(val), nil

	case uint:
		return object.IntFrom(int64(val)), nil
	case uint8:
		return object.IntFrom(int64(val)), nil
	case uint16:
		return object.IntFrom(int64(val)), nil
	case uint32:
		return object.IntFrom(int64(val)), nil
	case uint64:
		// uint64 may overflow int64; use big.Int for large values.
		if val > math.MaxInt64 {
			bi := new(big.Int).SetUint64(val)
			return object.IntFromBig(bi), nil
		}
		return object.IntFrom(int64(val)), nil

	case float32:
		return object.FloatFrom(float64(val)), nil
	case float64:
		return object.FloatFrom(val), nil

	case string:
		return object.StrFrom(val), nil
	}

	// Handle slices and maps via reflection.
	return goToPythonReflect(rv)
}

// goToPythonReflect handles slice and map conversion via reflect.
func goToPythonReflect(rv reflect.Value) (object.Object, error) {
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		items := make([]object.Object, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			elem, err := GoToPython(rv.Index(i).Interface())
			if err != nil {
				return nil, fmt.Errorf("list element %d: %w", i, err)
			}
			items[i] = elem
		}
		return object.NewList(items), nil

	case reflect.Map:
		d := object.NewDict()
		iter := rv.MapRange()
		for iter.Next() {
			key, err := GoToPython(iter.Key().Interface())
			if err != nil {
				return nil, fmt.Errorf("dict key: %w", err)
			}
			val, err := GoToPython(iter.Value().Interface())
			if err != nil {
				return nil, fmt.Errorf("dict value: %w", err)
			}
			if err := d.SetItem(key, val); err != nil {
				return nil, fmt.Errorf("dict SetItem: %w", err)
			}
		}
		return d, nil

	case reflect.Ptr:
		// Dereference pointer and convert the element.
		if rv.IsNil() {
			return object.None, nil
		}
		return GoToPython(rv.Elem().Interface())

	default:
		return nil, fmt.Errorf("TypeError: cannot convert Go type '%s' to Python object", rv.Type().String())
	}
}

// ============================================================
// PythonToGo: Python objects → Go values
// ============================================================

// PythonToGo converts a Python object to a Go reflect.Value of the specified target type.
// It raises TypeError for incompatible conversions and OverflowError for integer overflow.
func PythonToGo(obj object.Object, targetType reflect.Type) (reflect.Value, error) {
	// Check the converter cache first.
	if cached, ok := converterCache.Load(targetType); ok {
		return cached.(converterFunc)(obj)
	}

	// Build a converter for this target type.
	conv := buildConverter(targetType)
	converterCache.Store(targetType, conv)
	return conv(obj)
}

// buildConverter creates a converter function for the given target type.
func buildConverter(targetType reflect.Type) converterFunc {
	switch targetType.Kind() {
	case reflect.Bool:
		return convertToBool

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return makeIntConverter(targetType)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return makeUintConverter(targetType)

	case reflect.Float32, reflect.Float64:
		return makeFloatConverter(targetType)

	case reflect.String:
		return convertToString

	case reflect.Slice:
		return makeSliceConverter(targetType)

	case reflect.Map:
		return makeMapConverter(targetType)

	case reflect.Interface:
		return convertToInterface(targetType)

	case reflect.Ptr:
		return makePtrConverter(targetType)

	default:
		return func(obj object.Object) (reflect.Value, error) {
			return reflect.Value{}, newTypeError(obj, targetType)
		}
	}
}

// ============================================================
// Primitive converters
// ============================================================

func convertToBool(obj object.Object) (reflect.Value, error) {
	switch v := obj.(type) {
	case *object.BoolObject:
		return reflect.ValueOf(v.Value), nil
	default:
		return reflect.Value{}, newTypeError(obj, reflect.TypeOf(false))
	}
}

func makeIntConverter(targetType reflect.Type) converterFunc {
	return func(obj object.Object) (reflect.Value, error) {
		var bigVal *big.Int

		switch v := obj.(type) {
		case *object.IntObject:
			bigVal = v.Value
		case *object.BoolObject:
			if v.Value {
				bigVal = big.NewInt(1)
			} else {
				bigVal = big.NewInt(0)
			}
		default:
			return reflect.Value{}, newTypeError(obj, targetType)
		}

		// Range check for the target int type.
		if !bigVal.IsInt64() {
			return reflect.Value{}, newOverflowError(bigVal, targetType)
		}
		i64 := bigVal.Int64()

		result := reflect.New(targetType).Elem()
		if result.OverflowInt(i64) {
			return reflect.Value{}, newOverflowError(bigVal, targetType)
		}
		result.SetInt(i64)
		return result, nil
	}
}

func makeUintConverter(targetType reflect.Type) converterFunc {
	return func(obj object.Object) (reflect.Value, error) {
		var bigVal *big.Int

		switch v := obj.(type) {
		case *object.IntObject:
			bigVal = v.Value
		case *object.BoolObject:
			if v.Value {
				bigVal = big.NewInt(1)
			} else {
				bigVal = big.NewInt(0)
			}
		default:
			return reflect.Value{}, newTypeError(obj, targetType)
		}

		// Must be non-negative.
		if bigVal.Sign() < 0 {
			return reflect.Value{}, newOverflowError(bigVal, targetType)
		}

		// Check if it fits in uint64.
		if !bigVal.IsUint64() {
			return reflect.Value{}, newOverflowError(bigVal, targetType)
		}
		u64 := bigVal.Uint64()

		result := reflect.New(targetType).Elem()
		if result.OverflowUint(u64) {
			return reflect.Value{}, newOverflowError(bigVal, targetType)
		}
		result.SetUint(u64)
		return result, nil
	}
}

func makeFloatConverter(targetType reflect.Type) converterFunc {
	return func(obj object.Object) (reflect.Value, error) {
		var f64 float64

		switch v := obj.(type) {
		case *object.FloatObject:
			f64 = v.Value
		case *object.IntObject:
			// Allow int → float conversion.
			if v.Value.IsInt64() {
				f64 = float64(v.Value.Int64())
			} else {
				// Use big.Float for big ints.
				bf := new(big.Float).SetInt(v.Value)
				f64, _ = bf.Float64()
			}
		default:
			return reflect.Value{}, newTypeError(obj, targetType)
		}

		result := reflect.New(targetType).Elem()
		if result.OverflowFloat(f64) {
			return reflect.Value{}, object.NewException(
				object.TypeOverflowError,
				fmt.Sprintf("Python float %v too large for Go %s", f64, targetType),
			)
		}
		result.SetFloat(f64)
		return result, nil
	}
}

func convertToString(obj object.Object) (reflect.Value, error) {
	switch v := obj.(type) {
	case *object.StrObject:
		return reflect.ValueOf(v.Value), nil
	default:
		return reflect.Value{}, newTypeError(obj, reflect.TypeOf(""))
	}
}

// ============================================================
// Collection converters
// ============================================================

func makeSliceConverter(targetType reflect.Type) converterFunc {
	elemType := targetType.Elem()
	return func(obj object.Object) (reflect.Value, error) {
		var items []object.Object

		switch v := obj.(type) {
		case *object.ListObject:
			items = v.Items
		case *object.TupleObject:
			items = v.Items
		default:
			return reflect.Value{}, newTypeError(obj, targetType)
		}

		result := reflect.MakeSlice(targetType, len(items), len(items))
		for i, item := range items {
			elem, err := PythonToGo(item, elemType)
			if err != nil {
				return reflect.Value{}, fmt.Errorf("element %d: %w", i, err)
			}
			result.Index(i).Set(elem)
		}
		return result, nil
	}
}

func makeMapConverter(targetType reflect.Type) converterFunc {
	keyType := targetType.Key()
	valType := targetType.Elem()
	return func(obj object.Object) (reflect.Value, error) {
		d, ok := obj.(*object.DictObject)
		if !ok {
			return reflect.Value{}, newTypeError(obj, targetType)
		}

		result := reflect.MakeMap(targetType)
		keys := d.Keys()
		vals := d.Values()
		for i, k := range keys {
			goKey, err := PythonToGo(k, keyType)
			if err != nil {
				return reflect.Value{}, fmt.Errorf("dict key: %w", err)
			}
			goVal, err := PythonToGo(vals[i], valType)
			if err != nil {
				return reflect.Value{}, fmt.Errorf("dict value: %w", err)
			}
			result.SetMapIndex(goKey, goVal)
		}
		return result, nil
	}
}

func convertToInterface(targetType reflect.Type) converterFunc {
	return func(obj object.Object) (reflect.Value, error) {
		if object.IsNone(obj) {
			return reflect.Zero(targetType), nil
		}
		// For interface{}, unwrap the Python object to a Go native value.
		if targetType.NumMethod() == 0 {
			// Empty interface — convert to the most natural Go type.
			return pythonToNativeGo(obj)
		}
		return reflect.Value{}, newTypeError(obj, targetType)
	}
}

func makePtrConverter(targetType reflect.Type) converterFunc {
	elemType := targetType.Elem()
	return func(obj object.Object) (reflect.Value, error) {
		if object.IsNone(obj) {
			return reflect.Zero(targetType), nil
		}
		// Convert element and take its address.
		elem, err := PythonToGo(obj, elemType)
		if err != nil {
			return reflect.Value{}, err
		}
		ptr := reflect.New(elemType)
		ptr.Elem().Set(elem)
		return ptr, nil
	}
}

// pythonToNativeGo converts a Python object to its most natural Go type
// when the target is interface{}.
func pythonToNativeGo(obj object.Object) (reflect.Value, error) {
	switch v := obj.(type) {
	case *object.BoolObject:
		return reflect.ValueOf(v.Value), nil
	case *object.IntObject:
		if v.Value.IsInt64() {
			return reflect.ValueOf(v.Value.Int64()), nil
		}
		// Return as *big.Int for large values.
		return reflect.ValueOf(new(big.Int).Set(v.Value)), nil
	case *object.FloatObject:
		return reflect.ValueOf(v.Value), nil
	case *object.StrObject:
		return reflect.ValueOf(v.Value), nil
	case *object.NoneObject:
		return reflect.ValueOf(nil), nil
	case *object.ListObject:
		items := make([]interface{}, len(v.Items))
		for i, item := range v.Items {
			val, err := pythonToNativeGo(item)
			if err != nil {
				return reflect.Value{}, err
			}
			if val.IsValid() {
				items[i] = val.Interface()
			}
		}
		return reflect.ValueOf(items), nil
	case *object.DictObject:
		m := make(map[string]interface{})
		keys := v.Keys()
		vals := v.Values()
		for i, k := range keys {
			keyStr, ok := k.(*object.StrObject)
			if !ok {
				return reflect.Value{}, fmt.Errorf("TypeError: cannot convert dict with non-string keys to Go map[string]interface{}")
			}
			val, err := pythonToNativeGo(vals[i])
			if err != nil {
				return reflect.Value{}, err
			}
			if val.IsValid() {
				m[keyStr.Value] = val.Interface()
			} else {
				m[keyStr.Value] = nil
			}
		}
		return reflect.ValueOf(m), nil
	default:
		return reflect.Value{}, newTypeError(obj, reflect.TypeOf((*interface{})(nil)).Elem())
	}
}

// ============================================================
// Error helpers
// ============================================================

// newTypeError creates a TypeError for incompatible conversion.
func newTypeError(obj object.Object, targetType reflect.Type) *object.ExceptionObject {
	return object.NewException(
		object.TypeTypeError,
		fmt.Sprintf("cannot convert Python %s to Go %s", obj.Type().Name(), targetType),
	)
}

// newOverflowError creates an OverflowError for out-of-range integer conversion.
func newOverflowError(val *big.Int, targetType reflect.Type) *object.ExceptionObject {
	return object.NewException(
		object.TypeOverflowError,
		fmt.Sprintf("Python int %s overflows Go %s", val.String(), targetType),
	)
}
