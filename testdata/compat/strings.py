# Strings: concatenation, repetition, indexing, len, comparison, contains

# Concatenation
print("hello" + " " + "world")

# Repetition
print("ab" * 3)
print("x" * 0)
print("ha" * 1)

# Indexing
s = "python"
print(s[0])
print(s[5])
print(s[-1])
print(s[-6])

# Length
print(len(""))
print(len("hello"))
print(len("abc"))

# Comparison
print("abc" == "abc")
print("abc" == "def")
print("abc" < "abd")
print("abc" > "abb")
print("a" < "b")

# Contains (in operator)
print("lo" in "hello")
print("xyz" in "hello")
print("" in "hello")

# String with numbers via str()
print(str(42))
print(str(3.14))
print(str(True))
print(str(None))

# String concatenation with conversion
x = 10
print("value: " + str(x))
