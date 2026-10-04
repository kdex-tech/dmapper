package dmapper

import (
	"reflect"
	"strings"
)

// accumulate returns existing followed by each item of result not already
// present (order-preserving set-union) when both are lists; otherwise result,
// i.e. replace. The existing list's own duplicates are preserved; the result's
// are collapsed. The merged list is []string when every element is a string,
// else []any. existing is never mutated. See kdex-tech/host-manager#229.
func accumulate(existing, result any) any {
	ex, ok := asList(existing)
	if !ok {
		return result
	}
	res, ok := asList(result)
	if !ok {
		return result
	}

	merged := make([]any, 0, len(ex)+len(res))
	merged = append(merged, ex...)
	seen := make(map[string]struct{}, len(merged))
	for _, e := range merged {
		if s, ok := e.(string); ok {
			seen[s] = struct{}{}
		}
	}
	for _, r := range res {
		if s, ok := r.(string); ok {
			if _, dup := seen[s]; dup {
				continue
			}
			seen[s] = struct{}{}
		} else if containsDeepEqual(merged, r) {
			continue
		}
		merged = append(merged, r)
	}
	return narrow(merged)
}

// asList views v as a list. []string and []any are lists; nothing else is.
func asList(v any) ([]any, bool) {
	switch l := v.(type) {
	case []any:
		return l, true
	case []string:
		out := make([]any, len(l))
		for i, s := range l {
			out[i] = s
		}
		return out, true
	}
	return nil, false
}

// narrow returns l as []string when every element is a string, else l.
func narrow(l []any) any {
	out := make([]string, len(l))
	for i, e := range l {
		s, ok := e.(string)
		if !ok {
			return l
		}
		out[i] = s
	}
	return out
}

func containsDeepEqual(l []any, v any) bool {
	for _, e := range l {
		if reflect.DeepEqual(e, v) {
			return true
		}
	}
	return false
}

// getNestedPath reads the value at a dot-separated path, reporting whether
// every segment resolved.
func getNestedPath(m map[string]any, path string) (any, bool) {
	var cur any = m
	for _, part := range strings.Split(path, ".") {
		mm, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		if cur, ok = mm[part]; !ok {
			return nil, false
		}
	}
	return cur, true
}
