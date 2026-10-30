package cli_test

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestPlainMMPrintsHelpThatPointsToTheWebInterface(t *testing.T) {
	code, out, _ := run(t, filepath.Join(t.TempDir(), "lib"))
	if code != 0 || !strings.Contains(out, "Available Commands") || !strings.Contains(out, "web") || !strings.Contains(out, "mm web") {
		t.Errorf("code %d, out %q", code, out)
	}
}

func TestAnUnknownCommandStillFails(t *testing.T) {
	lib := filepath.Join(t.TempDir(), "lib")
	if code, _, errs := run(t, lib, "nonsense"); code == 0 || !strings.Contains(errs, "unknown command") {
		t.Errorf("code %d, err %q", code, errs)
	}
}

func TestTheWolfgangPreviewIsAFlagOfWebOnly(t *testing.T) {
	lib := filepath.Join(t.TempDir(), "lib")
	if code, _, errs := run(t, lib, "--wolfgang"); code == 0 || !strings.Contains(errs, "unknown flag: --wolfgang") {
		t.Errorf("code %d, err %q", code, errs)
	}
}
