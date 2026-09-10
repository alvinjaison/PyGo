# Control flow: if/elif/else, while, for, break/continue, while/else, for/else

# if/elif/else
x = 10
if x > 20:
    print("big")
elif x > 5:
    print("medium")
else:
    print("small")

# if with false condition
if False:
    print("never")
else:
    print("else branch")

# while loop
i = 0
total = 0
while i < 5:
    total = total + i
    i = i + 1
print(total)

# for loop over list
result = 0
for x in [1, 2, 3, 4, 5]:
    result = result + x
print(result)

# for loop over range
total = 0
for i in range(5):
    total = total + i
print(total)

# break in for loop
result = 0
for x in [1, 2, 3, 4, 5]:
    if x == 4:
        break
    result = result + x
print(result)

# continue in for loop
result = 0
for x in [1, 2, 3, 4, 5]:
    if x == 3:
        continue
    result = result + x
print(result)

# break in while loop
i = 0
while True:
    if i == 5:
        break
    i = i + 1
print(i)

# while/else - normal exit
i = 0
while i < 3:
    i = i + 1
else:
    print("while done")

# while/else - break exit (else skipped)
i = 0
while i < 10:
    if i == 3:
        break
    i = i + 1
else:
    print("should not print")
print(i)

# for/else - normal exit
for x in [1, 2, 3]:
    pass
else:
    print("for done")

# for/else - break exit (else skipped)
for x in [1, 2, 3, 4, 5]:
    if x == 3:
        break
else:
    print("should not print")
print(x)

# Nested loops with break
found = False
for i in [1, 2, 3]:
    for j in [10, 20, 30]:
        if j == 20:
            found = True
            break
print(found)
