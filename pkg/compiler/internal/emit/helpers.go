package emit

// CloneBoolMap copies a declared-variable tracking set so branch scopes can shadow
// variable declarations without leaking into sibling scopes. Shared by all step emitters.
func CloneBoolMap(in map[string]bool) map[string]bool {
	out := make(map[string]bool, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
