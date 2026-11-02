package ir

import "strings"

// HTTPPathParamNames lists the {name} segments of an @http route path. A path piece is a
// parameter when it is wrapped in a single brace pair, and its name is the piece with that
// pair trimmed. The name may be empty ("{}") or carry a constraint ("{id:uuid}"); callers
// decide which of those they accept.
func HTTPPathParamNames(path string) []string {
	var names []string
	for _, seg := range strings.Split(path, "/") {
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			names = append(names, strings.TrimSuffix(strings.TrimPrefix(seg, "{"), "}"))
		}
	}
	return names
}
