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

package testutil

import (
	"context"
	"fmt"
	"hash/fnv"
	"os"
	"testing"
	"time"

	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/go-kit/log"
	"github.com/polarsignals/frostdb"
	frostquery "github.com/polarsignals/frostdb/query"
	"go.opentelemetry.io/otel/trace/noop"

	pb "github.com/parca-dev/parca/gen/proto/go/parca/query/v1alpha1"
	"github.com/parca-dev/parca/pkg/clickhouse"
	"github.com/parca-dev/parca/pkg/demangle"
	"github.com/parca-dev/parca/pkg/ingester"
	"github.com/parca-dev/parca/pkg/parcacol"
	"github.com/parca-dev/parca/pkg/profile"
	"github.com/parca-dev/parca/pkg/symbolizer"
)

type Querier interface {
	Labels(context.Context, []string, time.Time, time.Time, string) ([]string, error)
	Values(context.Context, string, []string, time.Time, time.Time, string) ([]string, error)
	QueryRange(context.Context, string, time.Time, time.Time, time.Duration, uint32, []string) ([]*pb.MetricsSeries, error)
	ProfileTypes(context.Context, time.Time, time.Time) ([]*pb.ProfileType, error)
	QuerySingle(context.Context, string, time.Time, bool) (profile.Profile, error)
	QueryMerge(context.Context, string, time.Time, time.Time, []string, bool, string) (profile.Profile, error)
	GetProfileMetadataMappings(context.Context, string, time.Time, time.Time) ([]string, error)
	GetProfileMetadataLabels(context.Context, string, time.Time, time.Time) ([]string, error)
	HasProfileData(context.Context) (bool, error)
}

type Storage struct {
	Ingester    ingester.Ingester
	Querier     Querier
	ensureReady func() error
}

func (s *Storage) EnsureReady(t *testing.T) {
	t.Helper()
	if err := s.ensureReady(); err != nil {
		t.Fatal(err)
	}
}

func NewStorage(t *testing.T, allocator memory.Allocator) *Storage {
	t.Helper()
	switch backend := os.Getenv("PARCA_TEST_STORAGE"); backend {
	case "", "frostdb":
		return newFrostDBStorage(t, allocator)
	case "clickhouse":
		return newClickHouseStorage(t, allocator)
	default:
		t.Fatalf("unsupported PARCA_TEST_STORAGE %q", backend)
		return nil
	}
}

func newFrostDBStorage(t *testing.T, allocator memory.Allocator) *Storage {
	col, err := frostdb.New()
	if err != nil {
		t.Fatal(err)
	}
	db, err := col.DB(context.Background(), "parca")
	if err != nil {
		t.Fatal(err)
	}
	table, err := db.Table("stacktraces", frostdb.NewTableConfig(profile.SchemaDefinition()))
	if err != nil {
		t.Fatal(err)
	}
	logger := log.NewNopLogger()
	tracer := noop.NewTracerProvider().Tracer("")
	return &Storage{
		Ingester: ingester.NewIngester(logger, table),
		Querier: parcacol.NewQuerier(
			logger,
			tracer,
			frostquery.NewEngine(allocator, db.TableProvider()),
			"stacktraces",
			nil,
			nil,
			allocator,
		),
		ensureReady: table.EnsureCompaction,
	}
}

type nopSymbolizer struct{}

func (nopSymbolizer) Symbolize(context.Context, symbolizer.SymbolizationRequest) error { return nil }

func newClickHouseStorage(t *testing.T, allocator memory.Allocator) *Storage {
	address := os.Getenv("PARCA_TEST_CLICKHOUSE_ADDRESS")
	if address == "" {
		t.Fatal("PARCA_TEST_CLICKHOUSE_ADDRESS must be set when PARCA_TEST_STORAGE=clickhouse")
	}

	h := fnv.New64a()
	_, _ = fmt.Fprintf(h, "%s-%d", t.Name(), os.Getpid())
	table := fmt.Sprintf("stacktraces_%x", h.Sum64())
	ctx := context.Background()
	client, err := clickhouse.NewClient(ctx, clickhouse.Config{
		Address:     address,
		Database:    "parca_test",
		Table:       table,
		Compression: "none",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = client.Exec(context.Background(), "DROP TABLE IF EXISTS "+client.FullTableName())
		_ = client.Close()
	})

	logger := log.NewNopLogger()
	tracer := noop.NewTracerProvider().Tracer("")
	return &Storage{
		Ingester: clickhouse.NewIngester(logger, client),
		Querier: clickhouse.NewQuerier(
			client,
			logger,
			tracer,
			allocator,
			nopSymbolizer{},
			demangle.MustNewDefaultDemangler(),
		),
		ensureReady: func() error { return nil },
	}
}
