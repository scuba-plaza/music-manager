// Package version holds what a build knows about itself. The Nix package
// sets Version and Commit when it links the binary.
package version

// Set at link time with -ldflags "-X music-manager/internal/version.Version=...".
var (
	Version = ""
	Commit  = ""
)

// String describes this build: "dev" if nothing was stamped in, otherwise the
// version, followed by the commit if one is known.
func String() string {
	v := Version
	if v == "" {
		v = "dev"
	}
	if Commit != "" {
		v += " (" + Commit + ")"
	}
	return v
}
