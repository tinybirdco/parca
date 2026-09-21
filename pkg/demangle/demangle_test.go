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

package demangle

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewDemanglerForMode(t *testing.T) {
	mangled := []byte("_ZNSt6vectorIiE9push_backERKi")

	for mode, want := range map[string]string{
		"":          "std::vector::push_back",
		"simple":    "std::vector::push_back",
		"templates": "std::vector<int>::push_back",
		"full":      "std::vector<int>::push_back(int const&)",
	} {
		d, ok, err := NewDemanglerForMode(mode)
		require.NoError(t, err, mode)
		require.True(t, ok, mode)
		require.Equal(t, want, d.Demangle(mangled), mode)
	}

	_, ok, err := NewDemanglerForMode("none")
	require.NoError(t, err)
	require.False(t, ok)

	_, _, err = NewDemanglerForMode("bogus")
	require.Error(t, err)
}
