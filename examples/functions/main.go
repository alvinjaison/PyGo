// Command functions demonstrates exposing Go functions to Python and calling
// Python functions back from Go.
//
// Run it with:
//
//	go run ./examples/functions
package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/alvinjaison/PyGo/pygo"
)

func main() {
	rt := pygo.New()

	// Expose Go functions to Python. Arguments and results are converted
	// automatically between Go and Python types.
	rt.MustRegisterFunction("shout", strings.ToUpper)
	rt.MustRegisterFunction("add", func(a, b int) int { return a + b })
	rt.MustRegisterFunction("divide", func(a, b float64) (float64, error) {
		if b == 0 {
			return 0, fmt.Errorf("cannot divide by zero")
		}
		return a / b, nil
	})

	// Call them from Python.
	if err := rt.RunString(`
print(shout("hello"))
print(add(6, 7))
print(divide(10, 4))
`); err != nil {
		log.Fatal(err)
	}

	// A Go error returned by a registered function becomes a Python exception.
	err := rt.RunString(`
try:
    divide(1, 0)
except Exception as e:
    print("caught:", e)
`)
	if err != nil {
		log.Fatal(err)
	}

	// Define a Python function and call it from Go.
	if err := rt.RunString(`
def greet(name, greeting="Hello"):
    return f"{greeting}, {name}!"
`); err != nil {
		log.Fatal(err)
	}
	result, err := rt.CallByName("greet", "world")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(result.String())
}
