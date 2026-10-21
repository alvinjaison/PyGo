// Command embedding demonstrates the most basic use of PyGo: running Python
// source from Go and exchanging values across the boundary.
//
// Run it with:
//
//	go run ./examples/embedding
package main

import (
	"fmt"
	"log"

	"github.com/alvinjaison/PyGo/pygo"
)

func main() {
	rt := pygo.New()

	// Execute a Python program.
	if err := rt.RunString(`
numbers = [1, 2, 3, 4, 5]
total = sum(numbers)
average = total / len(numbers)
`); err != nil {
		log.Fatal(err)
	}

	// Read results back into Go.
	total, _ := rt.Get("total")
	avg, _ := rt.Get("average")

	t, _ := total.Int()
	a, _ := avg.Float()
	fmt.Printf("total=%d average=%.1f\n", t, a)

	// Evaluate a single expression.
	result, err := rt.Eval("[x * x for x in range(6)]")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("squares:", result.String())

	// Set a value from Go, then use it from Python.
	if err := rt.Set("config", map[string]interface{}{"debug": true, "port": 8080}); err != nil {
		log.Fatal(err)
	}
	if err := rt.RunString(`print(f"debug={config['debug']} port={config['port']}")`); err != nil {
		log.Fatal(err)
	}
}
