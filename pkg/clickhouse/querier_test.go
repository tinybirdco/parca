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
)

func TestSampleDataHasStoredFunction(t *testing.T) {
	require.True(t, (sampleData{functionSystemNames: []string{"local_function"}}).hasStoredFunction(0))
	require.True(t, (sampleData{functionNames: []string{"function"}}).hasStoredFunction(0))
	require.False(t, (sampleData{functionNames: []string{""}, functionSystemNames: []string{""}}).hasStoredFunction(0))
}
