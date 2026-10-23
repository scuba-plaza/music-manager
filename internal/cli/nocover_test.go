package cli_test

import (
	"strings"
	"testing"
)

// Covers are kept as they are given; making them fit a device is done when
// writing to it, so there is nothing to convert in the library.
func TestThereIsNoCommandToConvertCovers(t *testing.T) {
	code, _, errs := run(t, t.TempDir(), "cover", "normalize")
	if code == 0 || !strings.Contains(errs, "unknown command") {
		t.Errorf("code %d, stderr %q; want an unknown command", code, errs)
	}
}
