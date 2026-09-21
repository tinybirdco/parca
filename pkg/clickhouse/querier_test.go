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
	"context"
	"os"
	"testing"
	"time"

	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/parca-dev/parca/pkg/demangle"
	"github.com/parca-dev/parca/pkg/profile"
)

func TestSampleDataHasStoredFunction(t *testing.T) {
	require.True(t, (sampleData{functionSystemNames: []string{"local_function"}}).hasStoredFunction(0))
	require.True(t, (sampleData{functionNames: []string{"function"}}).hasStoredFunction(0))
	require.False(t, (sampleData{functionNames: []string{""}, functionSystemNames: []string{""}}).hasStoredFunction(0))
}

func TestDisplayFunctionName(t *testing.T) {
	q := &Querier{demangler: demangle.MustNewDefaultDemangler()}

	// parca-agent v2 rows: function_name empty, system_name populated.
	require.Equal(t, "run_until_complete", q.displayFunctionName(sampleData{
		functionNames:       []string{""},
		functionSystemNames: []string{"run_until_complete"},
	}, 0))

	// Mangled system names are demangled, like the FrostDB path.
	require.Equal(t, "TB::RowBinaryEncoder::convert", q.displayFunctionName(sampleData{
		functionNames:       []string{""},
		functionSystemNames: []string{"_ZN2TB16RowBinaryEncoder7convertEv"},
	}, 0))

	// v1 rows keep working when no system name is stored.
	require.Equal(t, "plain", q.displayFunctionName(sampleData{
		functionNames: []string{"plain"},
	}, 0))

	// An already-demangled stored name wins over the mangled system name and
	// is returned untouched, even when it is more detailed than the default
	// demangle mode would produce.
	require.Equal(t, "TB::RowBinaryEncoder::convert(int const&)", q.displayFunctionName(sampleData{
		functionNames:       []string{"TB::RowBinaryEncoder::convert(int const&)"},
		functionSystemNames: []string{"_ZN2TB16RowBinaryEncoder7convertERKi"},
	}, 0))

	// A mangled stored name is demangled, exactly as profile.DecodeInto does.
	require.Equal(t, "TB::RowBinaryEncoder::convert", q.displayFunctionName(sampleData{
		functionNames:       []string{"_ZN2TB16RowBinaryEncoder7convertEv"},
		functionSystemNames: []string{"_ZN2TB16RowBinaryEncoder7convertEv"},
	}, 0))

	// Without a demangler the raw system name is still better than nothing.
	noDemangler := &Querier{}
	require.Equal(t, "sys", noDemangler.displayFunctionName(sampleData{
		functionNames:       []string{""},
		functionSystemNames: []string{"sys"},
	}, 0))
}

func TestLabelsFromJSON(t *testing.T) {
	labels, err := labelsFromJSON(`{"job":"api","k8s":{"pod":"web-1","node":{"name":"n1"}},"replica":3,"gone":null}`)
	require.NoError(t, err)

	got := map[string]string{}
	for _, l := range labels {
		got[l.Name] = l.Value
	}
	require.Equal(t, map[string]string{
		"job":           "api",
		"k8s.node.name": "n1",
		"k8s.pod":       "web-1",
		"replica":       "3",
	}, got)
	require.Equal(t, "job", labels[0].Name, "labels are sorted by name")

	_, err = labelsFromJSON("not json")
	require.Error(t, err)
}

func TestClickHouseV2FunctionNameIntegration(t *testing.T) {
	address := os.Getenv("PARCA_TEST_CLICKHOUSE_ADDRESS")
	if address == "" {
		t.Skip("PARCA_TEST_CLICKHOUSE_ADDRESS is not set")
	}

	ctx := context.Background()
	client, err := NewClient(ctx, Config{
		Address:     address,
		Database:    "parca_test",
		Table:       "stacktraces_v2",
		Compression: "none",
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	require.NoError(t, client.EnsureSchema(ctx))
	t.Cleanup(func() { require.NoError(t, client.Exec(ctx, "DROP TABLE IF EXISTS parca_test.stacktraces_v2")) })

	ts := time.Unix(1700000000, 0)
	batch, err := client.PrepareBatch(ctx, InsertSQL(client.Database(), client.Table()))
	require.NoError(t, err)
	require.NoError(t, batch.Append(
		"process_cpu", "samples", "count", "cpu", "nanoseconds",
		int64(1), int64(0), ts.UnixMilli(), ts.UnixNano(), int64(1), map[string]string{},
		[]uint64{0}, []uint64{0}, []uint64{0}, []uint64{0}, []string{""}, []string{""},
		[]int64{1}, []string{""}, []string{"_ZN2TB16RowBinaryEncoder7convertEv"}, []string{"row_binary.cpp"}, []int64{1},
	))
	require.NoError(t, batch.Send())

	q := NewQuerier(
		client,
		log.NewNopLogger(),
		noop.NewTracerProvider().Tracer("test"),
		memory.NewGoAllocator(),
		nil,
		demangle.MustNewDefaultDemangler(),
	)
	result, err := q.QuerySingle(ctx, "process_cpu:samples:count:cpu:nanoseconds{}", ts, false)
	require.NoError(t, err)
	for _, record := range result.Samples {
		defer record.Release()
	}

	reader, err := profile.NewReader(result)
	require.NoError(t, err)
	require.Len(t, reader.RecordReaders, 1)
	r := reader.RecordReaders[0]
	require.Equal(t, "TB::RowBinaryEncoder::convert", string(r.LineFunctionNameDict.Value(int(r.LineFunctionNameIndices.Value(0)))))
}
