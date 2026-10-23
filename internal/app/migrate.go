package app

import (
	"path/filepath"

	"music-manager/internal/manifest"
)

// MigrationBackup is where the library file of the old version is kept by a
// migration.
func (a *App) MigrationBackup() string { return filepath.Join(a.root, ".mm", "library.json.v1") }

// Migrate turns the library file of version 1 into the current version. The
// old file is kept in MigrationBackup. With dryRun nothing is written, and the
// report says what would change.
func (a *App) Migrate(dryRun bool) (*manifest.MigrationReport, error) {
	return manifest.Migrate(filepath.Join(a.root, "library.json"), a.MigrationBackup(), dryRun)
}
