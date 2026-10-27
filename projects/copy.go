package projects

import "encoding/json"

// clone copies a snapshot through JSON, as inbox does: every persisted field
// has a JSON-safe type, and Checkout is deliberately left out.
func clone[T any](v T) T {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	var out T
	if err := json.Unmarshal(b, &out); err != nil {
		panic(err)
	}
	return out
}
