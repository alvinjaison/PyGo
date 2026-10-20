package object

import "github.com/pygo/pygo/internal/bytecode"

// GetAttr exposes list methods as bound builtins. Methods that mutate operate
// on the list in place.
func (l *ListObject) GetAttr(name string) (Object, error) {
	m, ok := listMethods[name]
	if !ok {
		return nil, NewAttributeError("list", name)
	}
	return NewBuiltin("list."+name, func(args []Object, _ map[string]Object) (Object, error) {
		return m(l, args)
	}), nil
}

func (l *ListObject) SetAttr(string, Object) error {
	return NewTypeError("'list' object attributes are read-only")
}
func (l *ListObject) DelAttr(string) error {
	return NewTypeError("'list' object attributes are read-only")
}

type listMethod func(l *ListObject, args []Object) (Object, error)

var listMethods = map[string]listMethod{
	"append": func(l *ListObject, args []Object) (Object, error) {
		if len(args) != 1 {
			return nil, NewTypeError("append() takes exactly one argument (%d given)", len(args))
		}
		l.Items = append(l.Items, args[0])
		return None, nil
	},
	"extend": func(l *ListObject, args []Object) (Object, error) {
		it, err := GetIter(args[0])
		if err != nil {
			return nil, err
		}
		for {
			v, err := it.Next()
			if err == ErrStopIteration {
				break
			}
			if err != nil {
				return nil, err
			}
			l.Items = append(l.Items, v)
		}
		return None, nil
	},
	"insert": func(l *ListObject, args []Object) (Object, error) {
		if len(args) != 2 {
			return nil, NewTypeError("insert() takes exactly 2 arguments")
		}
		idxObj, ok := args[0].(*IntObject)
		if !ok {
			return nil, NewTypeError("'%s' object cannot be interpreted as an integer", args[0].Type().Name())
		}
		i64, _ := idxObj.Int64()
		i := int(i64)
		if i < 0 {
			i += len(l.Items)
		}
		if i < 0 {
			i = 0
		}
		if i > len(l.Items) {
			i = len(l.Items)
		}
		l.Items = append(l.Items, nil)
		copy(l.Items[i+1:], l.Items[i:])
		l.Items[i] = args[1]
		return None, nil
	},
	"pop": func(l *ListObject, args []Object) (Object, error) {
		if len(l.Items) == 0 {
			return nil, NewIndexError("pop from empty list")
		}
		i := len(l.Items) - 1
		if len(args) > 0 {
			if n, ok := args[0].(*IntObject); ok {
				v, _ := n.Int64()
				i = int(v)
				if i < 0 {
					i += len(l.Items)
				}
			}
		}
		if i < 0 || i >= len(l.Items) {
			return nil, NewIndexError("pop index out of range")
		}
		v := l.Items[i]
		l.Items = append(l.Items[:i], l.Items[i+1:]...)
		return v, nil
	},
	"remove": func(l *ListObject, args []Object) (Object, error) {
		for i, v := range l.Items {
			eq, err := objectsEqual(v, args[0])
			if err != nil {
				return nil, err
			}
			if eq {
				l.Items = append(l.Items[:i], l.Items[i+1:]...)
				return None, nil
			}
		}
		return nil, NewValueError("list.remove(x): x not in list")
	},
	"index": func(l *ListObject, args []Object) (Object, error) {
		for i, v := range l.Items {
			eq, err := objectsEqual(v, args[0])
			if err != nil {
				return nil, err
			}
			if eq {
				return IntFrom(int64(i)), nil
			}
		}
		return nil, NewValueError("%s is not in list", args[0].Repr())
	},
	"count": func(l *ListObject, args []Object) (Object, error) {
		n := int64(0)
		for _, v := range l.Items {
			eq, err := objectsEqual(v, args[0])
			if err != nil {
				return nil, err
			}
			if eq {
				n++
			}
		}
		return IntFrom(n), nil
	},
	"reverse": func(l *ListObject, _ []Object) (Object, error) {
		for i, j := 0, len(l.Items)-1; i < j; i, j = i+1, j-1 {
			l.Items[i], l.Items[j] = l.Items[j], l.Items[i]
		}
		return None, nil
	},
	"clear": func(l *ListObject, _ []Object) (Object, error) {
		l.Items = nil
		return None, nil
	},
	"copy": func(l *ListObject, _ []Object) (Object, error) {
		return NewList(l.Items), nil
	},
	"sort": func(l *ListObject, _ []Object) (Object, error) {
		// In-place stable sort by default ordering.
		items := l.Items
		for i := 1; i < len(items); i++ {
			key := items[i]
			j := i - 1
			for j >= 0 {
				res, err := compareObjects(key, items[j], bytecode.CmpLt)
				if err != nil {
					return nil, err
				}
				less, _ := IsTruthy(res)
				if !less {
					break
				}
				items[j+1] = items[j]
				j--
			}
			items[j+1] = key
		}
		return None, nil
	},
}
