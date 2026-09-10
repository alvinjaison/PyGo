# Python 3 semantic correctness tests
# Tests: floor division, modulo, ZeroDivisionError, short-circuit evaluation,
#         LGB name lookup, truthiness, arbitrary-precision integers

# 1. Floor division rounds toward negative infinity
print(7 // 2)
print(-7 // 2)
print(7 // -2)
print(-7 // -2)

# 2. Modulo result sign matches divisor
print(7 % 3)
print(-7 % 3)
print(7 % -3)
print(-7 % -3)

# 3. ZeroDivisionError for // and %
try:
    x = 1 // 0
except ZeroDivisionError:
    print("caught //0")

try:
    x = 1 % 0
except ZeroDivisionError:
    print("caught %0")

# 4. Short-circuit: and/or return determining operand, not bool
print(1 and 2)
print(0 and 2)
print(1 or 2)
print(0 or 2)
print("" or "hello")
print("hi" and "bye")

# 5. LGB name lookup: local shadows global shadows builtin
x = "global"

def test_local():
    x = "local"
    print(x)

test_local()
print(x)
print(len("abc"))

# 6. Truthiness
print(bool(False))
print(bool(None))
print(bool(0))
print(bool(0.0))
print(bool(""))
print(bool([]))
print(bool(()))
print(bool({}))
print(bool(True))
print(bool(1))
print(bool(0.1))
print(bool("x"))
print(bool([0]))

# 7. Arbitrary-precision integers
print(2 ** 100)
print(10 ** 20 + 1)
