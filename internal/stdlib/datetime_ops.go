package stdlib

import (
	"time"

	"github.com/pygo/pygo/internal/object"
)

// registerDatetimeSlots wires arithmetic and comparison onto the date,
// datetime, and timedelta type objects so operators work via the standard
// dispatch in internal/object/ops.go.
func registerDatetimeSlots() {
	// --- timedelta ---
	ts := timedeltaType.Slots()
	ts.Add = func(a, b object.Object) (object.Object, error) {
		x, y := asDelta(a), asDelta(b)
		if x == nil || y == nil {
			return object.NotImplemented, nil
		}
		return &TimeDeltaObject{d: x.d + y.d}, nil
	}
	ts.Sub = func(a, b object.Object) (object.Object, error) {
		x, y := asDelta(a), asDelta(b)
		if x == nil || y == nil {
			return object.NotImplemented, nil
		}
		return &TimeDeltaObject{d: x.d - y.d}, nil
	}
	ts.Neg = func(self object.Object) (object.Object, error) {
		return &TimeDeltaObject{d: -asDelta(self).d}, nil
	}
	ts.RichCmp = func(a, b object.Object, op object.CompareOp) (object.Object, error) {
		x, y := asDelta(a), asDelta(b)
		if x == nil || y == nil {
			return object.NotImplemented, nil
		}
		return cmpDurations(x.d, y.d, op), nil
	}

	// --- date ---
	ds := dateType.Slots()
	ds.Add = func(a, b object.Object) (object.Object, error) {
		if d := asDate(a); d != nil {
			if td := asDelta(b); td != nil {
				return &DateObject{t: d.t.Add(td.d)}, nil
			}
		}
		if d := asDate(b); d != nil { // timedelta + date
			if td := asDelta(a); td != nil {
				return &DateObject{t: d.t.Add(td.d)}, nil
			}
		}
		return object.NotImplemented, nil
	}
	ds.Sub = func(a, b object.Object) (object.Object, error) {
		da := asDate(a)
		if da == nil {
			return object.NotImplemented, nil
		}
		if db := asDate(b); db != nil { // date - date = timedelta
			return &TimeDeltaObject{d: da.t.Sub(db.t)}, nil
		}
		if td := asDelta(b); td != nil { // date - timedelta = date
			return &DateObject{t: da.t.Add(-td.d)}, nil
		}
		return object.NotImplemented, nil
	}
	ds.RichCmp = func(a, b object.Object, op object.CompareOp) (object.Object, error) {
		x, y := asDate(a), asDate(b)
		if x == nil || y == nil {
			return object.NotImplemented, nil
		}
		return cmpTimes(x.t, y.t, op), nil
	}

	// --- datetime ---
	dts := datetimeType.Slots()
	dts.Add = func(a, b object.Object) (object.Object, error) {
		if dt := asDateTime(a); dt != nil {
			if td := asDelta(b); td != nil {
				return &DateTimeObject{t: dt.t.Add(td.d)}, nil
			}
		}
		if dt := asDateTime(b); dt != nil {
			if td := asDelta(a); td != nil {
				return &DateTimeObject{t: dt.t.Add(td.d)}, nil
			}
		}
		return object.NotImplemented, nil
	}
	dts.Sub = func(a, b object.Object) (object.Object, error) {
		da := asDateTime(a)
		if da == nil {
			return object.NotImplemented, nil
		}
		if db := asDateTime(b); db != nil { // datetime - datetime = timedelta
			return &TimeDeltaObject{d: da.t.Sub(db.t)}, nil
		}
		if td := asDelta(b); td != nil { // datetime - timedelta = datetime
			return &DateTimeObject{t: da.t.Add(-td.d)}, nil
		}
		return object.NotImplemented, nil
	}
	dts.RichCmp = func(a, b object.Object, op object.CompareOp) (object.Object, error) {
		x, y := asDateTime(a), asDateTime(b)
		if x == nil || y == nil {
			return object.NotImplemented, nil
		}
		return cmpTimes(x.t, y.t, op), nil
	}
}

// ---- type assertions ----

func asDelta(o object.Object) *TimeDeltaObject {
	d, _ := o.(*TimeDeltaObject)
	return d
}
func asDate(o object.Object) *DateObject {
	d, _ := o.(*DateObject)
	return d
}
func asDateTime(o object.Object) *DateTimeObject {
	d, _ := o.(*DateTimeObject)
	return d
}

// ---- comparison helpers ----

func cmpTimes(a, b time.Time, op object.CompareOp) object.Object {
	c := 0
	switch {
	case a.Before(b):
		c = -1
	case a.After(b):
		c = 1
	}
	return boolForCmp(c, op)
}

func cmpDurations(a, b time.Duration, op object.CompareOp) object.Object {
	c := 0
	switch {
	case a < b:
		c = -1
	case a > b:
		c = 1
	}
	return boolForCmp(c, op)
}

func boolForCmp(c int, op object.CompareOp) object.Object {
	switch op {
	case object.OpLt:
		return object.BoolFrom(c < 0)
	case object.OpLtE:
		return object.BoolFrom(c <= 0)
	case object.OpEq:
		return object.BoolFrom(c == 0)
	case object.OpNotEq:
		return object.BoolFrom(c != 0)
	case object.OpGt:
		return object.BoolFrom(c > 0)
	case object.OpGtE:
		return object.BoolFrom(c >= 0)
	}
	return object.PyFalse
}
