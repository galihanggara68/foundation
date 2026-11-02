package ir_test

import (
	"reflect"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

func TestHTTPPathParamNames(t *testing.T) {
	cases := []struct {
		path string
		want []string
	}{
		{"/v1/tables/{id}/preview", []string{"id"}},
		{"/v1/tables/{id}/versions/{version}", []string{"id", "version"}},
		{"/v1/{id}/tables/{id}/preview", []string{"id", "id"}},
		{"/v1/tables/preview", nil},
		{"/{}", []string{""}},
		{"/{id:uuid}", []string{"id:uuid"}},
		{"/{id", nil},
		{"/x{id}", nil},
		{"/v1/{id}/", []string{"id"}},
	}

	for _, tc := range cases {
		got := ir.HTTPPathParamNames(tc.path)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("HTTPPathParamNames(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}
