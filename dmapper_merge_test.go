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
