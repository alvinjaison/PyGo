# Arithmetic operations: basic math, floor division, modulo, power, mixed int/float
# Tests: addition, subtraction, multiplication, division, floor division,
#         modulo, power, unary minus, operator precedence, mixed int/float

# Basic integer arithmetic
print(2 + 3)
print(10 - 4)
print(3 * 7)
print(10 + 2 * 3)
print((10 + 2) * 3)

# Floor division
print(7 // 2)
print(-7 // 2)
print(10 // 3)
print(1 // 1)

# Modulo
print(7 % 3)
print(10 % 5)
print(-7 % 3)

# Power
print(2 ** 3)
print(2 ** 10)
print(3 ** 0)
print(10 ** 2)

# Unary minus
print(-5)
print(-(-3))

# Mixed int/float
# Note: PyGo uses Go's %g format for floats, so whole-number floats display
# without trailing .0 (e.g., 6.0 displays as "6"). This is a documented
# divergence from CPython where repr(6.0) == "6.0".
print(1 + 0.5)
print(2 * 3.0)
print(10 / 3)
print(7.0 // 2)

# Augmented assignments
x = 10
x += 5
print(x)
x -= 3
print(x)
x *= 2
print(x)
x //= 5
print(x)
x %= 3
print(x)
y = 2
y **= 8
print(y)
