# Builtins: len, range, int, float, str, bool, type, isinstance, repr, abs, min, max

# len
print(len([1, 2, 3]))
print(len("hello"))
print(len((1, 2)))
print(len({}))

# range (used in for loop)
result = []
for i in range(5):
    result.append(i)
print(result)

result = []
for i in range(2, 7):
    result.append(i)
print(result)

result = []
for i in range(0, 10, 3):
    result.append(i)
print(result)

# int conversion
print(int(3.7))
print(int("42"))
print(int(True))
print(int(False))

# float conversion
# Note: PyGo uses Go's %g format for floats. Whole-number floats display
# without trailing .0 (e.g., float(5) prints "5" not "5.0").
# This is a documented divergence from CPython.
print(float(5))
print(float("3.14"))
print(float(0))

# str conversion
print(str(123))
print(str(True))
print(str(None))

# bool conversion
print(bool(1))
print(bool(0))
print(bool(""))
print(bool("hi"))
print(bool([]))
print(bool([1]))

# type
print(type(42))
print(type("hi"))
print(type(True))
print(type(None))
print(type([1, 2]))

# isinstance
print(isinstance(42, int))
print(isinstance("hi", str))
print(isinstance(42, str))
print(isinstance(True, bool))

# repr
print(repr("hello"))
print(repr(42))
print(repr(None))
print(repr([1, 2, 3]))

# abs
print(abs(-5))
print(abs(3))
print(abs(-2.5))

# min and max
print(min(3, 1, 2))
print(max(3, 1, 2))
print(min([5, 2, 8]))
print(max([5, 2, 8]))
