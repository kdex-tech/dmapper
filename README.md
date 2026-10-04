# dmapper

A Go library for declarative data mapping using CEL (Common Expression Language).

## Features

- Declarative mapping rules
- CEL expressions for data transformation
- Required/optional rule handling
- Nested path support for target properties

## Installation

```bash
go get github.com/kdex-tech/dmapper
```

## Usage

```go
package main

import (
	"fmt"

	"github.com/kdex-tech/dmapper"
)

func main() {
	// Define mapping rules
	rules := []dmapper.MappingRule{
		{
			SourceExpression: "self.oidc.groups.filter(g, g.startsWith('app_'))",
			TargetPropPath:   "auth.internal_groups",
		},
		{
			SourceExpression: "self.oidc.name",
			TargetPropPath:   "name",
			Required:         true,
		},
	}

	// Create a new mapper
	m, err := dmapper.NewMapper(rules)
	if err != nil {
		fmt.Println("Error creating mapper:", err)
		return
	}

	// Input data
	input := map[string]any{
		"oidc": map[string]any{
			"name":   "John Doe",
			"groups": []string{"app_group1", "app_group2", "other_group"},
		},
	}

	// Execute mapping
	result, err := m.Execute(input)
	if err != nil {
		fmt.Println("Error executing mapping:", err)
		return
	}

	fmt.Println("Result:", result)
}
```

## MappingRule

The `MappingRule` struct defines a single mapping rule:

```go
type MappingRule struct {
	// Merge controls how the result combines with a value already at
	// TargetPropPath: Accumulate (default) or Replace.
	Merge MergeStrategy `json:"merge,omitempty"`

	// Required indicates that if the rule fails to produce a value the rule
	// will be skipped. Otherwise the execution should fail.
	Required bool `json:"required"`

	// SourceExpression is CEL program to compute a transformation of input into
	// a new form.
	SourceExpression string `json:"sourceExpression"`

	// TargetPropPath is a dot-separated property path for where the result
	// should be attached in the output.
	TargetPropPath string `json:"targetPropPath"`
}
```

### Fields

- **Merge**: `Accumulate` (the default) or `Replace`. See [Merge semantics](#merge-semantics).
- **Required**: If `true`, the mapper will return an error if this rule fails. If `false`, the rule will be skipped and execution will continue.
- **SourceExpression**: A CEL expression that operates on the input data. The input is available as the `self` variable.
- **TargetPropPath**: A dot-separated path indicating where the result should be placed in the output map.

## Merge semantics

Rules apply in order, and each rule sees earlier rules' output in `self`. A
rule's result is then combined with whatever is already at its
`targetPropPath`, according to `merge`:

- `Accumulate` (default): when both the existing value and the result are
  lists, the result is the existing list followed by the result's items that
  are not already present. Restating `self.<target>` is harmless. Any other
  shape pair replaces.
- `Replace`: the result always replaces. Use it for any rule meant to filter,
  transform, normalise or override a list already at its own target.

With `entitlements: [static:a]` and `extra_grants: [extra:b]`:

| Rules (all target `entitlements`) | Result |
|---|---|
| `self.extra_grants` | `[static:a extra:b]` |
| `self.entitlements + self.extra_grants` | `[static:a extra:b]` |
| `self.entitlements`, then `self.extra_grants` | `[static:a extra:b]` |

Under the default, values already at the target in the input stay in the
output, including claims asserted by an identity provider. So a rule that
reads `self.<target>` and writes `<target>` back changed must opt out:

- a filter (`groups` → only `app_*` groups) would remove nothing;
- a transform (`roles.map(r, r.lowerAscii())`, prefixing) would keep the
  originals beside the transformed values, and grows on every re-run;
- an override (`self.mapped_roles` → `roles`) would keep the roles it meant to
  replace.

(`self` is typed `map(string, any)`, so a top-level value needs `dyn()` before
a comprehension.)

```yaml
- sourceExpression: dyn(self.roles).filter(r, r != 'guest')
  targetPropPath: roles
  merge: Replace
```

Before v0.2.0 every rule replaced its target.

## CEL Support

This library uses Google's Common Expression Language (CEL) for data transformation. CEL supports:

- **Variables**: `self` (the input data)
- **Functions**: `filter()`, `size()`, `startsWith()`, etc.
- **Operators**: Standard arithmetic, comparison, and logical operators
- **Type System**: Strong typing with automatic type conversion

See the [CEL documentation](https://github.com/google/cel-spec) for complete CEL language details.

## Error Handling

- **Compilation Errors**: Errors during mapper compilation (e.g., invalid CEL expressions) will be returned by `NewMapper`.
- **Execution Errors**: Errors during expression evaluation will be handled based on the `Required` field:
  - If `Required` is `true` and evaluation fails, `Execute` will return an error.
  - If `Required` is `false` and evaluation fails, the rule will be skipped and `Execute` will continue with the next rule.

## Testing

To run the tests:

```bash
go test
```

## License

This project is licensed under the Apache 2.0 License - see the [LICENSE](LICENSE) file for details.
