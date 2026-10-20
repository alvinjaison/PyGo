package object

// Invoker calls a Python callable with positional arguments. The runtime sets
// this hook at startup so that any package (stdlib, interop) can invoke a
// callable — including user-defined functions that require the VM — without
// importing the VM directly (which would create an import cycle).
type Invoker func(callable Object, args []Object) (Object, error)

// invokeHook is the process-wide invocation hook.
var invokeHook Invoker

// SetInvoker installs the global callable invocation hook. It is called once by
// the runtime during initialization.
func SetInvoker(inv Invoker) { invokeHook = inv }

// Invoke calls a callable with the given arguments, using the installed hook
// when available (required for user-defined functions) and falling back to the
// Callable interface for builtins.
func Invoke(callable Object, args []Object) (Object, error) {
	if invokeHook != nil {
		return invokeHook(callable, args)
	}
	if c, ok := callable.(Callable); ok {
		return c.Call(args, nil)
	}
	return nil, NewTypeError("'%s' object is not callable", callable.Type().Name())
}
