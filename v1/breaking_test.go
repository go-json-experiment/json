package json

// List of breaking v1 changes.
// TODO: Some of these might need to be a v1 compatibility option
// or alternatively a GODEBUG option (see https://go.dev/doc/godebug).
const (
	// avoidNilPointerMethods means that implementation always avoids
	// calling MarshalJSON and MarshalText methods on nil pointers.
	// This is a break from historical v1 behavior.
	// In particular, v1 would call the method only if the type is an interface,
	// and the interface implements the method. However, if the type is a
	// concrete pointer type or an interface (without the method) but contains
	// a nil pointer, then the method is not called. This is inconsistent.
	//
	// See https://go.dev/play/p/WaTuxCmqDGL
	// See https://go.dev/play/p/9WeUhxXYiMH
	avoidNilPointerMethods = true

	// namedByteInSliceAsJSONArray means that a named byte in a slice
	// (e.g., []MyByte) is treated as JSON array of MyByte,
	// rather than as a JSON string that contains base64-encoded binary data.
	// The v1 behavior has always been relying on a bug in reflect,
	// where reflect.Value.Bytes can be called on such types even though
	// the Go language specifies that []MyByte cannot be directly converted
	// into a []byte (i.e., the two types are invariant).
	//
	// See https://go.dev/issue/24746
	namedByteInSliceAsJSONArray = true
)

func ternary[T any](boolVal bool, trueVal, falseVal T) T {
	if boolVal {
		return trueVal
	} else {
		return falseVal
	}
}
