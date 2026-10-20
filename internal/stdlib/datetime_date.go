package stdlib

import (
	"fmt"
	"time"

	"github.com/pygo/pygo/internal/object"
)

// ============================================================
// date
// ============================================================

// DateObject represents a calendar date (no time component).
type DateObject struct {
	t time.Time // normalized to midnight UTC
}

func (d *DateObject) Type() *object.TypeObject { return dateType }
func (d *DateObject) String() string           { return d.isoformat() }
func (d *DateObject) Repr() string {
	return fmt.Sprintf("datetime.date(%d, %d, %d)", d.t.Year(), int(d.t.Month()), d.t.Day())
}
func (d *DateObject) isoformat() string {
	return fmt.Sprintf("%04d-%02d-%02d", d.t.Year(), int(d.t.Month()), d.t.Day())
}

func (d *DateObject) GetAttr(name string) (object.Object, error) {
	switch name {
	case "year":
		return object.IntFrom(int64(d.t.Year())), nil
	case "month":
		return object.IntFrom(int64(d.t.Month())), nil
	case "day":
		return object.IntFrom(int64(d.t.Day())), nil
	case "isoformat":
		return fn("date.isoformat", func(_ []object.Object, _ map[string]object.Object) (object.Object, error) {
			return object.StrFrom(d.isoformat()), nil
		}), nil
	case "weekday":
		return fn("date.weekday", func(_ []object.Object, _ map[string]object.Object) (object.Object, error) {
			// Python: Monday is 0 … Sunday is 6.
			wd := (int(d.t.Weekday()) + 6) % 7
			return object.IntFrom(int64(wd)), nil
		}), nil
	case "strftime":
		return fn("date.strftime", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
			format, err := argStr(args, 0)
			if err != nil {
				return nil, err
			}
			return object.StrFrom(strftime(format, d.t)), nil
		}), nil
	case "replace":
		return fn("date.replace", func(args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
			y, mo, day := d.t.Year(), int(d.t.Month()), d.t.Day()
			y = intKw(kwargs, "year", y)
			mo = intKw(kwargs, "month", mo)
			day = intKw(kwargs, "day", day)
			return &DateObject{t: time.Date(y, time.Month(mo), day, 0, 0, 0, 0, time.UTC)}, nil
		}), nil
	}
	return nil, object.NewAttributeError("date", name)
}
func (d *DateObject) SetAttr(string, object.Object) error {
	return object.NewTypeError("'date' attributes are read-only")
}
func (d *DateObject) DelAttr(string) error {
	return object.NewTypeError("'date' attributes are read-only")
}

// dateFactory constructs and provides classmethods for date.
type dateFactory struct{}

func (f *dateFactory) Type() *object.TypeObject { return object.TypeType }
func (f *dateFactory) String() string           { return "<class 'datetime.date'>" }
func (f *dateFactory) Repr() string              { return f.String() }

func (f *dateFactory) Call(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	if len(args) != 3 {
		return nil, object.NewTypeError("date expected 3 arguments (year, month, day)")
	}
	y, err := argInt(args, 0)
	if err != nil {
		return nil, err
	}
	mo, err := argInt(args, 1)
	if err != nil {
		return nil, err
	}
	day, err := argInt(args, 2)
	if err != nil {
		return nil, err
	}
	return &DateObject{t: time.Date(int(y), time.Month(mo), int(day), 0, 0, 0, 0, time.UTC)}, nil
}

func (f *dateFactory) GetAttr(name string) (object.Object, error) {
	switch name {
	case "today":
		return fn("date.today", func(_ []object.Object, _ map[string]object.Object) (object.Object, error) {
			now := time.Now()
			return &DateObject{t: time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)}, nil
		}), nil
	case "fromisoformat":
		return fn("date.fromisoformat", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
			s, err := argStr(args, 0)
			if err != nil {
				return nil, err
			}
			parsed, perr := time.Parse("2006-01-02", s)
			if perr != nil {
				return nil, object.NewValueError("Invalid isoformat string: '%s'", s)
			}
			return &DateObject{t: parsed}, nil
		}), nil
	}
	return nil, object.NewAttributeError("date", name)
}
func (f *dateFactory) SetAttr(string, object.Object) error {
	return object.NewTypeError("cannot set attributes on date type")
}
func (f *dateFactory) DelAttr(string) error {
	return object.NewTypeError("cannot delete attributes on date type")
}

// ============================================================
// datetime
// ============================================================

// DateTimeObject represents a date with a time component.
type DateTimeObject struct {
	t time.Time
}

func (dt *DateTimeObject) Type() *object.TypeObject { return datetimeType }
func (dt *DateTimeObject) String() string           { return dt.isoformat() }
func (dt *DateTimeObject) Repr() string {
	return fmt.Sprintf("datetime.datetime(%d, %d, %d, %d, %d, %d)",
		dt.t.Year(), int(dt.t.Month()), dt.t.Day(), dt.t.Hour(), dt.t.Minute(), dt.t.Second())
}
func (dt *DateTimeObject) isoformat() string {
	return dt.t.Format("2006-01-02T15:04:05")
}

func (dt *DateTimeObject) GetAttr(name string) (object.Object, error) {
	switch name {
	case "year":
		return object.IntFrom(int64(dt.t.Year())), nil
	case "month":
		return object.IntFrom(int64(dt.t.Month())), nil
	case "day":
		return object.IntFrom(int64(dt.t.Day())), nil
	case "hour":
		return object.IntFrom(int64(dt.t.Hour())), nil
	case "minute":
		return object.IntFrom(int64(dt.t.Minute())), nil
	case "second":
		return object.IntFrom(int64(dt.t.Second())), nil
	case "isoformat":
		return fn("datetime.isoformat", func(_ []object.Object, _ map[string]object.Object) (object.Object, error) {
			return object.StrFrom(dt.isoformat()), nil
		}), nil
	case "strftime":
		return fn("datetime.strftime", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
			format, err := argStr(args, 0)
			if err != nil {
				return nil, err
			}
			return object.StrFrom(strftime(format, dt.t)), nil
		}), nil
	case "date":
		return fn("datetime.date", func(_ []object.Object, _ map[string]object.Object) (object.Object, error) {
			y, mo, day := dt.t.Date()
			return &DateObject{t: time.Date(y, mo, day, 0, 0, 0, 0, time.UTC)}, nil
		}), nil
	case "timestamp":
		return fn("datetime.timestamp", func(_ []object.Object, _ map[string]object.Object) (object.Object, error) {
			return object.FloatFrom(float64(dt.t.UnixNano()) / 1e9), nil
		}), nil
	}
	return nil, object.NewAttributeError("datetime", name)
}
func (dt *DateTimeObject) SetAttr(string, object.Object) error {
	return object.NewTypeError("'datetime' attributes are read-only")
}
func (dt *DateTimeObject) DelAttr(string) error {
	return object.NewTypeError("'datetime' attributes are read-only")
}

// datetimeFactory constructs and provides classmethods for datetime.
type datetimeFactory struct{}

func (f *datetimeFactory) Type() *object.TypeObject { return object.TypeType }
func (f *datetimeFactory) String() string           { return "<class 'datetime.datetime'>" }
func (f *datetimeFactory) Repr() string              { return f.String() }

func (f *datetimeFactory) Call(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	if len(args) < 3 {
		return nil, object.NewTypeError("datetime expected at least 3 arguments")
	}
	comps := []int{0, 1, 0, 0, 0, 0} // year, month, day, hour, minute, second
	for i := 0; i < 6 && i < len(args); i++ {
		v, err := argInt(args, i)
		if err != nil {
			return nil, err
		}
		comps[i] = int(v)
	}
	t := time.Date(comps[0], time.Month(comps[1]), comps[2], comps[3], comps[4], comps[5], 0, time.UTC)
	return &DateTimeObject{t: t}, nil
}

func (f *datetimeFactory) GetAttr(name string) (object.Object, error) {
	switch name {
	case "now":
		return fn("datetime.now", func(_ []object.Object, _ map[string]object.Object) (object.Object, error) {
			return &DateTimeObject{t: time.Now()}, nil
		}), nil
	case "utcnow":
		return fn("datetime.utcnow", func(_ []object.Object, _ map[string]object.Object) (object.Object, error) {
			return &DateTimeObject{t: time.Now().UTC()}, nil
		}), nil
	case "fromtimestamp":
		return fn("datetime.fromtimestamp", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
			secs, err := argFloat(args, 0)
			if err != nil {
				return nil, err
			}
			return &DateTimeObject{t: time.Unix(int64(secs), 0)}, nil
		}), nil
	}
	return nil, object.NewAttributeError("datetime", name)
}
func (f *datetimeFactory) SetAttr(string, object.Object) error {
	return object.NewTypeError("cannot set attributes on datetime type")
}
func (f *datetimeFactory) DelAttr(string) error {
	return object.NewTypeError("cannot delete attributes on datetime type")
}

// intKw reads an integer keyword argument, falling back to def.
func intKw(kwargs map[string]object.Object, name string, def int) int {
	if v, ok := kwargs[name]; ok {
		if n, ok := v.(*object.IntObject); ok {
			iv, _ := n.Int64()
			return int(iv)
		}
	}
	return def
}
