package dmapper_test

import (
	"testing"

	"github.com/kdex-tech/dmapper"
	"github.com/stretchr/testify/assert"
)

func TestMapper_Execute(t *testing.T) {
	tests := []struct {
		name    string
		rules   []dmapper.MappingRule
		input   map[string]any
		want    map[string]any
		wantErr bool
	}{
		{
			name: "simple mapping",
			rules: []dmapper.MappingRule{
				{
					SourceExpression: "self.oidc.groups.filter(g, g.startsWith('app_'))",
					TargetPropPath:   "auth.internal_groups",
				},
			},
			input: map[string]any{
				"oidc": map[string]any{
					"groups": []string{"app_group1", "app_group2", "other_group"},
				},
			},
			want: map[string]any{
				"auth": map[string]any{
					"internal_groups": []string{"app_group1", "app_group2"},
				},
			},
			wantErr: false,
		},
		{
			name: "identity",
			rules: []dmapper.MappingRule{
				{
					SourceExpression: "self.oidc",
					TargetPropPath:   "oidc",
				},
			},
			input: map[string]any{
				"oidc": map[string]any{
					"groups": []string{"app_group1", "app_group2", "other_group"},
				},
			},
			want: map[string]any{
				"oidc": map[string]any{
					"groups": []string{"app_group1", "app_group2", "other_group"},
				},
			},
			wantErr: false,
		},
		{
			name: "move groups to root",
			rules: []dmapper.MappingRule{
				{
					SourceExpression: "self.oidc.groups",
					TargetPropPath:   "groups",
				},
			},
			input: map[string]any{
				"oidc": map[string]any{
					"groups": []string{"app_group1", "app_group2", "other_group"},
				},
			},
			want: map[string]any{
				"groups": []string{"app_group1", "app_group2", "other_group"},
			},
			wantErr: false,
		},
		{
			name: "move groups to root with filter",
			rules: []dmapper.MappingRule{
				{
					SourceExpression: "self.oidc.groups.filter(g, g.startsWith('app_'))",
					TargetPropPath:   "groups",
				},
			},
			input: map[string]any{
				"oidc": map[string]any{
					"groups": []string{"app_group1", "app_group2", "other_group"},
				},
			},
			want: map[string]any{
				"groups": []string{"app_group1", "app_group2"},
			},
			wantErr: false,
		},
		{
			name: "compute a count of the groups",
			rules: []dmapper.MappingRule{
				{
					SourceExpression: "self.oidc.groups.filter(g, g.startsWith('app_')).size()",
					TargetPropPath:   "groups_count",
				},
			},
			input: map[string]any{
				"oidc": map[string]any{
					"groups": []string{"app_group1", "app_group2", "other_group"},
				},
			},
			want: map[string]any{
				"groups_count": int64(2),
			},
			wantErr: false,
		},
		{
			name: "failing but optional rule",
			rules: []dmapper.MappingRule{
				{
					SourceExpression: "self.oidc.name",
					TargetPropPath:   "name",
					Required:         false,
				},
				{
					SourceExpression: "self.oidc.groups[0]",
					TargetPropPath:   "first_group",
					Required:         true,
				},
			},
			input: map[string]any{
				"oidc": map[string]any{
					"groups": []string{"app_group1", "app_group2", "other_group"},
				},
			},
			want: map[string]any{
				"first_group": "app_group1",
			},
			wantErr: false,
		},
		{
			name: "failing but required rule",
			rules: []dmapper.MappingRule{
				{
					SourceExpression: "self.oidc.name",
					TargetPropPath:   "name",
					Required:         true,
				},
				{
					SourceExpression: "self.oidc.groups[0]",
					TargetPropPath:   "first_group",
					Required:         true,
				},
			},
			input: map[string]any{
				"oidc": map[string]any{
					"groups": []string{"app_group1", "app_group2", "other_group"},
				},
			},
			want:    map[string]any{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := dmapper.NewMapper(tt.rules)
			assert.NoError(t, err)
			got, gotErr := m.Execute(tt.input)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("Execute() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("Execute() succeeded unexpectedly")
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestMapper_Execute_ChainsAdditiveRulesOnSameTarget pins that rules chain: a
// later rule evaluates against earlier rules' outputs, so two additive rules on
// the same target ACCUMULATE instead of clobbering (last-wins). See #1.
func TestMapper_Execute_ChainsAdditiveRulesOnSameTarget(t *testing.T) {
	m, err := dmapper.NewMapper([]dmapper.MappingRule{
		{
			SourceExpression: `(has(self.entitlements) ? self.entitlements : []) + (has(self.a) ? self.a : [])`,
			TargetPropPath:   "entitlements",
		},
		{
			SourceExpression: `(has(self.entitlements) ? self.entitlements : []) + (has(self.b) ? self.b : [])`,
			TargetPropPath:   "entitlements",
		},
	})
	assert.NoError(t, err)

	input := map[string]any{
		"entitlements": []string{"base"},
		"a":            []string{"from-a"},
		"b":            []string{"from-b"},
	}
	got, err := m.Execute(input)
	assert.NoError(t, err)

	// Rule 2 must see rule 1's output for self.entitlements, so the result is
	// base + from-a + from-b, NOT base + from-b (which the pre-fix last-wins,
	// original-self behavior produced).
	assert.ElementsMatch(t, []string{"base", "from-a", "from-b"}, got["entitlements"])

	// Execute must not mutate the caller's input while chaining.
	assert.Equal(t, []string{"base"}, input["entitlements"], "input must not be mutated")
}

// TestMapper_Execute_ChainAcrossDifferentTargets pins that a rule can consume a
// prior rule's output written to a DIFFERENT target.
func TestMapper_Execute_ChainAcrossDifferentTargets(t *testing.T) {
	m, err := dmapper.NewMapper([]dmapper.MappingRule{
		{SourceExpression: `self.a + self.b`, TargetPropPath: "combined"},
		{SourceExpression: `self.combined + self.c`, TargetPropPath: "final"},
	})
	assert.NoError(t, err)

	got, err := m.Execute(map[string]any{
		"a": []string{"a1"},
		"b": []string{"b1"},
		"c": []string{"c1"},
	})
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"a1", "b1"}, got["combined"])
	assert.ElementsMatch(t, []string{"a1", "b1", "c1"}, got["final"],
		"the second rule must see the first rule's output at self.combined")
}
