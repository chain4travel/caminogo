package wrappers

func IgnoreError(val any, err error) interface{} { //nolint:revive
	return val
}
