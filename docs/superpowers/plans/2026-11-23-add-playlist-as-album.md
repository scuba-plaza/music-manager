# Add a YouTube Playlist as an Album Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

> **Skills that are mandatory for every task below**
> - **`tdd`** (`anthropic-skills:tdd`): every task is red, green, refactor. Write the failing test first, run it and see it fail for the right reason, write the least code that passes, then clean up with the tests green. Do not write production code without a failing test that demands it.
> - **`laws-of-simplicity`** (`anthropic-skills:laws-of-simplicity`): apply it whenever the plan leaves a choice open, and again as a short review pass at the end of each task (reduce, organize, save time, learn, differences, context, emotion, trust, failure, the one). When a choice is between reusing something that exists and adding something new, reuse. When in doubt, leave the feature out and say so.

**Goal:** Let the `a` shortcut take a YouTube playlist link and add its tracks as one album, with the cover, track numbers and album name filled in, after a review step in which the person ticks which tracks to add.

**Architecture:** A playlist link (`/playlist?list=…`) is recognised by its address alone. The server lists it with yt-dlp (titles, order, lengths and the playlist's cover, no audio), keeps the listing on a shelf like it keeps single-track drafts, and the page shows an album form above a list of tracks that are all ticked. On confirm a new job downloads the ticked tracks one after another through the existing single-track download, adds them with the shared album details, and records the whole addition as one undo step.

**Tech Stack:** Go 1.26 (standard library only), yt-dlp, React 19 + TypeScript + zustand, Vitest (unit and browser), Playwright. No new dependencies.

**Spec:** The "Decisions" section of this document. The design was agreed in conversation with the user; there is no separate spec file.

## Global Constraints

- **Limit:** at most `30` tracks in a playlist (`app.MaxAlbumTracks = 30`). A longer playlist is refused with a message; nothing is listed past 31 entries, nothing is downloaded.
- **Track numbers:** a track keeps its position in the playlist as its track number (unticking track 3 of 10 leaves 1, 2, 4 … 10), and "tracks on album" is the number of tracks in the playlist, for every added track.
- **All tracks are ticked** when the review opens.
- **No comments in any code you write**, Go or TypeScript (the user's rule; `web/eslint-rules/no-comments.js` enforces it for the page). Do not add doc comments either.
- **Never use the TypeScript `any` type**, and do not work around it with unsafe assertions; narrow `unknown` with guards in the style of `fetchedOf` in `web/src/state/add.ts`.
- **No new Go or npm dependencies.** `go.mod`, `go.sum`, `web/pnpm-lock.yaml`, `nix/package.nix` hashes stay untouched.
- **The server stays loopback-only and every request still needs the `X-MM` header**; new routes use the existing `read`, `fail`, `writeJSON`, `startJob` helpers.
- **JSON collections are never `null`**: `internal/web/nonull_test.go` checks this; new slices in views are built with `make` or `[]T{}`.
- **API types are generated:** after changing `internal/web/contract.go` or any type reachable from it, run `pnpm --dir web run gen:types`; `TestGeneratedTypesAreCurrent` fails otherwise.
- **Everything CI runs must pass at the end:** `gofmt -l .` empty, `go mod tidy -diff` clean, `go vet ./... && go vet -tags live ./...`, `go test -race ./...`, `pnpm --dir web run check`, `pnpm --dir web run test:browser`, `pnpm --dir web run test:e2e`.
- **Commits:** one per task, on branch `claude/eloquent-franklin-cch2rr`, a plain imperative sentence as the subject (the repository does not use `feat:` prefixes), ending with these two lines:
  ```
  Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
  Claude-Session: https://claude.ai/code/session_01Co5QwojZha19V8pyCsrAFH
  ```
  Do not push or open a pull request unless asked.

## Decisions

Agreed with the user:
1. **Review step with selection.** After a playlist link is fetched, a dialog shows the album details (editable) and the tracks (each with a checkbox, all ticked). Nothing is added until confirm.
2. **Limit of 30 tracks**, refused with a message when exceeded (not truncated, not partially selectable).
3. **Playlist positions are kept** as track numbers when tracks are unticked.

My calls (the user did not decide these; flag them in the final report):
4. **A playlist link is only a `/playlist?list=…` page** on `youtube.com`, `www.youtube.com`, `m.youtube.com` or `music.youtube.com`. A `watch?v=…&list=…` link keeps adding the single video, as today, and every other site is unchanged.
5. **Titles come from the playlist listing and are read-only in the review.** Album, artist, year and genre are shared by all tracks. Album and artist are required; an empty year or genre means "each track's own, if yt-dlp knows it".
6. **The cover is the playlist's own thumbnail**, written by yt-dlp, shown in the review and set on every added track. It cannot be replaced in the review; the existing album Cover field does that afterwards. If no cover can be fetched, each track keeps the cover its own download brought.
7. **Duplicates are found after downloading** (by audio hash, as for one track), skipped and counted. No attempt is made to spot them before.
8. **A track that fails to download is left out and reported; the rest are added.** Cancelling (Esc while downloading) adds nothing at all.
9. **One undo step** for the whole album (`addition of N tracks`), the same label the file import uses.
10. **Downloads are sequential**, with a progress bar and the title being fetched. Each track has the existing 10-minute timeout.
11. **Artist default** is the uploader of the playlist's first entry with ` - Topic` removed, as for one track. If YouTube's channel name is not the real artist (the sample album's channel is `Release - Topic`), the person corrects it in the review; that is what the review is for.

Non-goals: replacing the cover in the review, editing titles in the review, a live preview of file paths, spotting duplicates before download, adding a second album while one is downloading, supporting playlists of other sites, a progress bar for listing.

Known unverified (could not be tested from the planning sandbox because YouTube refused single-video requests there): the per-track metadata yt-dlp returns for the tracks of an album (year, genre) and whether the artist is better read from a track than from the channel. The listing and the playlist thumbnail were verified with the real yt-dlp (`2026.10.26`) on `https://youtube.com/playlist?list=OLAK5uy_miV0URrlTPpC0-nQhtHtRTEZfi5DzZP5o` (10 tracks, title `Al Fajer The Dawn`, a 640×640 cover). Task 11 ends with a manual check on a machine where YouTube answers.

## Review Focus

Most likely first. Each has a test in the task named in brackets.

1. A `watch?v=…&list=…` link must still add one track and never list anything. [Tasks 5, 7]
2. Exactly 30 tracks is accepted, 31 is refused, 0 says there is nothing to add. [Task 1; Task 10 for the page]
3. Unticking everything must not send a request and must say why. [Tasks 8, 9]
4. Closing the dialog or pressing Esc while tracks download must stop yt-dlp, add nothing and leave nothing on the shelf; closing the review must discard the listing. [Tasks 2, 6, 7, 8]
5. One failed download and one already-owned track in the middle of an album must not stop the others, and both must be reported. [Tasks 2, 6]

## Before you start

- [ ] `git status` is clean on `claude/eloquent-franklin-cch2rr`; `go test ./...` passes (it did on 2026-11-23 for `internal/app`, `internal/browse`, `internal/web`, `internal/ytdlp`).
- [ ] `pnpm --dir web install --frozen-lockfile`; `pnpm --dir web run check` passes. If Playwright cannot find a browser, read `web/launch.ts`.
- [ ] Read, in this order: `internal/app/add.go`, `internal/browse/add.go`, `internal/web/add.go`, `internal/web/jobs.go`, `web/src/state/add.ts`, `web/src/dialogs/Add.tsx`, and the existing tests next to each (`add_test.go`, `ytdlp_test.go`).

## File Structure

| File | Responsibility |
|---|---|
| `internal/app/album.go` (new) | `Listing`, `ListedTrack`, `Album`, `FetchAlbum`, `AddAlbum`, `Album.Check`, limits |
| `internal/app/add.go` | `Downloader` gains `List`; shared temp-dir helper; `DownloadTimeout` |
| `internal/ytdlp/list.go` (new) | `Downloader.List`: two yt-dlp runs, one shared `run` helper |
| `internal/browse/shelf.go` (new) | generic `shelf[T]` replacing the drafts map and order slice |
| `internal/browse/album.go` (new) | `IsPlaylistLink`, `AlbumForm`, views, `CheckAlbum`, `AddAlbum`, messages |
| `internal/browse/add.go`, `edit.go` | `Fetch` branches on playlist links; `Store` gains two methods; `Discard`/`DraftCover` cover albums |
| `internal/web/addalbum.go` (new) | `POST /api/add/album`, the `addalbum` job, `AlbumProgress` |
| `internal/web/webtest/fakes.go` | `FakeDownloader.List`, distinct audio per link, `Hold`, `KindOfBlue`, `TooLongLink` |
| `web/src/state/add.ts` | album phases, picks, save, cleanup |
| `web/src/dialogs/AddAlbum.tsx` (new), `Add.tsx` | the review and the progress screens |
| `cmd/e2e-server/main.go`, `web/e2e/add-album.spec.ts` (new) | end-to-end |
| `README.md`, `web/src/keys/keymap.ts` | documentation and the `a` help line |

---

### Task 1: List a playlist and fetch it as an Album (app layer)

**Files:**
- Create: `internal/app/album.go`, `internal/app/album_test.go`
- Modify: `internal/app/add.go` (interface, temp-dir helper), `internal/app/add_test.go` (the fake gains `List`), `internal/web/webtest/fakes.go` (minimal `List` so it still compiles), `internal/web/add.go` (use `app.DownloadTimeout`)

**Interfaces:**
- Produces, in package `app`:
  - `const MaxAlbumTracks = 30` and `const DownloadTimeout = 10 * time.Minute`
  - `type ListedTrack struct { URL, Title string; Length time.Duration }`
  - `type Listing struct { Title, Uploader string; Tracks []ListedTrack; Cover []byte }`
  - `Downloader` gains `List(ctx context.Context, url, dir string, limit int) (Listing, error)`; an implementation returns at most `limit` tracks.
  - `type Album struct { URL, Title, Uploader string; Tracks []ListedTrack; coverSHA string }`, `func (al *Album) CoverSHA() string`
  - `func (a *App) FetchAlbum(ctx context.Context, url string, dl Downloader) (*Album, error)`
- Errors (exact text, built with `MaxAlbumTracks`): `this playlist has more than 30 tracks: only playlists of up to 30 tracks can be added`; `this playlist has no tracks to add`.

- [ ] **Step 1: Write the failing tests** in `internal/app/album_test.go` (package `app_test`). Extend the `fakeDownloader` in `add_test.go` with `listing app.Listing`, `listErr error`, `listCalls []struct{ url string; limit int }` and a `List` method that records the call and returns `listing` (or `listErr`). Tests, each opening a library the way the other tests in `add_test.go` do:
  - `TestFetchAlbumListsTheTracksAndKeepsTheCover`: listing `{Title: "Kind of Blue", Uploader: "Miles Davis - Topic", Tracks: 3 entries, Cover: testmp3.JPEG}`; result has the same `Title`, `Uploader`, 3 `Tracks`, `CoverSHA() != ""`, and the fake saw one call with `limit == app.MaxAlbumTracks+1` and the given URL.
  - `TestFetchAlbumAcceptsExactlyThirtyTracks` (30 entries, no error) and `TestFetchAlbumRefusesMoreThanThirtyTracks` (the fake ignores `limit` and returns 31; error contains `more than 30 tracks`).
  - `TestFetchAlbumRefusesAnEmptyPlaylist` (error contains `no tracks to add`).
  - `TestFetchAlbumPassesOnWhatTheListingSaid` (`listErr` is wrapped; `errors.Is` finds it and the message names the link).
  - `TestFetchAlbumWithoutACoverStillWorks` (`Cover: nil` gives `CoverSHA() == ""`, no error).
- [ ] **Step 2: Run** `go test ./internal/app -run FetchAlbum -v`. Expected: FAIL (does not compile: `Listing`, `List`, `FetchAlbum` undefined).
- [ ] **Step 3: Implement** the types and `FetchAlbum` in `album.go`: ask `dl.List(ctx, url, dir, MaxAlbumTracks+1)` into a temp dir, check the count, store the cover with `StoreCover` and ignore its error exactly as `Fetch` does for a track cover. Extract the temp-dir creation that `Fetch` does inline into `func (a *App) downloadDir() (dir string, remove func(), err error)` and use it in both (the existing `Fetch` tests guard the refactor). Add `List` to the `Downloader` interface in `add.go`; add a `List` to `webtest.FakeDownloader` that returns an empty `Listing` for now; replace `downloadTimeout` in `internal/web/add.go` with `app.DownloadTimeout`.
- [ ] **Step 4: Run** `go test ./internal/app ./internal/web/... -v`. Expected: PASS (new and existing).
- [ ] **Step 5: Commit** — `Fetch a playlist listing as an album in the app layer`.

---

### Task 2: Add the chosen tracks of an album (app layer)

**Files:**
- Modify: `internal/app/album.go`, `internal/app/album_test.go`, `internal/app/add_test.go` (richer fake)

**Interfaces:**
- Consumes: Task 1 types, `(*App).Fetch`, `(*App).Finish`, `Edit`, `DuplicateError`, `manifest.Track`.
- Produces:
  - `func (al *Album) Check(numbers []int) error`: numbers are 1-based positions in `al.Tracks`. Errors: empty gives `pick at least one track`; any number outside `1..len(al.Tracks)`, repeated, or not strictly ascending gives `the chosen tracks are not in this playlist`.
  - `type AlbumFailure struct { Title string; Err error }`
  - `type AlbumReport struct { Added []manifest.Track; Duplicates int; Failed []AlbumFailure }`
  - `func (a *App) AddAlbum(ctx context.Context, al *Album, numbers []int, shared Edit, dl Downloader, progress func(done, total int, title string)) (AlbumReport, error)`
- Behaviour to implement: call `al.Check` first. For each chosen track in order: stop with `ctx.Err()` if the context is done; call `progress(i, len(numbers), title)`; download with `a.Fetch` under `context.WithTimeout(ctx, DownloadTimeout)`; a `*DuplicateError` counts as a duplicate; any other error is a `Failed` entry unless `ctx.Err() != nil`, which aborts. Only when every chosen track has been tried and the context is still live, finish the drafts in order, each with an `Edit` made from `shared` plus `Title` (from the listing), `TrackNumber` (the position), `TrackTotal` (`len(al.Tracks)`), and `Released`/`Genre` taken from the track's own `Info` when `shared`'s are nil or empty. Set each draft's cover to the album cover when the album has one. A `Finish` error is a `Failed` entry (a `*DuplicateError` is a duplicate). End with `progress(len(numbers), len(numbers), "")`. A cancelled run returns `AlbumReport{}` and the context error and adds nothing; the downloaded blobs are left for the next sync to collect, as for an unfinished draft.

- [ ] **Step 1: Write the failing tests.** Extend `fakeDownloader` with `audioFor func(url string) []byte` (falls back to `audio`), `errFor map[string]error`, `infoFor map[string]app.Info`, `onDownload func(url string)`. Tests in `album_test.go`, using a 3-track listing `A`, `B`, `C` with URLs `https://example.org/watch?v=1..3` and distinct audio per URL:
  - `TestAddAlbumAddsTheChosenTracksAsOneAlbum`: choose `[1, 3]`, `shared = Edit{Album: ptr("Kind of Blue"), Artist: ptr("Miles Davis")}`; two tracks added, titles `A` and `C`, `TrackNumber` 1 and 3, `TrackTotal` 3 on both, album and artist as given, `Status == manifest.Identified`, both with the album's `CoverSHA256`, and `m.Tracks` holds them in that order.
  - `TestAddAlbumFillsAnEmptyYearAndGenreFromEachTrack`: `infoFor` gives track 1 `Released: "1959-08-17", Genre: "Jazz"` and track 2 `Released: "1960"`; with empty shared year and genre each track gets its own; with `shared.Released = ptr("2001")` both get `2001`.
  - `TestAddAlbumKeepsATrackCoverWhenTheAlbumHasNone`.
  - `TestAddAlbumSkipsAudioThatIsAlreadyInTheLibrary`: add track B's audio first; `Duplicates == 1`, A and C added.
  - `TestAddAlbumGoesOnWhenOneDownloadFails`: `errFor[url2]`; `Failed` has one entry with `Title == "B"` and that error; A and C added.
  - `TestAddAlbumReportsProgressBeforeEachTrackAndAtTheEnd`: calls are exactly `(0,3,"A") (1,3,"B") (2,3,"C") (3,3,"")`.
  - `TestAddAlbumAddsNothingWhenCancelled`: `onDownload` cancels the context while downloading track 2; the error satisfies `errors.Is(err, context.Canceled)`, the report is empty and the library has the tracks it had before.
  - `TestAlbumCheckRefusesBadChoices`: table of `nil`, `[]int{}`, `[0]`, `[4]`, `[2,1]`, `[1,1]` against 3 tracks (messages above) and `[1]`, `[1,2,3]` accepted.
- [ ] **Step 2: Run** `go test ./internal/app -run 'AddAlbum|AlbumCheck' -v`. Expected: FAIL (undefined).
- [ ] **Step 3: Implement** `Check`, `AddAlbum` and the two report types as described. Keep it one function plus a small helper that builds the per-track `Edit`; set the draft's cover through the unexported `staged` field since this is the same package.
- [ ] **Step 4: Run** `go test ./internal/app -race -v`. Expected: PASS.
- [ ] **Step 5: Commit** — `Add the chosen tracks of an album, reporting duplicates and failures`.

---

### Task 3: yt-dlp lists a playlist (`ytdlp.List`)

**Files:**
- Create: `internal/ytdlp/list.go`, `internal/ytdlp/list_test.go`
- Modify: `internal/ytdlp/ytdlp.go` (extract `run`)

**Interfaces:**
- Consumes: `app.Listing`, `app.ListedTrack`.
- Produces: `func (d *Downloader) List(ctx context.Context, url, dir string, limit int) (app.Listing, error)` and the private `func (d *Downloader) run(ctx context.Context, args ...string) ([]byte, error)` (the whole exec block of `Download`: `LookPath` with the `%s not found in PATH: install it to download music` message, `WaitDelay`, the context error, the last stderr line). `Download` calls `run` and behaves exactly as before.
- yt-dlp calls (verified with the real tool): (1) `--flat-playlist -J -q --no-warnings --playlist-end <limit> -- <url>` prints one JSON object with `title` and `entries[]` (`id`, `title`, `duration` seconds, `channel`, `uploader`); (2) `--flat-playlist --skip-download --write-thumbnail --convert-thumbnails jpg --playlist-items 0 -q --no-warnings -o pl_thumbnail:<dir>/cover -o <dir>/%(id)s -- <url>` writes `<dir>/cover.jpg` (the bytes may be PNG; the app accepts both). `-J` implies a dry run, so the cover needs its own call. A track URL is `https://www.youtube.com/watch?v=<id>`; `Uploader` is the first entry's `channel`, else its `uploader`; entries without an id are skipped; a result with no `entries` gives an empty `Tracks`.

- [ ] **Step 1: Write the failing tests** in `list_test.go` (package `ytdlp_test`). Add a helper `fakePlaylistBin(t, listJSON string, listExit, coverExit int)` next to `fakeBin`'s style: a shell script that appends every argument to `$FAKE_ARGS` with a separator line between runs, prints `$FAKE_JSON` when called with `-J`, and when called with `--write-thumbnail` copies a fixture image to the path after `pl_thumbnail:` plus `.jpg`. Fixture JSON is the real shape: `title` `Al Fajer The Dawn`, two entries (`F2wUzaEinfQ` `Halalalalayya` 430 s, `wFJZY_oSjQk` `3an Insan` 344 s) with `channel` `Release - Topic`. Tests:
  - `TestListReadsThePlaylistAndItsCover`: `Title`, `Uploader == "Release - Topic"`, `Tracks[0] == app.ListedTrack{URL: "https://www.youtube.com/watch?v=F2wUzaEinfQ", Title: "Halalalalayya", Length: 430 * time.Second}`, `Cover` equals the fixture bytes, and the arguments of both runs are exactly the two lists above (limit 31, a temp `dir`).
  - `TestListGoesOnWithoutACoverWhenTheCoverRunFails`: cover run exits 1; `Cover == nil`, no error.
  - `TestListFailsWithTheLastLineOfWhatYtDlpSaid`: first run exits 1 printing `ERROR: Playlist does not exist` on stderr; the error contains that text.
  - `TestListNeedsYtDlpOnThePath`: `Bin: "no-such-yt-dlp"`; the error contains `not found in PATH`.
  - `TestListSkipsEntriesWithoutAnID` and `TestListOfASingleVideoHasNoTracks` (JSON without `entries`).
  - The existing `Download` tests must pass unchanged.
- [ ] **Step 2: Run** `go test ./internal/ytdlp -v`. Expected: FAIL (`List` undefined).
- [ ] **Step 3: Implement** `run` (extract, do not duplicate) and `List`. Parse with a small `rawListing` struct next to `rawInfo`.
- [ ] **Step 4: Run** `go test ./internal/ytdlp -race -v` and `go vet -tags live ./internal/ytdlp`. Expected: PASS.
- [ ] **Step 5: Commit** — `List a playlist and its cover with yt-dlp`.

---

### Task 4: A reusable shelf for waiting downloads (behaviour-preserving refactor)

**Files:**
- Create: `internal/browse/shelf.go`, `internal/browse/shelf_test.go`
- Modify: `internal/browse/edit.go` (Editor fields), `internal/browse/add.go` (`Fetch`, `DraftCover`, `ReplaceDraftCover`, `RemoveDraftCover`, `Discard`, `Finish`)

**Interfaces:**
- Produces: `type shelf[T any] struct{ items map[string]T; order []string }` with a usable zero value, `put(id string, v T)` (forgets the oldest while more than `maxDrafts` are held), `get(id string) (T, bool)`, `drop(id string)`; and `func newID() (string, error)` (8 random bytes in hex, extracted from `Fetch`). All use the editor's existing `mu`.
- Editor: `drafts shelf[*app.Draft]` replaces `drafts map` and `draftOrder`.

- [ ] **Step 1: Write the failing tests** in `shelf_test.go`: `TestAShelfGivesBackWhatWasPut` (zero value works), `TestAShelfForgetsTheOldestPastItsLimit` (put `maxDrafts+1`, the first is gone, the others stay), `TestDroppingSomethingNotThereChangesNothing`, `TestNewIDsAreSixteenHexCharactersAndDiffer`.
- [ ] **Step 2: Run** `go test ./internal/browse -run 'Shelf|NewID|Dropping' -v`. Expected: FAIL.
- [ ] **Step 3: Implement** the shelf and switch the editor's draft handling to it, deleting `draftOrder`.
- [ ] **Step 4: Run** `go test ./internal/browse ./internal/web -race`. Expected: PASS (every existing draft test is the safety net).
- [ ] **Step 5: Commit** — `Hold waiting downloads on a shelf the editor can share`.

---

### Task 5: Review side of an album in the editor (browse layer)

**Files:**
- Create: `internal/browse/album.go`, `internal/browse/album_test.go`
- Modify: `internal/browse/add.go`, `internal/browse/edit.go` (`Store` gains `FetchAlbum(ctx context.Context, url string, dl app.Downloader) (*app.Album, error)`; `Editor` gains `albums shelf[*app.Album]`)

**Interfaces:**
- Consumes: `app.Album`, shelf, `newID`.
- Produces:
  - `func IsPlaylistLink(link string) bool`: true only for an `http`/`https` link whose host is exactly `youtube.com`, `www.youtube.com`, `m.youtube.com` or `music.youtube.com`, whose path is exactly `/playlist`, and which has a non-empty `list` parameter.
  - `type AlbumForm struct { Album, Artist, Released, Genre string }` with JSON names `album`, `artist`, `released`, `genre`, and `func (f AlbumForm) Edit() (app.Edit, error)`: reuses the existing `draft{...}.edit()` helpers for released and genre, requires album and artist (`app.FieldErrors` with `Album cannot be empty` / `Artist cannot be empty`), collects every field error before returning.
  - `type AlbumTrackView struct { Number int; Title string; Seconds int }` with JSON names `number`, `title`, `seconds`.
  - `type AlbumView struct { ID string; Form AlbumForm; Note string; Cover string; Tracks []AlbumTrackView }` with JSON names `id`, `form`, `note`, `cover`, `tracks`.
  - `Fetched` gains `Album *AlbumView` with JSON name `album` and `omitempty`, like `Draft` and `Duplicate`.
  - Defaults: `Album` is the listing title, `Artist` is the uploader without the suffix ` - Topic` (extract that literal into one constant shared with `addDefaults`), `Released` and `Genre` empty. Note text exactly: `From the playlist: check the album and the artist. A year or genre left empty is taken from each track.`
  - `Editor.Fetch` calls `FetchAlbum` and shelves the result when `IsPlaylistLink(link)`; otherwise it behaves as before. `Discard(id)` drops from both shelves; `DraftCover(id)` answers for an album id too (its `CoverSHA()`).

- [ ] **Step 1: Write the failing tests** in `album_test.go` (package `browse`; add a `listingDownloader` test double implementing `app.Downloader` with a fixed `Listing` and a `calls` record, and use `seeded(t)` for the editor, adding `WithDownloader`):
  - `TestOnlyAPlaylistPageIsAPlaylistLink` (table). True: `https://www.youtube.com/playlist?list=OLAK5uy_x`, `https://music.youtube.com/playlist?list=PLabc`, `https://youtube.com/playlist?list=PLabc&si=zz`, `https://m.youtube.com/playlist?list=PLabc`. False: `https://www.youtube.com/watch?v=abc&list=PLabc`, `https://www.youtube.com/playlist`, `https://www.youtube.com/playlist?list=`, `https://example.org/playlist?list=PLabc`, `https://youtu.be/abc?list=PLabc`, `ftp://www.youtube.com/playlist?list=a`, `not a link`.
  - `TestTheAlbumFormStartsWithWhatThePlaylistSaid` (album `Kind of Blue`, artist `Miles Davis` from `Miles Davis - Topic`, year and genre empty).
  - `TestTheAlbumFormChecksWhatItMustHave` (empty album, empty artist, `released: "x"` each report under their own field at once; a valid form returns an `Edit` with album and artist set).
  - `TestFetchingAPlaylistLinkGivesAnAlbumToReview`: `Fetched.Album` set, `Draft` and `Duplicate` nil, `ID` non-empty, tracks numbered 1..N with the listed titles and `Seconds`, `Cover` equals the stored SHA, the note text above, `DraftCover(ID)` returns that SHA.
  - `TestAWatchLinkWithAListIsStillOneTrack` (`https://www.youtube.com/watch?v=abc&list=PLx`: `Draft` set, the double's `List` never called).
  - `TestDiscardForgetsAnAlbum`; `TestAPlaylistOfMoreThanThirtyTracksIsRefusedWithTheAppMessage`.
- [ ] **Step 2: Run** `go test ./internal/browse -run 'Album|Playlist' -v`. Expected: FAIL.
- [ ] **Step 3: Implement** as specified. Build `Tracks` with `make([]AlbumTrackView, 0, n)`.
- [ ] **Step 4: Run** `go test ./internal/browse ./internal/web -race`. Expected: PASS.
- [ ] **Step 5: Commit** — `Review an album fetched from a playlist link in the editor`.

---

### Task 6: Add the album through the editor (browse layer)

**Files:**
- Modify: `internal/browse/album.go`, `internal/browse/album_test.go`, `internal/browse/edit.go` (`Store` gains `AddAlbum(ctx context.Context, al *app.Album, numbers []int, shared app.Edit, dl app.Downloader, progress func(done, total int, title string)) (app.AlbumReport, error)`)

**Interfaces:**
- Consumes: `app.AddAlbum`, `Album.Check`, `Editor.Ingested`, `Plural`, `ErrDraftGone`.
- Produces:
  - `func (e *Editor) CheckAlbum(id string, f AlbumForm, numbers []int) error`: `ErrDraftGone` for an unknown id, the form's `app.FieldErrors`, or `Album.Check`'s error, in that order; changes nothing.
  - `func (e *Editor) AddAlbum(ctx context.Context, id string, f AlbumForm, numbers []int, progress func(done, total int, title string)) (Result, error)`: runs `CheckAlbum`, then `app.AddAlbum`; on a returned error it keeps the album on the shelf and returns the error; otherwise it drops the album, calls `e.Ingested(report.Added)` and returns the `Result`.
- Message (exact): added: `Added 2 tracks of "Kind of Blue"` (`Plural`, `%q` for the album) then `· 1 already there` and `· 1 left out: B: <error>` (several failures joined with `; `), each part preceded by ` · `, ending with ` · u undoes it`; `Reveal` is the first added track's ID. Nothing added: `Nothing was added`, the same ` · …` parts without the undo hint, `Result.Same = true`.

- [ ] **Step 1: Write the failing tests** (the `listingDownloader` double gets per-URL audio, errors and `Info`):
  - `TestAddingAnAlbumRecordsOneStepToUndo`: fetch, then choose `[1, 3]` of 3; message `Added 2 tracks of "Kind of Blue" · u undoes it`; `History().Undo == "addition of 2 tracks"`; one `Undo` removes both and `Redo` puts both back.
  - `TestTheMessageSaysWhatWasAlreadyThereAndWhatFailed` (one duplicate, one failed: both parts appear in the message above).
  - `TestAnAlbumOfOnlyFailuresSaysNothingWasAdded` (`Same`, no undo step recorded).
  - `TestCheckAlbumNamesWhatIsWrong` (gone id is `ErrDraftGone`; empty album gives `FieldErrors`; numbers `[]` give the message containing `at least one track`).
  - `TestAnAddedAlbumIsGone` (a second `AddAlbum` with the same id is `ErrDraftGone`).
  - `TestACancelledAlbumAddsNothingAndStaysForAnotherTry` (cancelled context: error, library unchanged, a second call with a live context succeeds).
- [ ] **Step 2: Run** `go test ./internal/browse -run 'Album' -v`. Expected: FAIL.
- [ ] **Step 3: Implement**; build the message in one small unexported function so it is testable on its own.
- [ ] **Step 4: Run** `go test ./internal/browse -race`. Expected: PASS.
- [ ] **Step 5: Commit** — `Add an album as one undo step with a report of what was left out`.

---

### Task 7: The album route and job (web server)

**Files:**
- Create: `internal/web/addalbum.go`, `internal/web/addalbum_test.go`
- Modify: `internal/web/add.go` (register the route in `addRoutes`), `internal/web/contract.go`, `internal/web/webtest/fakes.go`, `internal/web/webtest/fakes_test.go`, `web/src/api/types.gen.ts` (generated)

**Interfaces:**
- Produces:
  - `POST /api/add/album` with body `{ "id": string, "form": AlbumForm, "tracks": number[] }` (unknown fields are refused like every route). It calls `Editor.CheckAlbum` and answers `422` with `{error, fields}` on failure (`ErrDraftGone` also `422`, as `/api/add/finish` does); otherwise it starts a job of kind `addalbum` (`409`, `an album is being added already`, if one runs) and answers the `JobView`. While it runs the job's `Data` is `AlbumProgress`; when done it is a `ChangeResponse` (`Editor.AddAlbum`'s `Result` plus `s.ed.History()`).
  - `type AlbumProgress struct { Done, Total int; Title string }` with JSON names `done`, `total`, `title`, added to `Contract()`.
  - `DELETE /api/add/{draft}` and `GET /api/add/{draft}/cover` already work for album ids through the editor.
  - webtest: `FakeDownloader` gains `Listing app.Listing` (its `List` returns the first `limit` tracks), `ListCalls() []string`, `Hold chan struct{}` (when non-nil, `Download` waits for it to close or for the context and returns `ctx.Err()`), and distinct audio per distinct link (the first link keeps today's 700 frames, later ones add 10 frames each, `DuplicateLink` is unchanged). New `func KindOfBlue() app.Listing` (title `Kind of Blue`, uploader `Miles Davis - Topic`, five tracks `So What`, `Freddie Freeloader`, `Blue in Green`, `All Blues`, `Flamenco Sketches` with distinct `https://www.youtube.com/watch?v=kob1..5` links and about 5 minutes each, cover `testmp3.JPEG`) and `const TooLongLink = "https://music.youtube.com/playlist?list=toolong"` for which `List` returns 40 tracks cut to `limit`.

- [ ] **Step 1: Write the failing tests.** In `fakes_test.go`: `TestDifferentLinksBringDifferentAudio`, `TestListHonoursItsLimit`, `TestHoldBlocksUntilReleasedOrCancelled`. In `addalbum_test.go`, following `add_test.go` (`seed`, `start`, `send`, `sendInto`, `waitJob`):
  - `TestAPlaylistLinkIsFetchedForReviewAndTheChosenTracksAreAdded`: fetch job done with `Album` of 5 tracks, all JSON arrays non-null (`noNullCollections`); post `tracks: [1,2,4]`; the job ends `done` with a `ChangeResponse` whose message is `Added 3 tracks of "Kind of Blue" · u undoes it`; the library gained 3 tracks numbered 1, 2, 4 of 5; `POST /api/undo` removes all three.
  - `TestAnAlbumNeedsAnArtistAnAlbumAndATrack`: empty form gives `422` with `fields.album` and `fields.artist`; `tracks: []` gives `422` mentioning `at least one track`; an unknown id gives `422` `download is gone`.
  - `TestAWatchLinkWithAListStillAddsOneTrack`: `https://www.youtube.com/watch?v=1&list=PLx` gives a `Draft`; `ListCalls()` is empty.
  - `TestAPlaylistOfMoreThanThirtyTracksIsRefused`: `TooLongLink`; the fetch job fails with `more than 30 tracks`.
  - `TestTheAlbumCoverIsServedAndDiscardedLikeADraftCover`: `GET /api/add/<id>/cover` is `200` `image/jpeg`; after `DELETE /api/add/<id>` it is `404`.
  - `TestStoppingAnAlbumDownloadAddsNothing`: `Hold` set; start; `DELETE /api/jobs/<id>`; the job ends `stopped` and the library is unchanged.
  - `TestOnlyOneAlbumIsAddedAtATime` (`Hold` set; a second post gives `409`).
- [ ] **Step 2: Run** `go test ./internal/web -run 'Album|Playlist|Hold|List' -v`. Expected: FAIL.
- [ ] **Step 3: Implement** the handler (validate, then `startJob`, the work calling `Editor.AddAlbum` with a progress function that calls `report(AlbumProgress{…})`), the contract entry, the fake changes, then `pnpm --dir web run gen:types`.
- [ ] **Step 4: Run** `go test ./internal/web/... ./cmd/... -race`. Expected: PASS, including `TestGeneratedTypesAreCurrent` and the existing add tests (the distinct-audio change must not break them).
- [ ] **Step 5: Commit** — `Serve adding an album as a job with progress`.

---

### Task 8: Page state for the album review

**Files:**
- Modify: `web/src/state/add.ts`, `web/src/state/add.test.ts`

**Interfaces:**
- Consumes: generated `AlbumView`, `AlbumForm`, `AlbumTrackView`, `AlbumProgress`, `ChangeResponse`, `JobView`; `send`, `follow`, `stopJob`, `applyChange`, `fieldErrors`, `closeOverlay`.
- Produces: `AddPhase` gains `'album'` and `'adding'`. State gains `album: AlbumView | null`, `albumForm: AlbumForm`, `albumErrors: Record<string, string>`, `picked: readonly number[]`, `progress: AlbumProgress | null`. Exports: `type AlbumKey = keyof AlbumForm`, `albumOrder: readonly AlbumKey[]` (`album`, `artist`, `released`, `genre`), `setAlbumField(key: AlbumKey, text: string): void` (also clears that field's error), `togglePick(number: number): void`, `pickAll(all: boolean): void`, `saveAlbum(): Promise<AlbumKey | undefined>` (resolves with the first wrong field, if any).
- Behaviour: `receive` of a fetch outcome with `album` sets phase `album`, `picked` to every track number, `albumForm` to `album.form`, `cover` to `album.cover`. `saveAlbum` with nothing picked sets `error: 'Pick at least one track'` and sends nothing. Otherwise it sets phase `adding`, posts `/api/add/album` with `{ id, form, tracks }` (tracks ascending), follows the job (reading `progress` from job data through a guard), and on `done` runs the job's `ChangeResponse` through `applyChange` and closes the overlay; on `failed` shows phase `failed` with the job's error. A `422` with fields returns to phase `album` with `albumErrors` set. Closing the overlay in phase `adding` stops the job (`stopJob`); in phase `album` it `DELETE`s `/api/add/<album.id>`.

- [ ] **Step 1: Write the failing tests** in `add.test.ts`, in the style already there (stub `fetch`, drive the store): `receiving an album fills the review and picks every track`; `togglePick unticks and reticks, keeping the picks in order`; `pickAll(false) then pickAll(true)`; `saving with nothing picked says so and sends nothing`; `saving sends the id, the form and only the picked numbers in ascending order`; `progress from the job shows while adding`; `a finished job applies its message and closes the dialog`; `a field error returns to the review and names the field`; `closing while adding stops the job`; `closing the review discards the album on the server`.
- [ ] **Step 2: Run** `pnpm --dir web exec vitest run --project unit src/state/add.test.ts`. Expected: FAIL.
- [ ] **Step 3: Implement.** Keep the existing single-track paths untouched; add the album paths beside them. Narrow job data with guards, no assertions and no `any`.
- [ ] **Step 4: Run** `pnpm --dir web run check`. Expected: PASS (types, lint including the no-comments rule, unit tests).
- [ ] **Step 5: Commit** — `Keep the state of an album review and its download`.

---

### Task 9: The album review and progress screens

**Files:**
- Create: `web/src/dialogs/AddAlbum.tsx`, `web/src/dialogs/AddAlbum.browser.test.tsx`
- Modify: `web/src/dialogs/Add.tsx` (render the new phases, hints, keys), `web/src/styles/app.css`, `web/src/keys/keymap.ts` (line 167 help text becomes `add from a YouTube link or playlist, or drop files on the page`)

**Interfaces:**
- Consumes: Task 8's state and actions; `FormField`, `Problem`, `Progress`, `ringKey`.
- Produces: `AlbumReview()` and `AlbumProgressView()` components, used by `Add.tsx` for phases `album` and `adding`. `hintsFor` gains `album`: `space toggle`, `tab next field`, `enter add`, `esc close`; and `adding`: `esc cancel`.
- Screen: the asking text becomes `Paste a link to a video, a track or a playlist:`. The review shows a note line, the cover (`/api/add/<id>/cover?v=<first 16 of cover>`), the four album fields (labels `Album`, `Artist`, `Released`, `Genre`; year and genre placeholders `each track's own`), a line `N of M selected` with two buttons `Select all` and `Select none`, then a scrollable list in which every row is a checkbox named `<number>. <title>` with the title in `<bdi dir="auto">` and the length as `m:ss`. Tab order: fields, buttons, checkboxes. Enter on a field or a checkbox adds (a focused button is left to click); Space toggles the focused checkbox natively. The progress screen shows `Downloading <title>…`, the existing `Progress` bar with `done/total`, and `esc cancels.`

- [ ] **Step 1: Write the failing tests** in `AddAlbum.browser.test.tsx` (copy the setup of `AddAndCheck.browser.test.tsx`: `stubServer`, `FakeEventSource`, `emit`): `the review lists every track ticked with the album details filled in`; `an Arabic title is shown in a bdi with dir auto`; `space unticks the focused track and the count follows`; `Select none then Enter says to pick a track and sends nothing`; `Enter posts the album form and the ticked numbers`; `the progress screen shows the title and the bar from job events`; `Esc while adding asks the server to stop the job`; `Esc in the review discards the album`; `the first field is focused when the review opens`.
- [ ] **Step 2: Run** `pnpm --dir web exec vitest run --project browser src/dialogs/AddAlbum.browser.test.tsx`. Expected: FAIL.
- [ ] **Step 3: Implement** the components and the CSS (`.tracklist` with a maximum height and overflow, using the existing tokens; check `web/src/styles/styles.test.ts` for token rules). Update any existing test or e2e that quotes the old asking text.
- [ ] **Step 4: Run** `pnpm --dir web run check && pnpm --dir web run test:browser`. Expected: PASS.
- [ ] **Step 5: Commit** — `Show an album review with tracks to tick and a progress screen`.

---

### Task 10: End to end

**Files:**
- Create: `web/e2e/add-album.spec.ts`
- Modify: `cmd/e2e-server/main.go` (give the fake downloader `Listing: webtest.KindOfBlue()`), `cmd/e2e-server/main_test.go` if it asserts the fake's setup

**Interfaces:**
- Consumes: Task 7's `webtest.KindOfBlue`, `webtest.TooLongLink`; the fixtures `openPage`, `dialog`, `said`, `library`, `row` in `web/e2e/fixtures.ts`. The seeded library has 6 tracks.

- [ ] **Step 1: Write the failing tests** in `add-album.spec.ts`:
  - `a adds a playlist as an album with the tracks the person left ticked`: press `a`, fill `https://music.youtube.com/playlist?list=OLAK5uy_kindofblue`, Enter; the review shows album `Kind of Blue`, artist `Miles Davis`, five checked tracks, the cover loading (`naturalWidth > 0`); untick `2. Freddie Freeloader`; Enter; status `Added 4 tracks of "Kind of Blue"`; the header says `10 tracks`; the album's tracks are numbered 1, 3, 4, 5 of 5; `u` returns the header to `6 tracks`.
  - `a playlist with nothing ticked cannot be added`: `Select none`, Enter, the dialog says `Pick at least one track` and stays open.
  - `a playlist of more than 30 tracks is refused on the link step`: `TooLongLink` shows `more than 30 tracks` and the link box is offered again.
  - `a watch link with a list in it still adds one track`.
- [ ] **Step 2: Run** `pnpm --dir web exec playwright test e2e/add-album.spec.ts --trace off`. Expected: FAIL (before the server change) then, after Step 3, PASS. (Playwright builds the page and the e2e server itself.)
- [ ] **Step 3: Implement** the server fixture change; fix what the tests show.
- [ ] **Step 4: Run** `pnpm --dir web run test:e2e --trace off`. Expected: PASS, including every existing spec.
- [ ] **Step 5: Commit** — `Test adding a playlist as an album end to end`.

---

### Task 11: Documentation, full verification and a real-link check

**Files:**
- Modify: `README.md` (the **Add.** paragraph at line 69 and the first paragraph of The web interface if it lists the add sources)

- [ ] **Step 1: Update the README.** In the Add paragraph, after the sentence about one YouTube link, say: a `/playlist?list=…` link of at most 30 tracks opens an album review instead; album, artist, year and genre are given once for all tracks (an empty year or genre is taken from each track); the cover is the playlist's own; every track is ticked and can be unticked; each track keeps its place in the playlist as its track number and the playlist's size as "tracks on album"; the download shows progress, Esc stops it and adds nothing; tracks already in the library or that fail are left out and named; the whole album is one undo step; the cover and titles can be changed afterwards as for any album. Keep the README's plain, present-tense style.
- [ ] **Step 2: Run everything CI runs**, in this order, and expect each to pass: `gofmt -l .` (no output); `go mod tidy -diff`; `go vet ./... && go vet -tags live ./...`; `go test -race ./...`; `pnpm --dir web run check`; `pnpm --dir web run test:browser`; `pnpm --dir web run test:e2e --forbid-only`. `git status` must show no change to `go.mod`, `go.sum`, `web/pnpm-lock.yaml` or `nix/`.
- [ ] **Step 3: Review pass with `laws-of-simplicity`.** Read the whole diff once. Remove anything no test demands; check that no comment and no `any` slipped in (`git diff | grep -nE '^\+\s*(//|/\*)|: any|<any>'` prints nothing); check that nothing handles a case that is listed as a non-goal.
- [ ] **Step 4: Manual check on a machine where YouTube answers** (needs the real `yt-dlp` and `ffmpeg`; not possible in the planning sandbox). Run `go run ./cmd/mm --library /tmp/mm-album-check`, press `a`, paste `https://youtube.com/playlist?list=OLAK5uy_miV0URrlTPpC0-nQhtHtRTEZfi5DzZP5o`. Expect: 10 tracks, all ticked, album `Al Fajer The Dawn`, the artist field filled from the channel (`Release`; correct it if that is wrong), a square cover. Untick one, add, and check: the added tracks carry the cover, numbers match the playlist positions and "of 10", the year and genre appear if YouTube knows them, `u` undoes the lot. If the year or genre are empty although YouTube Music shows them, or the artist default is wrong for most albums, report it: the fix would be to read the first track's own metadata for the form defaults, deliberately left out of this plan.
- [ ] **Step 5: Commit** — `Document adding a playlist as an album`.
