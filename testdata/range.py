# Differential range test; expected output (range.out) produced by CPython 3.
specs = [(5,), (0,), (-3,), (2, 8), (8, 2), (0, 10, 3), (10, 0, -3), (10, 0, -1),
         (0, 10, 5), (0, 11, 5), (-5, 5, 2), (5, -5, -2), (3, 3), (0, 2**40, 2**38)]
for s in specs:
    r = range(*s)
    print(s, r, len(r), list(r)[:6] if len(r) < 100 else "big", bool(r))
    if len(r):
        print(r[0], r[-1], r[len(r) // 2], r[1:], r[::-1][:3] if len(r) < 100 else "")
    print(3 in r, 4 in r, 2.0 in r, -3 in r, 10 in r, True in r, "a" in r)
    print(r == range(*s), r != range(*s), r == range(7), sum(r) if len(r) < 100 else 0)
r = range(2, 20, 3)
print(r.start, r.stop, r.step, list(reversed(r)), sorted(r, reverse=True), max(r), min(r))
print(list(r[1:4]), list(r[::2]), list(r[-2:]), r[::-1])
print(list(zip(range(3), range(10, 0, -4))), list(map(lambda x: x * x, range(4))))
print(len(range(10**15)), 10**15 - 1 in range(10**15), range(0, 10**18, 7)[-1])
for bad in (lambda: range(2.5), lambda: range(0), lambda: range(1, 2, 0), lambda: range(3)[5], lambda: range(3)["a"]):
    try:
        bad()
        print("no error")
    except Exception as e:
        print(type(e).__name__)
