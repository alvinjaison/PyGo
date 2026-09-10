# Exceptions: try/except/finally, raise, multiple handlers

# Basic try/except
try:
    x = 1 / 0
except ZeroDivisionError:
    print("caught zero division")

# Try/except with as
try:
    raise ValueError("custom message")
except ValueError as e:
    print(e)

# Multiple except handlers
def test_handler(x):
    try:
        if x == 1:
            result = 1 / 0
        elif x == 2:
            result = int("abc")
        elif x == 3:
            result = [][5]
    except ZeroDivisionError:
        print("zero")
    except ValueError:
        print("value")
    except IndexError:
        print("index")

test_handler(1)
test_handler(2)
test_handler(3)

# Try/finally (no exception)
try:
    print("try block")
finally:
    print("finally block")

# Try/except/finally with exception
try:
    raise TypeError("type error")
except TypeError:
    print("caught type error")
finally:
    print("cleanup done")

# Exception propagation through function calls
def deep():
    raise RuntimeError("from deep")

def middle():
    deep()

try:
    middle()
except RuntimeError as e:
    print(e)

# Bare raise (re-raise) in function
def inner():
    try:
        raise ValueError("from inner")
    except ValueError:
        print("caught in inner")
        raise

try:
    inner()
except ValueError as e:
    print(e)

# Finally always runs even with unhandled exception caught at outer level
def risky():
    try:
        raise TypeError("unhandled")
    finally:
        print("finally ran")

try:
    risky()
except TypeError:
    print("outer caught")
