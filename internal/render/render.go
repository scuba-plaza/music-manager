// Package render turns the difference between the tree the manifest asks for
// and the tree on disk into a list of operations.
package render

import "sort"

// File fingerprints a file's content.
type File struct {
	Size int64
	SHA  string
}

// Tree maps slash-separated paths, relative to the tree root, to fingerprints.
type Tree map[string]File

type Kind int

const (
	Create Kind = iota // the file is missing
	Update             // the file exists with other content
	Remove             // a file nobody asked for, deleted because of prune
	Stray              // a file nobody asked for, left alone
)

func (k Kind) String() string { return [...]string{"create", "update", "remove", "stray"}[k] }

type Op struct {
	Kind Kind
	Path string
}

// Plan lists what must happen to turn existing into desired: writes first
// (sorted by kind, then path), then strays. Files that are in existing but
// not in desired are removed when prune is set and only reported otherwise.
func Plan(desired, existing Tree, prune bool) []Op {
	var ops []Op
	for p, want := range desired {
		have, ok := existing[p]
		switch {
		case !ok:
			ops = append(ops, Op{Create, p})
		case have != want:
			ops = append(ops, Op{Update, p})
		}
	}
	extra := Stray
	if prune {
		extra = Remove
	}
	for p := range existing {
		if _, ok := desired[p]; !ok {
			ops = append(ops, Op{extra, p})
		}
	}
	sort.Slice(ops, func(i, j int) bool {
		if ops[i].Kind != ops[j].Kind {
			return ops[i].Kind < ops[j].Kind
		}
		return ops[i].Path < ops[j].Path
	})
	return ops
}
