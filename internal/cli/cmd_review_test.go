package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeReview(t *testing.T, text string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "genres.txt")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestApplyingAReviewFromTheCommandLine(t *testing.T) {
	lib, id := libWithSongs(t, "Dream", "Atlek")
	review := writeReview(t, `## ARTISTS -> genre
Band  (2 tracks)  [medium]
    Folk
## TRACKS -> genre
`+id["Atlek"]+`  Band · - · Atlek  [low ; a guess]
    Anthem
Nobody  (1 track)  [high]
    Pop
`)
	// "Nobody" sits in TRACKS, where the key is an ID: it matches no track.
	code, out, errs := run(t, lib, "review", "apply", review, "--dry-run")
	if code != 0 || !strings.Contains(out, "would change 2 tracks") || !strings.Contains(out, "nothing was changed") {
		t.Fatalf("dry run: code %d, out %q, err %q", code, out, errs)
	}
	if _, out, _ := run(t, lib, "ls", "genre:folk"); strings.Contains(out, id["Dream"]) {
		t.Errorf("a dry run set a genre:\n%s", out)
	}

	code, out, errs = run(t, lib, "review", "apply", review)
	if code != 0 || !strings.Contains(out, "changed 2 tracks") {
		t.Fatalf("apply: code %d, out %q, err %q", code, out, errs)
	}
	if !strings.Contains(out, "1 low") || !strings.Contains(out, "TRACKS: Nobody") {
		t.Errorf("the report should say what was left out and what matched nothing:\n%s", out)
	}
	if _, out, _ := run(t, lib, "ls", "genre:folk"); !strings.Contains(out, id["Dream"]) || !strings.Contains(out, id["Atlek"]) {
		t.Errorf("both tracks should be Folk:\n%s", out)
	}

	// With the low ones, the track's own value wins.
	code, out, errs = run(t, lib, "review", "apply", review, "--include-low")
	if code != 0 || !strings.Contains(out, "changed 1 track") {
		t.Fatalf("include low: code %d, out %q, err %q", code, out, errs)
	}
	if _, out, _ := run(t, lib, "ls", "genre:anthem"); !strings.Contains(out, id["Atlek"]) {
		t.Errorf("Atlek should be Anthem:\n%s", out)
	}
}

func TestAMistakeInAReviewFileIsReportedAndNothingIsChanged(t *testing.T) {
	lib, id := libWithSongs(t, "Dream")
	review := writeReview(t, "## ARTISTS -> genre\nBand  (1 track)  [sure]\n    Folk\n")
	code, _, errs := run(t, lib, "review", "apply", review)
	if code == 0 || !strings.Contains(errs, "line 2") || !strings.Contains(errs, "sure") {
		t.Errorf("code %d, err %q", code, errs)
	}
	if _, out, _ := run(t, lib, "ls", "genre:folk"); strings.Contains(out, id["Dream"]) {
		t.Errorf("a genre was set although the file is wrong:\n%s", out)
	}

	if code, _, errs = run(t, lib, "review", "apply", filepath.Join(t.TempDir(), "missing.txt")); code == 0 || errs == "" {
		t.Errorf("a missing file: code %d, err %q", code, errs)
	}
}
