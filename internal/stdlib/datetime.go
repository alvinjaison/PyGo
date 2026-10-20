package stdlib

import (
	"fmt"
	"time"

	"github.com/pygo/pygo/internal/object"
)

// Dedicated type objects for the datetime classes. Arithmetic and comparison
// slots are wired in registerDatetimeSlots (called once from buildDatetime).
var (
	dateType      = object.NewClass("date", nil, object.NewDict())
	datetimeType  = object.NewClass("datetime", nil, object.NewDict())
	timedeltaType = object.NewClass("timedelta", nil, object.NewDict())
)

// init wires the datetime arithmetic/comparison slots once at package load,
// so they are immutable by the time any runtime imports the module (no
// concurrency concern across runtime instances).
func init() {
	registerDatetimeSlots()
}

// buildDatetime constructs the `datetime` module with date, datetime, and
// timedelta classes.
func buildDatetime(name string) *object.ModuleObject {
	mod, set := newModule(name)

	set("date", &dateFactory{})
	set("datetime", &datetimeFactory{})
	set("timedelta", &timedeltaFactory{})

	return mod
}

// ============================================================
// timedelta
// ============================================================

// TimeDeltaObject represents a duration. Internally stored as a Go duration;
// Python exposes days/seconds/microseconds.
type TimeDeltaObject struct {
	d time.Duration
}

func (t *TimeDeltaObject) Type() *object.TypeObject { return timedeltaType }
func (t *TimeDeltaObject) String() string           { return t.Repr() }
func (t *TimeDeltaObject) Repr() string {
	// Match Python's str(timedelta): e.g. "1 day, 2:03:04" or "0:00:05".
	total := t.d
	neg := total < 0
	if neg {
		total = -total
	}
	days := int64(total / (24 * time.Hour))
	rem := total % (24 * time.Hour)
	h := int64(rem / time.Hour)
	m := int64((rem % time.Hour) / time.Minute)
	s := int64((rem % time.Minute) / time.Second)

	var out string
	switch {
	case days == 1:
		out = fmt.Sprintf("1 day, %d:%02d:%02d", h, m, s)
	case days > 1 || days < -1 && false:
		out = fmt.Sprintf("%d days, %d:%02d:%02d", days, h, m, s)
	default:
		out = fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	if neg {
		out = "-" + out
	}
	return out
}
func (t *TimeDeltaObject) IsTrue() (bool, error) { return t.d != 0, nil }

func (t *TimeDeltaObject) totalSeconds() float64 { return t.d.Seconds() }

func (t *TimeDeltaObject) GetAttr(name string) (object.Object, error) {
	switch name {
	case "days":
		return object.IntFrom(int64(t.d / (24 * time.Hour))), nil
	case "seconds":
		// Seconds within the day (Python normalizes this way).
		rem := t.d % (24 * time.Hour)
		if rem < 0 {
			rem += 24 * time.Hour
		}
		return object.IntFrom(int64(rem / time.Second)), nil
	case "microseconds":
		us := (t.d % time.Second) / time.Microsecond
		if us < 0 {
			us += 1_000_000
		}
		return object.IntFrom(int64(us)), nil
	case "total_seconds":
		return fn("timedelta.total_seconds", func(_ []object.Object, _ map[string]object.Object) (object.Object, error) {
			return object.FloatFrom(t.totalSeconds()), nil
		}), nil
	}
	return nil, object.NewAttributeError("timedelta", name)
}
func (t *TimeDeltaObject) SetAttr(string, object.Object) error {
	return object.NewTypeError("'timedelta' attributes are read-only")
}
func (t *TimeDeltaObject) DelAttr(string) error {
	return object.NewTypeError("'timedelta' attributes are read-only")
}

// timedeltaFactory constructs timedelta objects.
type timedeltaFactory struct{}

func (f *timedeltaFactory) Type() *object.TypeObject { return object.TypeType }
func (f *timedeltaFactory) String() string           { return "<class 'datetime.timedelta'>" }
func (f *timedeltaFactory) Repr() string              { return f.String() }

// Call builds a timedelta from keyword-style components. Positional order
// matches Python: days, seconds, microseconds (then hours/minutes/weeks via kw).
func (f *timedeltaFactory) Call(args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
	var d time.Duration
	add := func(v object.Object, unit time.Duration) error {
		switch n := v.(type) {
		case *object.IntObject:
			iv, _ := n.Int64()
			d += time.Duration(iv) * unit
		case *object.FloatObject:
			d += time.Duration(n.Value * float64(unit))
		default:
			return object.NewTypeError("timedelta component must be a number")
		}
		return nil
	}

	units := []struct {
		name string
		unit time.Duration
	}{
		{"days", 24 * time.Hour},
		{"seconds", time.Second},
		{"microseconds", time.Microsecond},
	}
	for i, u := range units {
		if i < len(args) {
			if err := add(args[i], u.unit); err != nil {
				return nil, err
			}
		}
	}
	kwUnits := map[string]time.Duration{
		"days":         24 * time.Hour,
		"seconds":      time.Second,
		"microseconds": time.Microsecond,
		"milliseconds": time.Millisecond,
		"minutes":      time.Minute,
		"hours":        time.Hour,
		"weeks":        7 * 24 * time.Hour,
	}
	for name, v := range kwargs {
		unit, ok := kwUnits[name]
		if !ok {
			return nil, object.NewTypeError("'%s' is an invalid keyword argument for timedelta", name)
		}
		if err := add(v, unit); err != nil {
			return nil, err
		}
	}
	return &TimeDeltaObject{d: d}, nil
}
