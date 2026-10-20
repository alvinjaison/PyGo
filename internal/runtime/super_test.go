package runtime

import (
	"bytes"
	"context"
	"testing"
)

func superExpect(t *testing.T, src, want string) {
	t.Helper()
	var buf bytes.Buffer
	rt := New(Config{Stdout: &buf})
	if err := rt.RunString(context.Background(), src, "<super-test>"); err != nil {
		t.Fatalf("error: %v\nsource:\n%s", err, src)
	}
	if got := buf.String(); got != want {
		t.Errorf("source:\n%s\n got: %q\nwant: %q", src, got, want)
	}
}

func TestSuperInit(t *testing.T) {
	src := `class Animal:
    def __init__(self, name):
        self.name = name

class Dog(Animal):
    def __init__(self, name, breed):
        super().__init__(name)
        self.breed = breed

d = Dog("Rex", "Lab")
print(d.name)
print(d.breed)
`
	superExpect(t, src, "Rex\nLab\n")
}

func TestSuperMethod(t *testing.T) {
	src := `class Base:
    def greet(self):
        return "Hello"

class Child(Base):
    def greet(self):
        return super().greet() + ", world"

print(Child().greet())
`
	superExpect(t, src, "Hello, world\n")
}

func TestSuperThreeLevels(t *testing.T) {
	src := `class A:
    def who(self):
        return "A"

class B(A):
    def who(self):
        return "B->" + super().who()

class C(B):
    def who(self):
        return "C->" + super().who()

print(C().who())
`
	superExpect(t, src, "C->B->A\n")
}

func TestSuperExplicit(t *testing.T) {
	src := `class Base:
    def value(self):
        return 10

class Derived(Base):
    def value(self):
        return super(Derived, self).value() * 2

print(Derived().value())
`
	superExpect(t, src, "20\n")
}

func TestSuperDiamond(t *testing.T) {
	// Cooperative multiple inheritance via the MRO.
	src := `class A:
    def name(self):
        return "A"

class B(A):
    def name(self):
        return "B" + super().name()

class C(A):
    def name(self):
        return "C" + super().name()

class D(B, C):
    def name(self):
        return "D" + super().name()

# MRO of D is D, B, C, A → "DBCA"
print(D().name())
`
	superExpect(t, src, "DBCA\n")
}
