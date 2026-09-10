package main

import (
	"fmt"
	"os"

	pygo "github.com/alvinjaison/pygo/pygo"
)

type Calculator struct{}

func (c *Calculator) Add(a, b int) int {
	return a + b
}

func (c *Calculator) Multiply(a, b int) int {
	return a * b
}

func main() {
	runtime := pygo.New()

	calculator := &Calculator{}

	if err := runtime.RegisterFunction("calc_add", calculator.Add); err != nil {
		fmt.Fprintln(os.Stderr, "register calc_add:", err)
		os.Exit(1)
	}

	if err := runtime.RegisterFunction("calc_multiply", calculator.Multiply); err != nil {
		fmt.Fprintln(os.Stderr, "register calc_multiply:", err)
		os.Exit(1)
	}

	err := runtime.RunString(`
print(calc_add(10, 20))
print(calc_multiply(5, 6))
`)
	if err != nil {
		fmt.Fprintln(os.Stderr, "execute:", err)
		os.Exit(1)
	}
}