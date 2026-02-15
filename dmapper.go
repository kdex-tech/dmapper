package dmapper

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/google/cel-go/cel"
)

// Mapper is a collection of compiledMappingRules.
type Mapper struct {
	CompiledMappingRules []compiledMappingRule
}

// MappingRule defines a transformation rule for mapping arbitrary data from an
// input to an output.
type MappingRule struct {
	// required indicates that if the rule fails to produce a value the rule
	// will be skipped. Otherwise the execution should fail.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=false
	Required bool `json:"required"`

	// sourceExpression is CEL program to compute a transformation of input into
	// a new form.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=5
	// +kubebuilder:example:=`self.oidc.groups.filter(g, g.startsWith('app_')).join(',')`
	SourceExpression string `json:"sourceExpression"`

	// targetPropPath is a dot-separated property path for where the result
	// should be attached in the output.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=5
	// +kubebuilder:example:=`auth.internal_groups`
	TargetPropPath string `json:"targetPropPath"`
}

// compiledMappingRule is a compiled MappingRule.
type compiledMappingRule struct {
	MappingRule
	Program cel.Program
}

func (m *Mapper) Execute(input map[string]any) (map[string]any, error) {
	resultClaims := make(map[string]any)

	data := map[string]any{
		"self": input,
	}

	for _, rule := range m.CompiledMappingRules {
		out, _, err := rule.Program.Eval(data)
		if err != nil {
			if !rule.Required {
				continue
			}

			return nil, fmt.Errorf("failed to eval expression %q: %w", rule.SourceExpression, err)
		}

		var val any
		// Try to convert to common native Go types. CEL's ConvertToNative is more reliable
		// than Value() for obtaining specific Go types like []string or map[string]any.
		for _, t := range []reflect.Type{
			reflect.TypeFor[[]string](),
			reflect.TypeFor[map[string]any](),
			reflect.TypeFor[string](),
			reflect.TypeFor[int64](),
			reflect.TypeFor[float64](),
			reflect.TypeFor[bool](),
			reflect.TypeFor[[]any](),
		} {
			if v, err := out.ConvertToNative(t); err == nil {
				val = v
				break
			}
		}

		if val == nil {
			val = out.Value()
		}

		if err := setNestedPath(resultClaims, rule.TargetPropPath, val); err != nil {
			if !rule.Required {
				continue
			}

			return nil, err
		}
	}

	return resultClaims, nil
}

func NewMapper(rules []MappingRule) (*Mapper, error) {
	cm, err := compileMappers(rules)
	if err != nil {
		return nil, err
	}
	return &Mapper{CompiledMappingRules: cm}, nil
}

func compileMappers(rules []MappingRule) ([]compiledMappingRule, error) {
	cm := []compiledMappingRule{}

	env, _ := cel.NewEnv(cel.Variable("self", cel.MapType(cel.StringType, cel.AnyType)))

	for _, rule := range rules {
		ast, issues := env.Compile(rule.SourceExpression)
		if issues.Err() != nil {
			return nil, issues.Err()
		}
		prog, err := env.Program(ast)
		if err != nil {
			return nil, err
		}
		cm = append(cm, compiledMappingRule{
			MappingRule: rule,
			Program:     prog,
		})
	}

	return cm, nil
}

func setNestedPath(input map[string]any, path string, value any) error {
	parts := strings.Split(path, ".")
	current := input

	for i, part := range parts {
		if i == len(parts)-1 {
			current[part] = value
			return nil
		}

		if _, exists := current[part]; !exists {
			current[part] = make(map[string]any)
		}

		next, ok := current[part].(map[string]any)
		if !ok {
			return fmt.Errorf("path conflict at %s", part)
		}
		current = next
	}
	return nil
}
