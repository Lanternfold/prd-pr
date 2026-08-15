package cost

import "testing"

func TestAppendAndSpent(t *testing.T) {
	root := t.TempDir()
	if err := Append(root, Line{Model: "cheap", Purpose: "review", EstimatedUSD: 0.01}); err != nil {
		t.Fatal(err)
	}
	if err := Append(root, Line{Model: "NONE", Purpose: "review"}); err != nil {
		t.Fatal(err)
	}
	if SpentUSD(root) != 0.01 {
		t.Fatalf("spent=%v", SpentUSD(root))
	}
}
