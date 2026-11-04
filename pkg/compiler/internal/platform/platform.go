// Package platform holds the policy vocabulary the compiler owns and every emitter
// consumes.
//
// Platform decisions have a single source instead of per-emitter literals.
package platform

import "github.com/galihanggara68/foundation/pkg/compiler/internal/ir"

// Compiler-owned pagination defaults.
const (
	DefaultPageLimit    = 20
	DefaultMaxPageLimit = 100
)

// Info carries the platform policy that the compiler owns.
// Emitters consume platform decisions instead of inventing local constants.
type Info struct {
	Pagination     PaginationDefaults `json:"pagination"`
	ReservedErrors []ReservedError    `json:"reserved_errors"`
}

// PaginationDefaults describes the platform page-size policy.
type PaginationDefaults struct {
	DefaultLimit int `json:"default_limit"`
	MaxLimit     int `json:"max_limit"`
}

// ReservedError defines a standard error pre-allocated by the platform.
type ReservedError struct {
	Name    string          `json:"name"`
	Kind    ir.ErrorKind    `json:"kind"`
	Status  int             `json:"status"`
	Posture ir.ErrorPosture `json:"posture"`
}

// StandardReservedErrors returns the seven standard platform errors in stable order.
func StandardReservedErrors() []ReservedError {
	return []ReservedError{
		{
			Name:    "NotFound",
			Kind:    ir.ErrorKindNotFound,
			Status:  404,
			Posture: ir.PostureBusiness,
		},
		{
			Name:    "Validation",
			Kind:    ir.ErrorKindValidation,
			Status:  400,
			Posture: ir.PostureBusiness,
		},
		{
			Name:    "Conflict",
			Kind:    ir.ErrorKindConflict,
			Status:  409,
			Posture: ir.PostureBusiness,
		},
		{
			Name:    "Unauthenticated",
			Kind:    ir.ErrorKindUnauthenticated,
			Status:  401,
			Posture: ir.PostureBusiness,
		},
		{
			Name:    "Unauthorized",
			Kind:    ir.ErrorKindUnauthorized,
			Status:  403,
			Posture: ir.PostureBusiness,
		},
		{
			Name:    "Forbidden",
			Kind:    ir.ErrorKindForbidden,
			Status:  403,
			Posture: ir.PostureBusiness,
		},
		{
			Name:    "Internal",
			Kind:    ir.ErrorKindInternal,
			Status:  500,
			Posture: ir.PostureTechnical,
		},
	}
}

// DefaultInfo returns the compiler-owned platform policy.
func DefaultInfo() Info {
	return Info{
		Pagination: PaginationDefaults{
			DefaultLimit: DefaultPageLimit,
			MaxLimit:     DefaultMaxPageLimit,
		},
		ReservedErrors: StandardReservedErrors(),
	}
}
