# Decorator chains: evaluation order, application order, parameterized
# decorators, and interaction with closures and *args/**kwargs.

calls = []


def tag(label):
    def deco(fn):
        calls.append("build:" + label)

        def wrapper(*args, **kwargs):
            return label + "(" + fn(*args, **kwargs) + ")"
        return wrapper
    return deco


def plain(label):
    def deco(fn):
        calls.append("build:" + label)

        def wrapper(*args, **kwargs):
            return label + "[" + fn(*args, **kwargs) + "]"
        return wrapper
    return deco


# Three stacked decorators. The expressions tag("a"), tag("b"), plain("c") are
# evaluated top to bottom, then applied bottom to top, so "c" wraps the raw
# function and "a" ends up outermost.
@tag("a")
@tag("b")
@plain("c")
def core():
    return "core"


print(calls)
print(core())

# A single decorator that changes the return value entirely.
def constant(fn):
    return "replaced"


@constant
def gone():
    return "never"


print(gone)


# Decorators compose with default, variadic and keyword arguments.
def audit(fn):
    def wrapper(*args, **kwargs):
        keys = []
        for k in kwargs:
            keys.append(k)
        return (fn(*args, **kwargs), len(args), keys)
    return wrapper


@audit
def totals(a, b=2, *rest, **opts):
    total = a + b
    for r in rest:
        total = total + r
    return total


print(totals(1))
print(totals(1, 5))
print(totals(1, 2, 3, 4))
print(totals(1, 2, 3, flag=True, mode="x"))


# A decorator built inside a function closes over that function's locals.
def multiplier(k):
    def deco(fn):
        def wrapper(x):
            return fn(x) * k
        return wrapper
    return deco


def make(k):
    @multiplier(k)
    def inner(x):
        return x + 1
    return inner


print(make(2)(3))
print(make(10)(3))


# An identity decorator must leave behaviour untouched.
def identity(fn):
    return fn


@identity
def square(x):
    return x * x


print(square(9))


# Decorated functions remain ordinary values.
@multiplier(3)
def triple(x):
    return x


holder = [triple]
print(holder[0](7))
