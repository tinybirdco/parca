// Copyright 2026 The Parca Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package clickhouse

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/parca-dev/parca/pkg/demangle"
)

func TestSampleDataHasStoredFunction(t *testing.T) {
	require.True(t, (sampleData{functionSystemNames: []string{"local_function"}}).hasStoredFunction(0))
	require.True(t, (sampleData{functionNames: []string{"function"}}).hasStoredFunction(0))
	require.False(t, (sampleData{functionNames: []string{""}, functionSystemNames: []string{""}}).hasStoredFunction(0))
}

func TestDisplayFunctionNameUsesSystemName(t *testing.T) {
	q := &Querier{demangler: demangle.MustNewDefaultDemangler()}

	// parca-agent v2 rows: function_name empty, system_name populated.
	require.Equal(t, "run_until_complete", q.displayFunctionName(sampleData{
		functionNames:       []string{""},
		functionSystemNames: []string{"run_until_complete"},
	}, 0))

	// Mangled system names are demangled, like the FrostDB path.
	require.Equal(t, "TB::RowBinaryEncoder::convert()", q.displayFunctionName(sampleData{
		functionNames:       []string{""},
		functionSystemNames: []string{"_ZN2TB16RowBinaryEncoder7convertEv"},
	}, 0))

	// v1 rows keep working when no system name is stored.
	require.Equal(t, "plain", q.displayFunctionName(sampleData{
		functionNames: []string{"plain"},
	}, 0))

	// Without a demangler the raw system name is still better than nothing.
	noDemangler := &Querier{}
	require.Equal(t, "sys", noDemangler.displayFunctionName(sampleData{
		functionNames:       []string{""},
		functionSystemNames: []string{"sys"},
	}, 0))
}
