package runtime

import "testing"

func TestObjectAndClassBuiltins(t *testing.T) {
	matchExpect(t, `class A(object):
    def __init__(self, v):
        self.v = v
    def __call__(self):
        return self.v
a = A(3)
print(isinstance(a, object), issubclass(A, object), issubclass(int, object), issubclass(bool, int))
print(issubclass(A, (str, A)), issubclass(A, str))
print(callable(a), callable(len), callable(5), callable(A))
`, "True True True True\nTrue False\nTrue True False True\n")
}

func TestZipMapFilterAreLazy(t *testing.T) {
	matchExpect(t, `print(list(zip([1, 2, 3], "ab")), list(zip()))
print(list(map(lambda x, y: x + y, [1, 2], [10, 20])))
print(list(filter(None, [0, 1, "", 2])), list(filter(lambda x: x > 1, [1, 2, 3])))
def count():
    n = 0
    while True:
        yield n
        n += 1
z = zip(count(), "abc")
print(next(z), next(z), list(z))
print(list(map(str, filter(lambda n: n % 2, range(6)))))
`, "[(1, 'a'), (2, 'b')] []\n[11, 22]\n[1, 2] [2, 3]\n(0, 'a') (1, 'b') [(2, 'c')]\n['1', '3', '5']\n")
}

func TestNumericBuiltins(t *testing.T) {
	matchExpect(t, `print(divmod(7, 2), divmod(-7, 2), divmod(7.5, 2))
print(pow(2, 10), pow(2, 10, 1000), pow(3, 4, -7), pow(2, -1))
print(bin(5), bin(-5), oct(8), hex(255), hex(-255))
`, "(3, 1) (-4, 1) (3.0, 1.5)\n1024 24 -3 0.5\n0b101 -0b101 0o10 0xff -0xff\n")
}

func TestIdDelattrSlice(t *testing.T) {
	matchExpect(t, `class A:
    pass
a = A()
a.v = 1
b = A()
print(id(a) == id(a), id(a) != id(b))
delattr(a, "v")
print(hasattr(a, "v"))
l = [0, 1, 2, 3, 4]
print(l[slice(1, 4)], l[slice(None, None, 2)], l[slice(2)])
`, "True True\nFalse\n[1, 2, 3] [0, 2, 4] [0, 1]\n")
}

func TestSetComparison(t *testing.T) {
	matchExpect(t, `print({1, 2} == {2, 1}, {1} != {1, 2}, {1} < {1, 2}, {1, 2} <= {1, 2})
print({1, 2, 3} > {1}, {1} > {1}, set() == set(), {1} == [1], {1, 2} == {1, 3})
`, "True True True True\nTrue False True False False\n")
}

func TestFloatReprFollowsCPython(t *testing.T) {
	matchExpect(t, `print(3.0, 1e10, 1.5, 1e16, 2.0 ** 70, 0.1 + 0.2, float(2), 1e-5, 0.0001, -0.0)
print([3.0], str(3.0), repr(2.5), 3.0 * 2, 7.5 // 2)
`, "3.0 10000000000.0 1.5 1e+16 1.1805916207174113e+21 0.30000000000000004 2.0 1e-05 0.0001 -0.0\n[3.0] 3.0 2.5 6.0 3.0\n")
}
