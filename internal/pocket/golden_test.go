package pocket

import (
	"encoding/hex"
	"flag"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the golden files in testdata")

func TestSampleLibraryMatchesTheGoldenFiles(t *testing.T) {
	files := Files(sample())
	var b strings.Builder
	for _, name := range slices.Sorted(maps.Keys(files)) {
		b.WriteString("== " + name + "\n" + hex.Dump(files[name]))
	}
	golden := filepath.Join("testdata", "sample.golden")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if b.String() != string(want) {
		t.Fatalf("index differs from %s; run go test ./internal/pocket -update and review the diff", golden)
	}
}
