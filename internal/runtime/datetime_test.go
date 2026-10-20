package runtime

import (
	"bytes"
	"context"
	"testing"
)

func dtExpect(t *testing.T, src, want string) {
	t.Helper()
	var buf bytes.Buffer
	rt := New(Config{Stdout: &buf})
	if err := rt.RunString(context.Background(), src, "<dt-test>"); err != nil {
		t.Fatalf("error: %v\nsource:\n%s", err, src)
	}
	if got := buf.String(); got != want {
		t.Errorf("source:\n%s\n got: %q\nwant: %q", src, got, want)
	}
}

func TestDate(t *testing.T) {
	src := `from datetime import date
d = date(2024, 1, 15)
print(d.year, d.month, d.day)
print(d.isoformat())
print(d)
`
	dtExpect(t, src, "2024 1 15\n2024-01-15\n2024-01-15\n")
}

func TestDateArithmetic(t *testing.T) {
	src := `from datetime import date, timedelta
d = date(2024, 1, 15)
later = d + timedelta(days=10)
print(later.isoformat())
diff = date(2024, 1, 25) - date(2024, 1, 15)
print(diff.days)
`
	dtExpect(t, src, "2024-01-25\n10\n")
}

func TestDateComparison(t *testing.T) {
	src := `from datetime import date
a = date(2024, 1, 1)
b = date(2024, 6, 1)
print(a < b)
print(a == date(2024, 1, 1))
print(b > a)
`
	dtExpect(t, src, "True\nTrue\nTrue\n")
}

func TestDatetime(t *testing.T) {
	src := `from datetime import datetime
dt = datetime(2024, 3, 14, 15, 9, 26)
print(dt.year, dt.hour, dt.minute, dt.second)
print(dt.isoformat())
`
	dtExpect(t, src, "2024 15 9 26\n2024-03-14T15:09:26\n")
}

func TestDatetimeArithmetic(t *testing.T) {
	src := `from datetime import datetime, timedelta
start = datetime(2024, 1, 1, 12, 0, 0)
end = datetime(2024, 1, 1, 14, 30, 0)
delta = end - start
print(delta.total_seconds())
later = start + timedelta(hours=1)
print(later.hour)
`
	dtExpect(t, src, "9000\n13\n")
}

func TestTimedelta(t *testing.T) {
	src := `from datetime import timedelta
d = timedelta(days=2, hours=3)
print(d.days)
print(d.seconds)
print(d.total_seconds())
`
	dtExpect(t, src, "2\n10800\n183600\n")
}

func TestTimedeltaArithmetic(t *testing.T) {
	src := `from datetime import timedelta
a = timedelta(hours=5)
b = timedelta(hours=3)
print((a + b).total_seconds())
print((a - b).total_seconds())
print(a > b)
`
	dtExpect(t, src, "28800\n7200\nTrue\n")
}

func TestDateStrftime(t *testing.T) {
	src := `from datetime import date
d = date(2024, 12, 25)
print(d.strftime("%Y/%m/%d"))
`
	dtExpect(t, src, "2024/12/25\n")
}

func TestDateReplace(t *testing.T) {
	src := `from datetime import date
d = date(2024, 1, 15)
d2 = d.replace(year=2025)
print(d2.isoformat())
print(d.isoformat())
`
	dtExpect(t, src, "2025-01-15\n2024-01-15\n")
}

func TestDateWeekday(t *testing.T) {
	src := `from datetime import date
# 2024-01-15 is a Monday → weekday() == 0
print(date(2024, 1, 15).weekday())
`
	dtExpect(t, src, "0\n")
}

func TestDatetimeImport(t *testing.T) {
	src := `import datetime
d = datetime.date(2024, 1, 1)
print(d.isoformat())
`
	dtExpect(t, src, "2024-01-01\n")
}
