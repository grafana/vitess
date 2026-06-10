/*
Copyright 2019 The Vitess Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sqlparser

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func tableHintsFromQuery(t *testing.T, query string) []TableHint {
	t.Helper()

	stmt, err := Parse(query)
	require.NoError(t, err)

	sel, ok := stmt.(*Select)
	require.True(t, ok, "expected SELECT, got %T", stmt)

	at, ok := sel.From[0].(*AliasedTableExpr)
	require.True(t, ok, "expected AliasedTableExpr, got %T", sel.From[0])
	require.NotNil(t, at.TableHints)

	return at.TableHints.Hints
}

func TestParseTableHints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		query string
		want  []TableHint
	}{
		{
			name:  "rate and instant",
			query: "SELECT 1 FROM db.t FOR (rate('1m'), instant)",
			want: []TableHint{
				{Name: "rate", Value: "1m"},
				{Name: "instant"},
			},
		},
		{
			name:  "empty rate parens and step",
			query: "SELECT 1 FROM db.t FOR (rate(), step('30s'))",
			want: []TableHint{
				{Name: "rate", EmptyParens: true},
				{Name: "step", Value: "30s"},
			},
		},
		{
			name:  "numeric exemplars",
			query: "SELECT 1 FROM db.t FOR (exemplars(10))",
			want: []TableHint{
				{Name: "exemplars", Value: "10", NumericArg: true},
			},
		},
		{
			name:  "parser hint",
			query: "SELECT 1 FROM db.t FOR (parser('json'))",
			want: []TableHint{
				{Name: "parser", Value: "json"},
			},
		},
		{
			name:  "loki qualified table before limit",
			query: "SELECT timestamp, line FROM `loki::uid`.`grafana` FOR (rate('1m'), instant) LIMIT 10",
			want: []TableHint{
				{Name: "rate", Value: "1m"},
				{Name: "instant"},
			},
		},
		{
			name:  "tempo rate and step before group by",
			query: "SELECT timestamp, count(value) FROM `tempo::uid`.span_metrics FOR (rate(), step('30s')) GROUP BY timestamp LIMIT 15",
			want: []TableHint{
				{Name: "rate", EmptyParens: true},
				{Name: "step", Value: "30s"},
			},
		},
		{
			name:  "uppercase hint names",
			query: "SELECT 1 FROM db.t FOR (RATE('5m'), INSTANT)",
			want: []TableHint{
				{Name: "RATE", Value: "5m"},
				{Name: "INSTANT"},
			},
		},
		{
			name:  "parser with rate",
			query: "SELECT 1 FROM db.t FOR (parser('json'), rate('5m'))",
			want: []TableHint{
				{Name: "parser", Value: "json"},
				{Name: "rate", Value: "5m"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, tableHintsFromQuery(t, tc.query))
		})
	}
}

func TestParseTableHints_absent(t *testing.T) {
	t.Parallel()

	tests := []string{
		"SELECT * FROM db.tbl",
		"SELECT 1 FROM db.t WHERE msg = 'for (oops)'",
	}

	for _, query := range tests {
		t.Run(query, func(t *testing.T) {
			t.Parallel()

			stmt, err := Parse(query)
			require.NoError(t, err)

			sel := stmt.(*Select)
			at := sel.From[0].(*AliasedTableExpr)
			require.Nil(t, at.TableHints)
		})
	}
}

func TestFormatTableHintsRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		query     string
		formatted string
		want      []TableHint
	}{
		{
			name:      "rate and instant",
			query:     "SELECT 1 FROM db.t FOR (rate('1m'), instant)",
			formatted: "select 1 from db.t for (rate('1m'), instant)",
			want: []TableHint{
				{Name: "rate", Value: "1m"},
				{Name: "instant"},
			},
		},
		{
			name:      "empty rate parens before group by",
			query:     "SELECT 1 FROM db.t FOR (rate(), step('30s')) GROUP BY timestamp",
			formatted: "select 1 from db.t for (rate(), step('30s')) group by `timestamp`",
			want: []TableHint{
				{Name: "rate", EmptyParens: true},
				{Name: "step", Value: "30s"},
			},
		},
		{
			name:      "numeric exemplars",
			query:     "SELECT 1 FROM db.t FOR (exemplars(10))",
			formatted: "select 1 from db.t for (exemplars(10))",
			want: []TableHint{
				{Name: "exemplars", Value: "10", NumericArg: true},
			},
		},
		{
			name:      "parser with rate",
			query:     "SELECT 1 FROM db.t FOR (parser('json'), rate('5m'))",
			formatted: "select 1 from db.t for (parser('json'), rate('5m'))",
			want: []TableHint{
				{Name: "parser", Value: "json"},
				{Name: "rate", Value: "5m"},
			},
		},
		{
			name:      "loki qualified table",
			query:     "SELECT timestamp, line FROM `loki::uid`.`grafana` FOR (rate('1m'), instant) LIMIT 10",
			formatted: "select `timestamp`, line from `loki::uid`.grafana for (rate('1m'), instant) limit 10",
			want: []TableHint{
				{Name: "rate", Value: "1m"},
				{Name: "instant"},
			},
		},
		{
			name:      "tempo metrics query",
			query:     "SELECT timestamp, count(value) FROM `tempo::uid`.span_metrics FOR (rate(), step('30s')) GROUP BY timestamp LIMIT 15",
			formatted: "select `timestamp`, count(`value`) from `tempo::uid`.span_metrics for (rate(), step('30s')) group by `timestamp` limit 15",
			want: []TableHint{
				{Name: "rate", EmptyParens: true},
				{Name: "step", Value: "30s"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			stmt, err := Parse(tc.query)
			require.NoError(t, err)

			formatted := String(stmt)
			require.Equal(t, tc.formatted, formatted)

			stmt2, err := Parse(formatted)
			require.NoError(t, err)

			got := tableHintsFromQuery(t, formatted)
			require.Equal(t, tc.want, got, "hints must survive format round-trip")

			// Sanity: second parse should produce identical formatted SQL.
			require.Equal(t, formatted, String(stmt2))
		})
	}
}
