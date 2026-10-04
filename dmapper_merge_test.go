package dmapper_test

import (
	"testing"

	"github.com/kdex-tech/dmapper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewMapper_MergeStrategyValidation pins that only the empty value,
// Accumulate and Replace compile; anything else is an error, never a silent
// fallback. See kdex-tech/host-manager#229.
func TestNewMapper_MergeStrategyValidation(t *testing.T) {
	for _, ok := range []dmapper.MergeStrategy{"", dmapper.MergeAccumulate, dmapper.MergeReplace} {
		_, err := dmapper.NewMapper([]dmapper.MappingRule{{SourceExpression: "self.a", TargetPropPath: "b", Merge: ok}})
		assert.NoErrorf(t, err, "merge %q must compile", ok)
	}
	_, err := dmapper.NewMapper([]dmapper.MappingRule{{SourceExpression: "self.a", TargetPropPath: "b", Merge: "append"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"append"`)
}

// TestMapper_Execute_AccumulateIssue229 pins the three rows of
// kdex-tech/host-manager#229: every natural way to write "also add these"
// keeps the existing list.
func TestMapper_Execute_AccumulateIssue229(t *testing.T) {
	input := map[string]any{
		"entitlements": []string{"static:a"},
		"extra_grants": []string{"extra:b"},
	}
	tests := []struct {
		name  string
		rules []dmapper.MappingRule
	}{
		{"A: rule omits self.entitlements", []dmapper.MappingRule{
			{SourceExpression: "self.extra_grants", TargetPropPath: "entitlements"},
		}},
		{"B: rule restates self.entitlements", []dmapper.MappingRule{
			{SourceExpression: "self.entitlements + self.extra_grants", TargetPropPath: "entitlements"},
		}},
		{"C: identity rule then omitting rule", []dmapper.MappingRule{
			{SourceExpression: "self.entitlements", TargetPropPath: "entitlements"},
			{SourceExpression: "self.extra_grants", TargetPropPath: "entitlements"},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := dmapper.NewMapper(tt.rules)
			require.NoError(t, err)
			got, err := m.Execute(input)
			require.NoError(t, err)
			assert.Equal(t, []string{"static:a", "extra:b"}, got["entitlements"])
		})
	}
	assert.Equal(t, []string{"static:a"}, input["entitlements"], "input must not be mutated")
}

// TestMapper_Execute_AccumulateIsIdempotent pins that re-running the mapper
// over its own merged output adds nothing (host-manager enriches the auth
// context, then Project runs the same mapper over the enriched context).
func TestMapper_Execute_AccumulateIsIdempotent(t *testing.T) {
	m, err := dmapper.NewMapper([]dmapper.MappingRule{
		{SourceExpression: "self.roles + self.extra_roles", TargetPropPath: "roles"},
	})
	require.NoError(t, err)
	in := map[string]any{"roles": []any{"admin"}, "extra_roles": []any{"auditor"}}
	first, err := m.Execute(in)
	require.NoError(t, err)
	assert.Equal(t, []string{"admin", "auditor"}, first["roles"])

	in["roles"] = first["roles"]
	second, err := m.Execute(in)
	require.NoError(t, err)
	assert.Equal(t, []string{"admin", "auditor"}, second["roles"])
}

// TestMapper_Execute_ReplaceNarrows pins the opt-out: a filtering rule only
// narrows with merge: Replace; under the default it cannot remove items.
func TestMapper_Execute_ReplaceNarrows(t *testing.T) {
	in := map[string]any{"roles": []string{"keep", "drop"}}
	filter := "dyn(self.roles).filter(r, r != 'drop')"

	acc, err := dmapper.NewMapper([]dmapper.MappingRule{{SourceExpression: filter, TargetPropPath: "roles"}})
	require.NoError(t, err)
	got, err := acc.Execute(in)
	require.NoError(t, err)
	assert.Equal(t, []string{"keep", "drop"}, got["roles"], "Accumulate cannot narrow")

	rep, err := dmapper.NewMapper([]dmapper.MappingRule{{SourceExpression: filter, TargetPropPath: "roles", Merge: dmapper.MergeReplace}})
	require.NoError(t, err)
	got, err = rep.Execute(in)
	require.NoError(t, err)
	assert.Equal(t, []string{"keep"}, got["roles"])
}

// TestMapper_Execute_AccumulateShapes pins mixed []any/[]string inputs and
// non-string elements.
func TestMapper_Execute_AccumulateShapes(t *testing.T) {
	m, err := dmapper.NewMapper([]dmapper.MappingRule{{SourceExpression: "self.extra", TargetPropPath: "list"}})
	require.NoError(t, err)

	got, err := m.Execute(map[string]any{"list": []any{"a", "b"}, "extra": []string{"b", "c", "c"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b", "c"}, got["list"], "all-string union narrows to []string; result-internal dupes collapse")

	got, err = m.Execute(map[string]any{"list": []string{"a", "a"}, "extra": []string{"b"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "a", "b"}, got["list"], "existing list's own duplicates are preserved")

	got, err = m.Execute(map[string]any{"list": []any{int64(1)}, "extra": []any{int64(1), int64(2)}})
	require.NoError(t, err)
	assert.Equal(t, []any{int64(1), int64(2)}, got["list"], "non-string elements compare by deep equality; mixed result stays []any")
}

// TestMapper_Execute_AccumulateNestedPath pins that a dotted target accumulates.
func TestMapper_Execute_AccumulateNestedPath(t *testing.T) {
	m, err := dmapper.NewMapper([]dmapper.MappingRule{{SourceExpression: "self.more", TargetPropPath: "auth.groups"}})
	require.NoError(t, err)
	got, err := m.Execute(map[string]any{
		"auth": map[string]any{"groups": []string{"g1"}},
		"more": []string{"g2"},
	})
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"groups": []string{"g1", "g2"}}, got["auth"])
}

// TestMapper_Execute_NonListReplaces pins that scalars, maps and mismatched
// shapes still replace.
func TestMapper_Execute_NonListReplaces(t *testing.T) {
	tests := []struct {
		name string
		expr string
		in   any
		want any
	}{
		{"scalar onto scalar", "'pro'", "free", "pro"},
		{"scalar onto list", "'solo'", []string{"a"}, "solo"},
		{"list onto scalar", "['a']", "solo", []string{"a"}},
		{"map onto map", "{'k': 'new'}", map[string]any{"k": "old", "other": "x"}, map[string]any{"k": "new"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := dmapper.NewMapper([]dmapper.MappingRule{{SourceExpression: tt.expr, TargetPropPath: "t"}})
			require.NoError(t, err)
			got, err := m.Execute(map[string]any{"t": tt.in})
			require.NoError(t, err)
			assert.Equal(t, tt.want, got["t"])
		})
	}
}
