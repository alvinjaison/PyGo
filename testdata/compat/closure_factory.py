# Closure factory patterns: nested functions capturing enclosing variables

# 1. Basic closure: capture a single variable
def make_greeter(name):
    def greet():
        return "hello " + name
    return greet

g = make_greeter("world")
print(g())

# 2. Closure factory: multiple closures from same factory
def make_multiplier(factor):
    def multiply(x):
        return x * factor
    return multiply

double = make_multiplier(2)
triple = make_multiplier(3)
print(double(5))
print(triple(5))
print(double(7))

# 3. Nonlocal modification: counter pattern
def make_counter():
    count = 0
    def increment():
        nonlocal count
        count = count + 1
        return count
    return increment

c = make_counter()
print(c())
print(c())
print(c())

# 4. Multiple closures sharing a cell variable
def make_pair():
    value = 0
    def getter():
        return value
    def setter(v):
        nonlocal value
        value = v
    return [getter, setter]

pair = make_pair()
get = pair[0]
set_fn = pair[1]
print(get())
set_fn(42)
print(get())
set_fn(99)
print(get())

# 5. Independent instances from same factory
c1 = make_counter()
c2 = make_counter()
print(c1())
print(c1())
print(c2())
print(c1())
print(c2())

# 6. Triple-nested closure (read-only capture through intermediate scope)
def level1():
    x = 100
    def level2():
        def level3():
            return x
        return level3
    return level2

f = level1()()
print(f())

# 7. Nonlocal modification (one level deep)
def modifier():
    value = 0
    def inc():
        nonlocal value
        value = value + 1
    inc()
    inc()
    return value

print(modifier())

# 8. Closure preserving state after enclosing function returns
def make_accumulator(initial):
    total = initial
    def add(n):
        nonlocal total
        total = total + n
        return total
    return add

acc = make_accumulator(100)
print(acc(10))
print(acc(20))
print(acc(30))

# 9. LEGB resolution: enclosing shadows global
x = "global"

def test_legb():
    x = "enclosing"
    def inner():
        print(x)
    inner()

test_legb()
print(x)

# 10. Nonlocal binds to nearest enclosing scope
def nearest_test():
    x = "outer"
    def middle():
        x = "middle"
        def inner():
            nonlocal x
            x = "changed"
        inner()
        print(x)
    middle()
    print(x)

nearest_test()
