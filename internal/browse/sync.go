package browse

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"music-manager/internal/device"
	"music-manager/internal/manifest"
	"music-manager/internal/render"
)

// This file is what the page needs to write the library to a folder or a
// drive: where it can go, and what a plan of it says.

// OpCounts says how many operations of each kind a plan holds.
type OpCounts struct {
	Create int `json:"create"`
	Update int `json:"update"`
	Stray  int `json:"stray"`
	Remove int `json:"remove"`
}

// CountOps counts the operations of each kind.
func CountOps(ops []render.Op) OpCounts {
	var c OpCounts
	for _, op := range ops {
		switch op.Kind {
		case render.Create:
			c.Create++
		case render.Update:
			c.Update++
		case render.Stray:
			c.Stray++
		case render.Remove:
			c.Remove++
		}
	}
	return c
}

// Writes is how many files would be written.
func (c OpCounts) Writes() int { return c.Create + c.Update }

// Text says in words what the operations are.
func (c OpCounts) Text() string {
	var parts []string
	if c.Create > 0 {
		parts = append(parts, fmt.Sprintf("%d to create", c.Create))
	}
	if c.Update > 0 {
		parts = append(parts, fmt.Sprintf("%d to update", c.Update))
	}
	if c.Stray > 0 {
		parts = append(parts, plural(c.Stray, "stray file"))
	}
	if c.Remove > 0 {
		parts = append(parts, fmt.Sprintf("%d to delete", c.Remove))
	}
	if len(parts) == 0 {
		return "Nothing to do"
	}
	return strings.Join(parts, " · ")
}

// Summary says what a sync did.
func Summary(c OpCounts, collected int) string {
	var parts []string
	if c.Create > 0 {
		parts = append(parts, fmt.Sprintf("%d created", c.Create))
	}
	if c.Update > 0 {
		parts = append(parts, fmt.Sprintf("%d updated", c.Update))
	}
	if c.Remove > 0 {
		parts = append(parts, fmt.Sprintf("%d deleted", c.Remove))
	}
	if collected > 0 {
		parts = append(parts, plural(collected, "unused file")+" cleared away")
	}
	if len(parts) == 0 {
		return "Nothing needed doing."
	}
	return strings.Join(parts, " · ")
}

// OpView is one thing a sync would do to the target.
type OpView struct {
	Kind string `json:"kind"` // create, update, remove or stray
	Path string `json:"path"`
}

// WarningView is something the player will not do well with a track.
type WarningView struct {
	Track   string `json:"track"`
	Message string `json:"message"`
}

// PlanView is what a sync would do, ready to be looked at.
type PlanView struct {
	Plan     string        `json:"plan"`
	Where    string        `json:"where"`
	Prune    bool          `json:"prune"`
	Ops      []OpView      `json:"ops"`
	Counts   OpCounts      `json:"counts"`
	Text     string        `json:"text"`
	Warnings []WarningView `json:"warnings"`
}

// Plan lays out a plan's operations and warnings. s names the tracks the
// warnings are about.
func Plan(s Snapshot, id, where string, prune bool, ops []render.Op, warnings []device.Warning) PlanView {
	v := PlanView{Plan: id, Where: where, Prune: prune, Ops: []OpView{}, Warnings: []WarningView{}}
	kinds := map[render.Kind]string{render.Create: "create", render.Update: "update", render.Remove: "remove", render.Stray: "stray"}
	for _, op := range ops {
		v.Ops = append(v.Ops, OpView{Kind: kinds[op.Kind], Path: op.Path})
	}
	v.Counts = CountOps(ops)
	v.Text = v.Counts.Text()
	for _, w := range warnings {
		name := w.TrackID
		if t, ok := s.Track(w.TrackID); ok {
			name = trackName(t)
		}
		v.Warnings = append(v.Warnings, WarningView{Track: name, Message: w.Message})
	}
	return v
}

func trackName(t manifest.Track) string {
	if t.Title != "" {
		return t.Title
	}
	return t.ID
}

// TargetView is a place the music can be written to.
type TargetView struct {
	Label string `json:"label"`
	Path  string `json:"path"`
	Kind  string `json:"kind"` // library or drive
}

// Targets lists the library folder and the drives that are mounted.
func Targets(root string, mounts []string) []TargetView {
	out := []TargetView{{Label: "Library folder", Path: Tilde(root), Kind: "library"}}
	for _, m := range mounts {
		out = append(out, TargetView{Label: filepath.Base(m), Path: m, Kind: "drive"})
	}
	return out
}

// MountCandidates lists the folders directly inside the given roots, which is
// where removable drives are mounted. skip names a folder to leave out, such as
// the user's own folder inside /media.
func MountCandidates(roots []string, skip string) []string {
	var found []string
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		var names []string
		for _, e := range entries {
			if e.IsDir() && e.Name() != skip {
				names = append(names, e.Name())
			}
		}
		sort.Strings(names)
		for _, n := range names {
			found = append(found, filepath.Join(root, n))
		}
	}
	return found
}

// DetectMounts finds the drives mounted for the current user.
func DetectMounts() []string {
	user := os.Getenv("USER")
	return MountCandidates([]string{"/run/media/" + user, "/media/" + user, "/media"}, user)
}

// CleanPath turns what was typed or pasted as a file name into a path: it
// drops quotes and shell escapes and a file:// prefix, and expands ~.
func CleanPath(s string) string {
	s = strings.TrimSpace(s)
	if n := len(s); n >= 2 && (s[0] == '\'' && s[n-1] == '\'' || s[0] == '"' && s[n-1] == '"') {
		s = s[1 : n-1]
	}
	if rest, ok := strings.CutPrefix(s, "file://"); ok {
		if p, err := url.PathUnescape(rest); err == nil {
			s = p
		}
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
		}
		b.WriteByte(s[i])
	}
	s = b.String()
	if s == "~" || strings.HasPrefix(s, "~/") {
		s = os.Getenv("HOME") + s[1:]
	}
	return s
}

// Tilde writes a path under the home folder as ~/…
func Tilde(p string) string {
	home := os.Getenv("HOME")
	switch {
	case home == "":
		return p
	case p == home:
		return "~"
	case strings.HasPrefix(p, home+"/"):
		return "~" + strings.TrimPrefix(p, home)
	}
	return p
}
