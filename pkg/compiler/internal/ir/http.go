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

// ActionRoutePaths returns all route path strings declared on action traits.
func ActionRoutePaths(a ActionDecl) []string {
	var paths []string
	for _, tr := range a.Traits {
		if tr.Name == "route" || tr.Name == "http" {
			for _, key := range []string{"path", "_", "_0", "_1"} {
				if val, ok := tr.Args[key]; ok && val != "" {
					paths = append(paths, strings.Trim(val, `"'`))
					break
				}
			}
		}
	}
	return paths
}

// ParamMatchesRoutePath reports whether paramName appears as {paramName} in the route path.
func ParamMatchesRoutePath(paramName, path string) bool {
	if paramName == "" || path == "" {
		return false
	}
	if strings.Contains(path, "{"+paramName+"}") || strings.Contains(path, "{"+paramName+":") {
		return true
	}
	for _, seg := range HTTPPathParamNames(path) {
		base := strings.Split(seg, ":")[0]
		if base == paramName {
			return true
		}
	}
	return false
}

// EnrichActionParamsInPath marks action parameters present in route paths with InPath = true.
func EnrichActionParamsInPath(a *ActionDecl) {
	paths := ActionRoutePaths(*a)
	if len(paths) == 0 {
		return
	}
	for _, path := range paths {
		for pi := range a.Params {
			p := &a.Params[pi]
			if ParamMatchesRoutePath(p.Name, path) {
				p.InPath = true
			}
		}
	}
}
