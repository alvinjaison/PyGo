// Command objects demonstrates exposing Go structs to Python and defining
// Python classes that use operator overloading.
//
// Run it with:
//
//	go run ./examples/objects
package main

import (
	"log"

	"github.com/alvinjaison/PyGo/pygo"
)

// DeploymentService is a Go type whose exported fields and methods are made
// available to Python scripts.
type DeploymentService struct {
	Environment string
}

// Deploy is callable from Python as deployment.deploy(...).
func (d *DeploymentService) Deploy(app string) string {
	return "deployed " + app + " to " + d.Environment
}

func main() {
	rt := pygo.New()

	// Expose a Go struct instance. Fields and methods become Python
	// attributes (snake_case method names are also accepted).
	rt.MustRegister("deployment", &DeploymentService{Environment: "production"})

	if err := rt.RunString(`
print(deployment.Environment)
print(deployment.deploy("web-app"))
`); err != nil {
		log.Fatal(err)
	}

	// Define a Python class with operator overloading and a property.
	if err := rt.RunString(`
class Vector:
    def __init__(self, x, y):
        self.x = x
        self.y = y
    def __add__(self, other):
        return Vector(self.x + other.x, self.y + other.y)
    def __repr__(self):
        return f"Vector({self.x}, {self.y})"
    @property
    def magnitude(self):
        return (self.x ** 2 + self.y ** 2) ** 0.5

v = Vector(3, 4)
print(v)
print("sum:", v + Vector(1, 1))
print("magnitude:", v.magnitude)
`); err != nil {
		log.Fatal(err)
	}
}
