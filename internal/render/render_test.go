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

	t.Run("without prune strays are reported, not removed", func(t *testing.T) {
		want := []render.Op{
			{render.Create, "A/a-first.mp3"},
			{render.Create, "A/new.mp3"},
			{render.Create, "A/zz-last.mp3"},
			{render.Create, "B/cover.jpg"},
			{render.Update, "A/changed.mp3"},
			{render.Update, "A/resized.mp3"},
			{render.Stray, "A/old.mp3"},
			{render.Stray, "Z/stale.jpg"},
		}
		got := render.Plan(desired, existing, false)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got  %v\nwant %v", got, want)
		}
	})

	t.Run("with prune strays are removed", func(t *testing.T) {
		got := render.Plan(desired, existing, true)
		if n := len(got); n != 8 || got[6] != (render.Op{render.Remove, "A/old.mp3"}) || got[7] != (render.Op{render.Remove, "Z/stale.jpg"}) {
			t.Errorf("got %v", got)
		}
	})

	t.Run("an up-to-date tree needs nothing", func(t *testing.T) {
		if got := render.Plan(desired, desired, true); len(got) != 0 {
			t.Errorf("got %v, want no ops", got)
		}
	})

	t.Run("nothing desired and nothing there", func(t *testing.T) {
		if got := render.Plan(nil, nil, true); len(got) != 0 {
			t.Errorf("got %v", got)
		}
	})
}
