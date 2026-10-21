package stdlib

import (
	"time"

	"github.com/alvinjaison/PyGo/internal/object"
)

// buildTime constructs the `time` module.
func buildTime(name string) *object.ModuleObject {
	mod, set := newModule(name)

	// time() → seconds since the Unix epoch as a float.
	set("time", fn("time.time", func(_ []object.Object, _ map[string]object.Object) (object.Object, error) {
		return object.FloatFrom(float64(time.Now().UnixNano()) / 1e9), nil
	}))

	// time_ns() → nanoseconds since the epoch as an int.
	set("time_ns", fn("time.time_ns", func(_ []object.Object, _ map[string]object.Object) (object.Object, error) {
		return object.IntFrom(time.Now().UnixNano()), nil
	}))

	// monotonic() → a monotonically increasing clock in seconds.
	start := time.Now()
	set("monotonic", fn("time.monotonic", func(_ []object.Object, _ map[string]object.Object) (object.Object, error) {
		return object.FloatFrom(time.Since(start).Seconds()), nil
	}))

	// perf_counter() → high-resolution timer in seconds (same base here).
	set("perf_counter", fn("time.perf_counter", func(_ []object.Object, _ map[string]object.Object) (object.Object, error) {
		return object.FloatFrom(time.Since(start).Seconds()), nil
	}))

	// sleep(seconds) — suspends execution. Accepts int or float.
	set("sleep", fn("time.sleep", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		secs, err := argFloat(args, 0)
		if err != nil {
			return nil, err
		}
		if secs < 0 {
			return nil, object.NewValueError("sleep length must be non-negative")
		}
		time.Sleep(time.Duration(secs * float64(time.Second)))
		return object.None, nil
	}))

	// strftime(format[, struct]) — minimal: formats the current local time.
	set("strftime", fn("time.strftime", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		format, err := argStr(args, 0)
		if err != nil {
			return nil, err
		}
		return object.StrFrom(strftime(format, time.Now())), nil
	}))

	return mod
}

// strftime translates a subset of C/Python strftime directives using Go's
// reference-time formatting.
func strftime(format string, t time.Time) string {
	replacements := []struct{ dir, layout string }{
		{"%Y", "2006"},
		{"%m", "01"},
		{"%d", "02"},
		{"%H", "15"},
		{"%M", "04"},
		{"%S", "05"},
		{"%y", "06"},
		{"%p", "PM"},
		{"%A", "Monday"},
		{"%a", "Mon"},
		{"%B", "January"},
		{"%b", "Jan"},
		{"%j", "002"},
		{"%Z", "MST"},
	}
	out := format
	for _, r := range replacements {
		out = replaceAll(out, r.dir, t.Format(r.layout))
	}
	out = replaceAll(out, "%%", "%")
	return out
}

func replaceAll(s, old, new string) string {
	var b []byte
	for i := 0; i < len(s); {
		if i+len(old) <= len(s) && s[i:i+len(old)] == old {
			b = append(b, new...)
			i += len(old)
		} else {
			b = append(b, s[i])
			i++
		}
	}
	return string(b)
}
