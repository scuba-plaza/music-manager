package web

import (
	"os"
	"testing"

	"music-manager/internal/tsgen"
)

func TestGeneratedTypesAreCurrent(t *testing.T) {
	want, err := tsgen.Generate(Contract()...)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile("../../web/src/api/types.gen.ts")
	if err != nil {
		t.Fatalf("%v\nrun: go run ./cmd/gentypes > web/src/api/types.gen.ts", err)
	}
	if string(got) != want {
		t.Fatal("web/src/api/types.gen.ts is out of date\nrun: go run ./cmd/gentypes > web/src/api/types.gen.ts")
	}
}
