// Copyright 2024-2026 The Parca Authors
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

	pprofpb "github.com/parca-dev/parca/gen/proto/go/google/pprof"
	"github.com/parca-dev/parca/pkg/profile"
)

// Interpreter frames (e.g. Python) arrive already symbolized by the agent:
// the encoded location carries line and function info that must survive the
// round-trip through decodeLineInfo into the ClickHouse columns.
func TestDecodeLineInfoAgentSymbolizedFrame(t *testing.T) {
	stringTable := []string{"", "build-id-123", "python3.11", "handle_request", "handle_request_system", "app/server.py"}
	encoded := profile.EncodePprofLocation(
		&pprofpb.Location{
			Address: 0x14e,
			Line: []*pprofpb.Line{
				{FunctionId: 1, Line: 42},
			},
		},
		&pprofpb.Mapping{
			BuildId:     1,
			Filename:    2,
			MemoryStart: 0x1000,
			MemoryLimit: 0x2000,
			FileOffset:  0x10,
		},
		[]*pprofpb.Function{
			{Name: 3, SystemName: 4, Filename: 5, StartLine: 40},
		},
		stringTable,
	)

	info := decodeLineInfo(encoded)
	require.Equal(t, int64(42), info.LineNumber)
	require.Equal(t, int64(40), info.FunctionStartLine)
	require.Equal(t, "handle_request", info.FunctionName)
	require.Equal(t, "handle_request_system", info.FunctionSystemName)
	require.Equal(t, "app/server.py", info.FunctionFilename)
}

// Synthetic interpreter mappings have no mapping info at all; the decoder must
// still find the line/function section.
func TestDecodeLineInfoNoMapping(t *testing.T) {
	stringTable := []string{"", "decode", "decode_system", "app/importer.py"}
	encoded := profile.EncodePprofLocation(
		&pprofpb.Location{
			Address: 0x47,
			Line: []*pprofpb.Line{
				{FunctionId: 1, Line: 7},
			},
		},
		nil,
		[]*pprofpb.Function{
			{Name: 1, SystemName: 2, Filename: 3, StartLine: 5},
		},
		stringTable,
	)

	info := decodeLineInfo(encoded)
	require.Equal(t, int64(7), info.LineNumber)
	require.Equal(t, "decode", info.FunctionName)
	require.Equal(t, "app/importer.py", info.FunctionFilename)
}

// Native frames are sent unsymbolized (no lines); nothing should be decoded.
func TestDecodeLineInfoUnsymbolizedFrame(t *testing.T) {
	stringTable := []string{"", "build-id-456", "libc.so.6"}
	encoded := profile.EncodePprofLocation(
		&pprofpb.Location{Address: 0xdeadbeef},
		&pprofpb.Mapping{
			BuildId:     1,
			Filename:    2,
			MemoryStart: 0x1000,
			MemoryLimit: 0x2000,
		},
		nil,
		stringTable,
	)

	info := decodeLineInfo(encoded)
	require.Equal(t, LineInfo{}, info)
}
