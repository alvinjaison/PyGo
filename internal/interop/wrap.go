package interop

import (
	"fmt"
	"reflect"

	"github.com/alvinjaison/pygo/internal/object"
)

// errorInterface is the reflect.Type for the Go error interface.
var errorInterface = reflect.TypeOf((*error)(nil)).Elem()

// WrapGoFunction wraps an arbitrary Go function as a Python-callable BuiltinFuncObject.
// The function must be a valid Go function; otherwise an error is returned.
//
// The generated adapter:
//   - Validates argument count (raises TypeError on mismatch)
//   - Converts Python arguments to Go parameter types via PythonToGo
//   - Calls the Go function via reflection
//   - Converts Go return values back to Python objects via GoToPython
//   - Handles (value, error) return pattern: non-nil error raises a Python exception
func WrapGoFunction(name string, fn interface{}) (*object.BuiltinFuncObject, error) {
	if fn == nil {
		return nil, fmt.Errorf("WrapGoFunction: fn must not be nil")
	}

	fnType := reflect.TypeOf(fn)
	if fnType.Kind() != reflect.Func {
		return nil, fmt.Errorf("WrapGoFunction: expected a function, got %s", fnType.Kind())
	}

	fnValue := reflect.ValueOf(fn)
	numIn := fnType.NumIn()
	numOut := fnType.NumOut()

	// Determine return pattern.
	returnPattern := classifyReturns(fnType, numOut)

	adapter := func(args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		// Check argument count.
		if len(args) != numIn {
			return nil, object.NewTypeError(
				"%s() takes %d argument(s) but %d were given",
				name, numIn, len(args),
			)
		}

		// Convert Python args → Go params.
		goArgs := make([]reflect.Value, numIn)
		for i := 0; i < numIn; i++ {
			paramType := fnType.In(i)
			converted, err := PythonToGo(args[i], paramType)
			if err != nil {
				return nil, object.NewTypeError(
					"%s() argument %d: %s", name, i+1, err.Error(),
				)
			}
			goArgs[i] = converted
		}

		// Call the Go function.
		results := fnValue.Call(goArgs)

		// Convert Go returns → Python objects.
		return handleReturns(results, returnPattern)
	}

	return object.NewBuiltin(name, adapter), nil
}

// returnKind classifies the return signature of a wrapped Go function.
type returnKind int

const (
	returnNone       returnKind = iota // no return values
	returnValue                        // one non-error return
	returnError                        // one error return
	returnValueError                   // (value, error) pattern
)

// classifyReturns determines how to handle the function's return values.
func classifyReturns(fnType reflect.Type, numOut int) returnKind {
	switch numOut {
	case 0:
		return returnNone
	case 1:
		if fnType.Out(0).Implements(errorInterface) {
			return returnError
		}
		return returnValue
	case 2:
		if fnType.Out(1).Implements(errorInterface) {
			return returnValueError
		}
		// Treat as single value (first return), ignoring second.
		return returnValue
	default:
		// For 3+ returns, just convert the first non-error return.
		return returnValue
	}
}

// handleReturns converts reflected return values to a Python object based on the return pattern.
func handleReturns(results []reflect.Value, pattern returnKind) (object.Object, error) {
	switch pattern {
	case returnNone:
		return object.None, nil

	case returnValue:
		val := results[0].Interface()
		pyObj, err := GoToPython(val)
		if err != nil {
			return nil, object.NewTypeError("cannot convert return value: %s", err.Error())
		}
		return pyObj, nil

	case returnError:
		errVal := results[0].Interface()
		if errVal != nil {
			return nil, object.NewException(
				object.TypeRuntimeError,
				errVal.(error).Error(),
			)
		}
		return object.None, nil

	case returnValueError:
		errVal := results[1].Interface()
		if errVal != nil {
			return nil, object.NewException(
				object.TypeRuntimeError,
				errVal.(error).Error(),
			)
		}
		val := results[0].Interface()
		pyObj, err := GoToPython(val)
		if err != nil {
			return nil, object.NewTypeError("cannot convert return value: %s", err.Error())
		}
		return pyObj, nil

	default:
		return object.None, nil
	}
}
