package runtime

import (
	"bytes"
	"context"
	"testing"
)

func collExpect(t *testing.T, src, want string) {
	t.Helper()
	var buf bytes.Buffer
	rt := New(Config{Stdout: &buf})
	if err := rt.RunString(context.Background(), src, "<coll-test>"); err != nil {
		t.Fatalf("error: %v\nsource:\n%s", err, src)
	}
	if got := buf.String(); got != want {
		t.Errorf("source:\n%s\n got: %q\nwant: %q", src, got, want)
	}
}

func TestCounter(t *testing.T) {
	src := `from collections import Counter
c = Counter("abracadabra")
print(c["a"])
print(c["b"])
print(c["z"])
`
	collExpect(t, src, "5\n2\n0\n")
}

func TestCounterMostCommon(t *testing.T) {
	src := `from collections import Counter
c = Counter([1, 1, 1, 2, 2, 3])
print(c.most_common(2))
`
	collExpect(t, src, "[(1, 3), (2, 2)]\n")
}

func TestCounterFromList(t *testing.T) {
	src := `from collections import Counter
words = ["apple", "banana", "apple", "cherry", "banana", "apple"]
c = Counter(words)
print(c["apple"], c["banana"], c["cherry"])
`
	collExpect(t, src, "3 2 1\n")
}

func TestDefaultDict(t *testing.T) {
	src := `from collections import defaultdict
d = defaultdict(list)
d["a"].append(1)
d["a"].append(2)
d["b"].append(3)
print(d["a"])
print(d["b"])
`
	collExpect(t, src, "[1, 2]\n[3]\n")
}

func TestDefaultDictInt(t *testing.T) {
	src := `from collections import defaultdict
counts = defaultdict(int)
for ch in "hello":
    counts[ch] = counts[ch] + 1
print(counts["l"])
print(counts["o"])
`
	collExpect(t, src, "2\n1\n")
}

func TestOrderedDict(t *testing.T) {
	src := `from collections import OrderedDict
d = OrderedDict()
d["z"] = 1
d["a"] = 2
d["m"] = 3
print(list(d.keys()))
print(list(d.values()))
`
	collExpect(t, src, "['z', 'a', 'm']\n[1, 2, 3]\n")
}

func TestDeque(t *testing.T) {
	src := `from collections import deque
d = deque([1, 2, 3])
d.append(4)
d.appendleft(0)
print(list(d))
print(d.pop())
print(d.popleft())
print(list(d))
`
	collExpect(t, src, "[0, 1, 2, 3, 4]\n4\n0\n[1, 2, 3]\n")
}

func TestNamedTuple(t *testing.T) {
	src := `from collections import namedtuple
Point = namedtuple("Point", "x y")
p = Point(3, 4)
print(p.x)
print(p.y)
print(p[0], p[1])
print(p)
`
	collExpect(t, src, "3\n4\n3 4\nPoint(x=3, y=4)\n")
}

func TestNamedTupleFields(t *testing.T) {
	src := `from collections import namedtuple
Color = namedtuple("Color", ["r", "g", "b"])
c = Color(255, 128, 0)
print(c._fields)
print(sum(c))
`
	collExpect(t, src, "('r', 'g', 'b')\n383\n")
}

func TestNamedTupleUnpack(t *testing.T) {
	src := `from collections import namedtuple
Pair = namedtuple("Pair", "first second")
p = Pair(10, 20)
a, b = p
print(a + b)
`
	collExpect(t, src, "30\n")
}

func TestCollectionsImport(t *testing.T) {
	// `import collections` then qualified access.
	src := `import collections
c = collections.Counter("mississippi")
print(c["s"], c["i"], c["p"])
`
	collExpect(t, src, "4 4 2\n")
}

func TestListMethods(t *testing.T) {
	collExpect(t, "xs = [3, 1, 2]\nxs.append(4)\nprint(xs)\n", "[3, 1, 2, 4]\n")
	collExpect(t, "xs = [1, 2, 3]\nprint(xs.pop())\nprint(xs)\n", "3\n[1, 2]\n")
	collExpect(t, "xs = [1, 2, 3]\nxs.insert(1, 99)\nprint(xs)\n", "[1, 99, 2, 3]\n")
	collExpect(t, "xs = [1, 2, 3, 2]\nxs.remove(2)\nprint(xs)\n", "[1, 3, 2]\n")
	collExpect(t, "xs = [1, 2, 3]\nxs.extend([4, 5])\nprint(xs)\n", "[1, 2, 3, 4, 5]\n")
	collExpect(t, "xs = [3, 1, 2]\nxs.sort()\nprint(xs)\n", "[1, 2, 3]\n")
	collExpect(t, "xs = [1, 2, 3]\nxs.reverse()\nprint(xs)\n", "[3, 2, 1]\n")
	collExpect(t, "xs = [1, 2, 2, 3, 2]\nprint(xs.count(2))\nprint(xs.index(3))\n", "3\n3\n")
}

func TestTupleSubscript(t *testing.T) {
	collExpect(t, "t = (10, 20, 30)\nprint(t[0], t[-1])\n", "10 30\n")
}
