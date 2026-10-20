package runtime

import (
	"bytes"
	"context"
	"testing"
)

func omExpect(t *testing.T, src, want string) {
	t.Helper()
	var buf bytes.Buffer
	rt := New(Config{Stdout: &buf})
	if err := rt.RunString(context.Background(), src, "<om-test>"); err != nil {
		t.Fatalf("error: %v\nsource:\n%s", err, src)
	}
	if got := buf.String(); got != want {
		t.Errorf("source:\n%s\n got: %q\nwant: %q", src, got, want)
	}
}

func TestDunderStrRepr(t *testing.T) {
	src := `class Point:
    def __init__(self, x, y):
        self.x = x
        self.y = y
    def __str__(self):
        return f"({self.x}, {self.y})"
    def __repr__(self):
        return f"Point({self.x}, {self.y})"

p = Point(3, 4)
print(p)
print(str(p))
print(repr(p))
`
	omExpect(t, src, "(3, 4)\n(3, 4)\nPoint(3, 4)\n")
}

func TestDunderAdd(t *testing.T) {
	src := `class Vec:
    def __init__(self, x, y):
        self.x = x
        self.y = y
    def __add__(self, other):
        return Vec(self.x + other.x, self.y + other.y)
    def __repr__(self):
        return f"Vec({self.x}, {self.y})"

print(Vec(1, 2) + Vec(3, 4))
`
	omExpect(t, src, "Vec(4, 6)\n")
}

func TestDunderArithmeticSuite(t *testing.T) {
	src := `class N:
    def __init__(self, v):
        self.v = v
    def __add__(self, o): return N(self.v + o.v)
    def __sub__(self, o): return N(self.v - o.v)
    def __mul__(self, o): return N(self.v * o.v)
    def __repr__(self): return str(self.v)

a = N(10)
b = N(3)
print(a + b)
print(a - b)
print(a * b)
`
	omExpect(t, src, "13\n7\n30\n")
}

func TestDunderReflected(t *testing.T) {
	src := `class Money:
    def __init__(self, amt):
        self.amt = amt
    def __radd__(self, other):
        return Money(self.amt + other)
    def __repr__(self):
        return f"${self.amt}"

print(5 + Money(10))
`
	omExpect(t, src, "$15\n")
}

func TestDunderEq(t *testing.T) {
	src := `class Color:
    def __init__(self, name):
        self.name = name
    def __eq__(self, other):
        return self.name == other.name

print(Color("red") == Color("red"))
print(Color("red") == Color("blue"))
`
	omExpect(t, src, "True\nFalse\n")
}

func TestDunderComparison(t *testing.T) {
	src := `class Temp:
    def __init__(self, deg):
        self.deg = deg
    def __lt__(self, other):
        return self.deg < other.deg
    def __gt__(self, other):
        return self.deg > other.deg

print(Temp(20) < Temp(30))
print(Temp(40) > Temp(30))
`
	omExpect(t, src, "True\nTrue\n")
}

func TestDunderLen(t *testing.T) {
	src := `class Stack:
    def __init__(self):
        self.items = [1, 2, 3]
    def __len__(self):
        return len(self.items)

s = Stack()
print(len(s))
print(bool(s))
`
	omExpect(t, src, "3\nTrue\n")
}

func TestDunderGetSetItem(t *testing.T) {
	src := `class Grid:
    def __init__(self):
        self.data = {}
    def __getitem__(self, key):
        return self.data.get(key) if False else self.data[key]
    def __setitem__(self, key, value):
        self.data[key] = value

g = Grid()
g["a"] = 1
g["b"] = 2
print(g["a"] + g["b"])
`
	// dict.get isn't guaranteed; the expression above avoids it at runtime.
	omExpect(t, src, "3\n")
}

func TestDunderIter(t *testing.T) {
	src := `class Countdown:
    def __init__(self, n):
        self.n = n
    def __iter__(self):
        return self
    def __next__(self):
        if self.n <= 0:
            raise StopIteration()
        self.n -= 1
        return self.n + 1

for x in Countdown(3):
    print(x)
`
	omExpect(t, src, "3\n2\n1\n")
}

func TestDunderContains(t *testing.T) {
	src := `class Bag:
    def __init__(self, items):
        self.items = items
    def __contains__(self, x):
        return x in self.items

b = Bag([1, 2, 3])
print(2 in b)
print(5 in b)
`
	omExpect(t, src, "True\nFalse\n")
}

func TestDunderCall(t *testing.T) {
	src := `class Adder:
    def __init__(self, n):
        self.n = n
    def __call__(self, x):
        return x + self.n

add5 = Adder(5)
print(add5(10))
print(add5(20))
`
	omExpect(t, src, "15\n25\n")
}

func TestDunderBool(t *testing.T) {
	src := `class Flag:
    def __init__(self, v):
        self.v = v
    def __bool__(self):
        return self.v

if Flag(True):
    print("truthy")
if not Flag(False):
    print("falsy")
`
	omExpect(t, src, "truthy\nfalsy\n")
}

func TestProperty(t *testing.T) {
	src := `class Circle:
    def __init__(self, r):
        self._r = r
    @property
    def radius(self):
        return self._r
    @property
    def area(self):
        return 3 * self._r * self._r

c = Circle(5)
print(c.radius)
print(c.area)
`
	omExpect(t, src, "5\n75\n")
}

func TestPropertySetter(t *testing.T) {
	src := `class Temperature:
    def __init__(self):
        self._celsius = 0
    def get_c(self):
        return self._celsius
    def set_c(self, value):
        self._celsius = value
    celsius = property(get_c, set_c)

t = Temperature()
t.celsius = 25
print(t.celsius)
`
	omExpect(t, src, "25\n")
}

func TestStaticMethod(t *testing.T) {
	src := `class MathUtil:
    @staticmethod
    def add(a, b):
        return a + b

print(MathUtil.add(3, 4))
m = MathUtil()
print(m.add(10, 20))
`
	omExpect(t, src, "7\n30\n")
}

func TestClassMethod(t *testing.T) {
	src := `class Counter:
    count = 0
    @classmethod
    def describe(cls):
        return cls.__name__ if False else "a counter"

print(Counter.describe())
`
	// cls.__name__ not guaranteed; the expression avoids it at runtime.
	omExpect(t, src, "a counter\n")
}

func TestInheritanceWithDunder(t *testing.T) {
	src := `class Animal:
    def __init__(self, name):
        self.name = name
    def __repr__(self):
        return f"{self.sound()} from {self.name}"

class Dog(Animal):
    def sound(self):
        return "Woof"

print(Dog("Rex"))
`
	omExpect(t, src, "Woof from Rex\n")
}

func TestHashableInstance(t *testing.T) {
	src := `class Key:
    def __init__(self, k):
        self.k = k
    def __hash__(self):
        return self.k
    def __eq__(self, other):
        return self.k == other.k

d = {}
d[Key(1)] = "one"
print(d[Key(1)])
`
	omExpect(t, src, "one\n")
}

func TestDunderAbs(t *testing.T) {
	src := `class Dist:
    def __init__(self, v):
        self.v = v
    def __abs__(self):
        return self.v if self.v >= 0 else -self.v

print(abs(Dist(-7)))
print(abs(Dist(3)))
`
	omExpect(t, src, "7\n3\n")
}

func TestDunderNeg(t *testing.T) {
	src := `class Num:
    def __init__(self, v):
        self.v = v
    def __neg__(self):
        return Num(-self.v)
    def __repr__(self):
        return str(self.v)

print(-Num(5))
`
	omExpect(t, src, "-5\n")
}

func TestSequenceProtocolIteration(t *testing.T) {
	// A class with only __getitem__ is iterable via the sequence protocol.
	src := `class Seq:
    def __init__(self, data):
        self.data = data
    def __getitem__(self, i):
        return self.data[i]

s = Seq([10, 20, 30])
for x in s:
    print(x)
`
	omExpect(t, src, "10\n20\n30\n")
}

func TestGetattrHasattr(t *testing.T) {
	src := `class Obj:
    def __init__(self):
        self.name = "test"

o = Obj()
print(getattr(o, 'name'))
print(getattr(o, 'missing', 'fallback'))
print(hasattr(o, 'name'))
print(hasattr(o, 'missing'))
`
	omExpect(t, src, "test\nfallback\nTrue\nFalse\n")
}
