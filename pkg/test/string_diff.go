package test

import (
	"testing"

	"github.com/pmezard/go-difflib/difflib"
)

func CompareStrings(t *testing.T, expected, got string) {
	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(expected),
		B:        difflib.SplitLines(got),
		FromFile: "Expected",
		ToFile:   "Got",
		Context:  3,
	}
	diffStr, err := difflib.GetUnifiedDiffString(diff)
	if err != nil {
		t.Fatal(err)
	}
	if diffStr != "" {
		t.Errorf("got diff between expected and actual result: \n%s\n", diffStr)
	}
}
