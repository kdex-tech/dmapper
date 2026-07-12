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
	// +kubebuilder:validation:MinLength=2
	// +kubebuilder:example:=`self.oidc.groups.filter(g, g.startsWith('app_')).join(',')`
	SourceExpression string `json:"sourceExpression"`

	// targetPropPath is a dot-separated property path for where the result
	// should be attached in the output.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=2
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

	// self is the working context each rule evaluates against. It starts as a
	// map-deep copy of input (so the caller's input is never mutated) and
	// accumulates each rule's output, so a later rule SEES earlier rules'
	// contributions: additive rules on the same target accumulate instead of
	// clobbering (last-wins), and a rule can consume a prior rule's output. Rules
	// apply in order. See kdex-tech/dmapper#1.
	self, ok := cloneMaps(input).(map[string]any)
	if !ok || self == nil {
		self = map[string]any{}
	}
	data := map[string]any{
		"self": self,
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
		// Chain: reflect this rule's output into the working context so subsequent
		// rules can see it. Best-effort — if the target path conflicts with the
		// input's existing structure, the value still appears in the returned
		// result; it just isn't visible to later rules' self.
		_ = setNestedPath(self, rule.TargetPropPath, val)
	}

	return resultClaims, nil
}

// cloneMaps returns a copy of v with every nested map[string]any cloned, so the
// working context can be mutated (via setNestedPath) without touching the
// caller's input. Non-map values — including slices — are shared by reference;
// setNestedPath only creates/descends maps and replaces leaf values, so it never
// mutates a shared slice or scalar.
func cloneMaps(v any) any {
	m, ok := v.(map[string]any)
	if !ok {
		return v
	}
	out := make(map[string]any, len(m))
	for k, val := range m {
		out[k] = cloneMaps(val)
	}
	return out
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
