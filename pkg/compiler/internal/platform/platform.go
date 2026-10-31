// Package platform holds the policy vocabulary the compiler owns and every emitter
// consumes, so platform decisions have a single source instead of per-emitter literals.
//
// It is a leaf package: beyond the ir vocabulary itself it imports nothing, so the
// validation and emission stages can both depend on it without pulling architecture
// or wire-protocol concerns into either.
package platform

// Compiler-owned pagination defaults.
const (
	DefaultPageLimit    = 20
	DefaultMaxPageLimit = 100
)

// Info carries the platform policy the compiler owns, so emitters consume platform
// decisions instead of inventing their own and drifting from siblings.
//
// Only pagination is populated today: it is the one platform fact with a real consumer.
// Identity posture and a database schema target are deliberately absent — nothing
// derives or consumes them yet, and publishing unconsumed fields is the
// documented-but-empty trap spec 05 item 8 warns about. They return as additive
// sub-objects once they have a consumer.
type Info struct {
	Pagination PaginationDefaults `json:"pagination"`
}

// PaginationDefaults are the platform's page-size policy.
type PaginationDefaults struct {
	DefaultLimit int `json:"default_limit"`
	MaxLimit     int `json:"max_limit"`
}

// DefaultInfo returns the compiler-owned platform policy published on every request.
func DefaultInfo() Info {
	return Info{
		Pagination: PaginationDefaults{
			DefaultLimit: DefaultPageLimit,
			MaxLimit:     DefaultMaxPageLimit,
		},
	}
}
