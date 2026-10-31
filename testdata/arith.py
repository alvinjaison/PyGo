# Differential arithmetic test. The expected output (arith.out) was produced
# by CPython 3; PyGo must match it byte for byte.
vals = [0, 1, -1, 3, -7, 256, 257, -6, 2**31, 2**53 + 1, -2**53 - 1,
        2**62, 2**63 - 1, -2**63, 2**63, -2**63 - 1, 10**30, -10**30]
for a in vals:
    for b in vals:
        print(a, b, a + b, a - b, a * b, a < b, a <= b, a == b, a != b, a > b, a >= b, a & b, a | b, a ^ b)
        if b != 0:
            print(a // b, a % b, a / b, divmod(a, b))
fl = [0.0, 1.5, -1.5, 1e10, 0.1, 1e300]
for a in vals:
    for f in fl:
        print(a + f, a - f, a * f, f + a, f - a, f * a, a < f, a == f, a >= f, f < a, f == a)
        if f != 0.0:
            print(a / f)
        if a != 0:
            print(f / a)
for f in fl:
    for g in fl:
        print(f + g, f - g, f * g, f < g, f == g)
        if g != 0.0:
            print(f / g)
print(hash(2**63), hash(2**63 - 1), hash(-2**63), 2**63 - 1 + 1 == 2**63)
d = {}
d[2**63] = "a"
d[2**63 - 1 + 1] = "b"
d[5] = "c"
d[5 + 0] = "d"
d[2**62 * 2 - 1 + 1] = "e"
print(len(d), d[2**63], d[5])
print(-(2**63), -(-2**63), abs(-2**63), (-2**63) // -1, (-2**63) % -1, (-2**63) * -1)
print("a" < "b", "b" < "a", "a" == "a", "abc" >= "abd", "é" < "z")
