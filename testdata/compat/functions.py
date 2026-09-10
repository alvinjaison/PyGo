# Functions: def, call, recursion, passing functions as arguments

# Basic function definition and call
def greet():
    print("hello")

greet()

# Function with arguments
def add(a, b):
    return a + b

print(add(3, 4))

# Function with multiple params
def power(base, exp):
    result = 1
    i = 0
    while i < exp:
        result = result * base
        i = i + 1
    return result

print(power(3, 2))
print(power(2, 10))

# Recursion
def factorial(n):
    if n <= 1:
        return 1
    return n * factorial(n - 1)

print(factorial(5))
print(factorial(1))
print(factorial(10))

# Nested function calls
def double(x):
    return x * 2

def triple(x):
    return x * 3

print(double(triple(4)))

# Function as argument
def apply(fn, val):
    return fn(val)

print(apply(double, 5))

# Early return
def find_first_negative(lst):
    for x in lst:
        if x < 0:
            return x
    return 0

print(find_first_negative([1, 2, -3, 4]))
print(find_first_negative([1, 2, 3]))
