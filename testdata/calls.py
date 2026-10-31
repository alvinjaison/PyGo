# Calls, keyword binding and unpacking. Expected output (calls.out) is CPython 3.
def f(a, b=2, *, c=3):
    return (a, b, c)
print(f(1), f(1, 5), f(a=1, b=9), f(1, c=7), f(b=4, a=0), f(1, 2, c=0))
def g(*args, **kw):
    return (args, kw)
print(g(), g(1, 2), g(x=1, y=2), g(1, k=3))
def h(a, *rest, key=None, **opts):
    return (a, rest, key, opts)
print(h(1), h(1, 2, 3, key=5), h(1, z=1), h(a=2))
def e(a, b=1, *c):
    return (a, b, c)
print(e(1), e(1, 2, 3, 4))
t = (1, 2)
d = {"b": 5}
print(f(*t), f(1, **d), f(*[7], **{"c": 9}), g(*t, **{"q": 1}), max(*t), len(*[[1, 2]]))
print([*t, 3, *"ab"], (*t, 9), {*t, 7} == {1, 2, 7}, {**d, "z": 1, **{"b": 6}})
print(print(*t, sep="-") is None)
for bad in (lambda: f(), lambda: f(1, 2, 3), lambda: f(1, a=2), lambda: f(1, x=2), lambda: f(c=1)):
    try:
        bad()
        print("no error")
    except TypeError as ex:
        print("TypeError:", ex)
# Messages for these mention the callee in CPython; compare only the type.
for bad in (lambda: g(*5), lambda: f(**{1: 2}), lambda: f(1, **{"b": 1}, **{"b": 2})):
    try:
        bad()
        print("no error")
    except TypeError as ex:
        print("TypeError")
