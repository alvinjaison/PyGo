# Collections: list/tuple/dict operations, indexing, methods

# List creation and indexing
lst = [10, 20, 30, 40, 50]
print(lst[0])
print(lst[2])
print(lst[-1])

# List length
print(len(lst))

# List append and pop
lst = [1, 2, 3]
lst.append(4)
print(lst)
popped = lst.pop()
print(popped)
print(lst)

# List contains
print(2 in [1, 2, 3])
print(5 in [1, 2, 3])

# List iteration
result = []
for x in [1, 2, 3]:
    result.append(x * 2)
print(result)

# Tuple creation and indexing
t = (1, 2, 3, 4, 5)
print(t[0])
print(t[-1])
print(len(t))

# Tuple contains
print(3 in (1, 2, 3))
print(7 in (1, 2, 3))

# Dict creation and access
d = {"a": 1, "b": 2, "c": 3}
print(d["a"])
print(d["c"])

# Dict assignment
d["d"] = 4
print(d["d"])

# Dict length
print(len(d))

# Dict contains (key membership)
print("a" in d)
print("z" in d)

# Dict get with default
print(d.get("a"))
print(d.get("z", 99))

# List modification by index
lst = [1, 2, 3, 4]
lst[0] = 10
print(lst)
