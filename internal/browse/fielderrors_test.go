package browse

import (
	"errors"
	"strings"
	"testing"

	"music-manager/internal/app"
)

func fieldErrorsOf(t *testing.T, err error) app.FieldErrors {
	t.Helper()
	var fields app.FieldErrors
	if !errors.As(err, &fields) {
		t.Fatalf("%v is not a list of field errors", err)
	}
	return fields
}

func TestEveryFieldOfTheAddFormThatIsWrongIsNamed(t *testing.T) {
	_, err := AddForm{
		Title: " ", Artist: "", Track: "two", Of: "-1", Released: "soon", CutStart: "soon", CutEnd: "1:99",
	}.Edit()

	fields := fieldErrorsOf(t, err)
	for _, key := range []string{"title", "artist", "track", "of", "released", "cutStart", "cutEnd"} {
		if fields[key] == "" {
			t.Errorf("no message for %s: %v", key, fields)
		}
	}
	if len(fields) != 7 {
		t.Errorf("%d messages, want 7: %v", len(fields), fields)
	}
	if fields["title"] != "Title cannot be empty" || !strings.Contains(fields["released"], "1999-05-17") {
		t.Errorf("%v", fields)
	}
}

func TestAnAddFormThatIsRightIsNotAnError(t *testing.T) {
	e, err := AddForm{Title: "A song", Artist: "Some Band", Track: "2", Of: "9", Released: "2001", CutStart: "0:05"}.Edit()

	if err != nil || e.Title == nil || *e.Title != "A song" || *e.TrackTotal != 9 {
		t.Errorf("%+v %v", e, err)
	}
}

func TestTheTrackNumberAndTheTrackTotalAreNamedApart(t *testing.T) {
	_, err := AddForm{Title: "A", Artist: "B", Track: "x"}.Edit()
	if fields := fieldErrorsOf(t, err); len(fields) != 1 || fields["track"] == "" {
		t.Errorf("%v", fields)
	}

	_, err = AddForm{Title: "A", Artist: "B", Track: "1", Of: "x"}.Edit()
	if fields := fieldErrorsOf(t, err); len(fields) != 1 || fields["of"] == "" {
		t.Errorf("%v", fields)
	}
}

func TestATotalBelowTheTrackNumberIsRefusedOnTheTotal(t *testing.T) {
	_, err := AddForm{Title: "A", Artist: "B", Track: "5", Of: "3"}.Edit()

	fields := fieldErrorsOf(t, err)
	if len(fields) != 1 || !strings.Contains(fields["of"], "5") {
		t.Errorf("%v", fields)
	}
}

func TestEveryFieldOfTheBulkFormThatIsWrongIsNamed(t *testing.T) {
	_, err := BulkValues{
		Artist: "-", Of: "many", Released: "soon", CutStart: "soon", CutEnd: "1:99",
		ArtistLatin: "a\nb", AlbumLatin: "a\nb", GenreLatin: "a\nb",
	}.edit()

	fields := fieldErrorsOf(t, err)
	for _, key := range []string{"artist", "of", "released", "cutStart", "cutEnd", "artistLatin", "albumLatin", "genreLatin"} {
		if fields[key] == "" {
			t.Errorf("no message for %s: %v", key, fields)
		}
	}
	if len(fields) != 8 {
		t.Errorf("%d messages, want 8: %v", len(fields), fields)
	}
}

func TestThePreviewOfABulkFormNamesTheWrongFields(t *testing.T) {
	ed, a, ts := seeded(t, shadows...)

	p := PreviewBulk(snap(t, a), idsOf(ts[:2]), BulkValues{Artist: "-", CutStart: "soon"})

	if len(p.Fields) != 2 || p.Fields["artist"] == "" || p.Fields["cutStart"] == "" || p.Changed != 0 {
		t.Errorf("%+v", p)
	}
	_, err := ed.Bulk(idsOf(ts[:2]), BulkValues{Artist: "-"})
	if fields := fieldErrorsOf(t, err); fields["artist"] == "" {
		t.Errorf("%v", fields)
	}
}
