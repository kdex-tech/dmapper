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
