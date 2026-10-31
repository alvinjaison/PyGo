# Sorting, min/max with key/default. Expected output (sorting.out) is CPython 3.
data = [5, 3, 9, 1, 3, 7, -2, 0, 5]
print(sorted(data), sorted(data, reverse=True), sorted(data, key=lambda x: -x))
words = ["pear", "fig", "apple", "kiwi", "date", "plum", "banana"]
print(sorted(words), sorted(words, key=len), sorted(words, key=len, reverse=True))
# Stability, with and without reverse.
pairs = [("a", 2), ("b", 1), ("c", 2), ("d", 1), ("e", 2)]
print(sorted(pairs, key=lambda p: p[1]), sorted(pairs, key=lambda p: p[1], reverse=True))
xs = list(data)
print(xs.sort(), xs)
xs.sort(reverse=True)
print(xs)
xs.sort(key=lambda v: (v % 3, v))
print(xs)
print(sorted("hello"), sorted((3, 1, 2)), sorted({3, 1, 2}), sorted({"b": 1, "a": 2}))
print(sorted([2.5, 1, 3.5, 0]), sorted(["b", "A", "a", "B"]), sorted([]))
print(sorted(range(10, 0, -3)), sorted([[2, 1], [1, 5], [1, 2]]))
print(min(data), max(data), min(data, key=lambda x: abs(x - 4)), max(words, key=len))
print(min(3, 1, 2), max(3, 1, 2), min("b", "a"), max([1, 5], [2]))
print(min([], default=-1), max([], default="none"), min([4, 2], default=0))
big = [(i * 7919) % 1009 for i in range(3000)]
s = sorted(big)
print(s[:5], s[-5:], len(s), s == sorted(s), sorted(big, reverse=True)[:3])
for bad in (lambda: min([]), lambda: max(), lambda: sorted([1, "a"]), lambda: sorted(1)):
    try:
        bad()
        print("no error")
    except Exception as e:
        print(type(e).__name__)
