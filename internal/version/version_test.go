package version_test

import (
	"testing"

	"music-manager/internal/version"
)

func TestString(t *testing.T) {
	defer func(v, c string) { version.Version, version.Commit = v, c }(version.Version, version.Commit)

	version.Version, version.Commit = "", ""
	if got := version.String(); got != "dev" {
		t.Errorf("unstamped build = %q, want dev", got)
	}
	version.Version = "git-abc1234"
	if got := version.String(); got != "git-abc1234" {
		t.Errorf("= %q", got)
	}
	version.Commit = "abc1234def5678"
	if got := version.String(); got != "git-abc1234 (abc1234def5678)" {
		t.Errorf("with commit = %q", got)
	}
}
