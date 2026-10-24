package app

import "music-manager/internal/manifest"

// Records is the store of the records, for tests that stand in for another mm
// in the middle of a change.
func (a *App) Records() *manifest.Store { return a.store }
