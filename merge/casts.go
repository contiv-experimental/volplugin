package merge

import (
	"reflect"
	"strconv"
)

func castString(field *reflect.Value, val string) error {
	field.Set(reflect.ValueOf(val))
	return nil
}

func castUint32(field *reflect.Value, val string) error {
	out, err := strconv.ParseUint(val, 10, 32)
	if err != nil {
		return err
	}

	field.Set(reflect.ValueOf(uint(out)))
	return nil
}

// these cannot be abstracted; I already tried. :D
func castUint64(field *reflect.Value, val string) error {
	out, err := strconv.ParseUint(val, 10, 64)
	if err != nil {
		return err
	}

	field.Set(reflect.ValueOf(uint64(out)))
	return nil
}

func castInt32(field *reflect.Value, val string) error {
	out, err := strconv.ParseInt(val, 10, 32)
	if err != nil {
		return err
	}

	field.Set(reflect.ValueOf(int(out)))
	return nil
}

func castInt64(field *reflect.Value, val string) error {
	out, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return err
	}

	field.Set(reflect.ValueOf(int64(out)))
	return nil
}

func castBool(field *reflect.Value, val string) error {
	out, err := strconv.ParseBool(val)
	if err != nil {
		return err
	}

	field.Set(reflect.ValueOf(out))
	return nil
}

func castPtr(field *reflect.Value, val string) error {
	// in this case we have a pointer; we call the Elem() method and recurse to
	// (hopefully) avoid a panic.
	ptrField := field.Elem()
	return setValueWithType(&ptrField, val)
}
