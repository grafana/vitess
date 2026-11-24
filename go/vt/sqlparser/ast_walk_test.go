package sqlparser

import "testing"

func TestWalkVisitsStatementFields(t *testing.T) {
	column := &ColName{Name: NewColIdent("target")}
	cteQuery := &Select{SelectExprs: SelectExprs{&AliasedExpr{Expr: column}}}
	cte := &CommonTableExpr{AliasedTableExpr: &AliasedTableExpr{Expr: &Subquery{Select: cteQuery}}}
	for _, tt := range []struct {
		name string
		node SQLNode
	}{
		{"SELECT window", &Select{Window: Window{&WindowDef{PartitionBy: Exprs{column}}}}},
		{"set operation CTE", &SetOp{With: &With{Ctes: []*CommonTableExpr{cte}}}},
		{"set operation order", &SetOp{OrderBy: OrderBy{&Order{Expr: column}}}},
		{"set operation limit", &SetOp{Limit: &Limit{Rowcount: column}}},
		{"INSERT returning", &Insert{Returning: SelectExprs{&AliasedExpr{Expr: column}}}},
		{"SUBSTRING literal from", &SubstrExpr{StrVal: NewStrVal([]byte("abc")), From: column}},
		{"SUBSTRING literal length", &SubstrExpr{StrVal: NewStrVal([]byte("abc")), From: NewIntVal([]byte("1")), To: column}},
		{"SUBSTRING column from", &SubstrExpr{Name: &ColName{Name: NewColIdent("source")}, From: column}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			visits := 0
			if err := Walk(func(node SQLNode) (bool, error) {
				if node == column {
					visits++
				}
				return true, nil
			}, tt.node); err != nil {
				t.Fatal(err)
			}
			if visits != 1 {
				t.Fatalf("visited column %d times, want 1", visits)
			}
		})
	}
}
