package render_test

import (
	"reflect"
	"testing"

	"music-manager/internal/render"
)

func f(size int64, sha string) render.File { return render.File{Size: size, SHA: sha} }

func TestPlan(t *testing.T) {
	desired := render.Tree{
		"A/new.mp3":     f(10, "n"),
		"A/same.mp3":    f(20, "s"),
		"A/changed.mp3": f(30, "c2"),
		"A/resized.mp3": f(41, "r"),
		"B/cover.jpg":   f(5, "j"),
		"A/zz-last.mp3": f(1, "z"),
		"A/a-first.mp3": f(1, "x"),
	}
	existing := render.Tree{
		"A/same.mp3":    f(20, "s"),
		"A/changed.mp3": f(30, "c1"),
		"A/resized.mp3": f(40, "r"),
		"A/old.mp3":     f(7, "o"),
		"Z/stale.jpg":   f(7, "k"),
	}

	for _, c := range []struct {
		name  string
		prune bool
		extra render.Kind
	}{
		{"without prune strays are reported, not removed", false, render.Stray},
		{"with prune strays are removed", true, render.Remove},
	} {
		t.Run(c.name, func(t *testing.T) {
			want := []render.Op{
				{render.Create, "A/a-first.mp3"},
				{render.Create, "A/new.mp3"},
				{render.Create, "A/zz-last.mp3"},
				{render.Create, "B/cover.jpg"},
				{render.Update, "A/changed.mp3"},
				{render.Update, "A/resized.mp3"},
				{c.extra, "A/old.mp3"},
				{c.extra, "Z/stale.jpg"},
			}
			if got := render.Plan(desired, existing, c.prune); !reflect.DeepEqual(got, want) {
				t.Errorf("got  %v\nwant %v", got, want)
			}
		})
	}

	t.Run("an up-to-date tree needs nothing", func(t *testing.T) {
		if got := render.Plan(desired, desired, true); len(got) != 0 {
			t.Errorf("got %v, want no ops", got)
		}
	})
}
