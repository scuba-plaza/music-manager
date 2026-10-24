package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"music-manager/internal/app"
)

// The header says how many tracks the library folder lags behind the
// records. Normally that is a comparison of records and nothing more; only
// the very first time, when nothing has ever been synced, the folder itself
// is looked at to see how far behind it is.

// syncStatusMsg is the result of working out how far behind the folder is.
type syncStatusMsg struct {
	n    int
	err  error
	keep bool // nothing new was learned: leave the count as it is
}

// refreshSync works out the header's count. With scan set it may also scan
// the folder, once, if there is no earlier sync to compare with.
func (m Model) refreshSync(scan bool) tea.Cmd {
	lib := m.lib
	return func() tea.Msg {
		n, known, err := lib.Unsynced()
		if err != nil || known {
			return syncStatusMsg{n: n, err: err}
		}
		if !scan {
			// Never synced, and scanning the folder again for every edit would
			// be slow. The count from the one scan is still a fair account.
			return syncStatusMsg{keep: true}
		}
		plan, err := lib.PlanSync(app.SyncOptions{})
		if err != nil {
			return syncStatusMsg{err: err}
		}
		if writes := countOps(plan.Ops()).writes(); writes > 0 {
			return syncStatusMsg{n: writes}
		}
		if err := lib.RecordSynced(); err != nil {
			return syncStatusMsg{err: err}
		}
		return syncStatusMsg{}
	}
}

// recordSynced notes that the folder is up to date, after a plan showed it.
func (m Model) recordSynced() tea.Cmd {
	lib := m.lib
	return func() tea.Msg {
		if err := lib.RecordSynced(); err != nil {
			return syncStatusMsg{err: err}
		}
		return syncStatusMsg{}
	}
}

func (m Model) syncStatus(msg syncStatusMsg) (tea.Model, tea.Cmd) {
	if msg.err == nil && !msg.keep {
		m.syncN = msg.n
	}
	return m, nil
}
