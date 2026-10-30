# mm: web-only, device profiles, TypeScript front end — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **REQUIRED SUB-SKILL FOR EVERY TASK: superpowers:test-driven-development** (the TDD skill). Invoke it at the start of each task. No production code is written before a failing test that you have watched fail for the stated reason. Where a task changes existing behaviour that already has tests, the existing tests are the safety net and must stay green.

**Goal:** Turn `mm` into a web-only program (no CLI), make device targets data-driven profiles, and replace the vanilla-JS page with a typed, testable React front end, while keeping the Go core, the record/render model and first-class Nix support.

**Architecture:** The Go core (`app`, `browse`, `device`, `manifest`, `mp3`, …) stays; it is already a headless core with a thin client. `mm` becomes one binary that starts the loopback web server. `device.Target` gains a data-driven implementation, `Profile`, so the Wolfgang WMP01 and the generic target become built-in TOML profiles and the user can add more in `devices.toml`. A new `web/` project (React 19 + TypeScript + Vite) talks to the same JSON API with types generated from the Go structs, and is embedded into the Go binary.

**Tech Stack:** Go 1.26 (stdlib `net/http`, `embed`, `github.com/pelletier/go-toml/v2`), TypeScript (strict), React 19, Vite, zustand, Vitest 4 (Browser Mode, Playwright provider), Playwright Test, ESLint, Nix flakes (`buildGoModule`, `buildNpmPackage`, home-manager module).

**Spec:** none written separately; you asked for a plan directly. The approved decisions are in "Decisions" below and are the spec this plan argues from.

## Decisions (from the conversation)

| # | Decision |
|---|---|
| D1 | Drop the CLI entirely, including `review apply`, `playlist import`, the interactive `mm add` form, shell completions. `mm` starts the web server. |
| D2 | Keep the model: records in `library.json` + content-addressed `.mm/`; the music folder is rendered output by sync. |
| D3 | Drop the terminal look. Proportional UI font, Arabic-capable. Keep the keyboard shortcuts as one declarative keymap that also generates the footer hints and the `?` help. |
| D4 | Keep the on-demand local server on `127.0.0.1:7878`. |
| D5 | Device profiles as data: built-in `generic` and `wolfgang-wmp01`, plus user profiles in `devices.toml`. The library folder is always written with `generic`. Not included: remembering a card by label/UUID. |
| D6 | No in-app player. Keep the 8-second cut preview. Gapless stays the player. |
| D7 | Keep the Go core; rebuild the front end in TypeScript with React 19, Vite, Vitest, Playwright; types generated from Go. |
| D8 | Browser tests (Vitest browser project, Playwright e2e) run only in the dev shell (`nix develop`). Go tests, `tsc`, ESLint and node-mode unit tests run in `nix flake check`. |
| D9 | Also ship a home-manager module that writes `devices.toml` from Nix options. |

**Assumptions (change them before starting if you disagree):** npm, not pnpm; `zustand` for state; TOML via `go-toml/v2` with strict decoding; the TypeScript generator is a small in-repo reflect-based tool, because the only maintained-looking generator found (`tygo`) last tagged a release in 2022; config path is `$MM_CONFIG`, else `$XDG_CONFIG_HOME/mm/devices.toml`, else `~/.config/mm/devices.toml`; a missing default config means built-ins only, an invalid one refuses to start; new front end lives in `web/`, builds into `internal/web/dist/`.

## Global Constraints

- **No comments in code.** Applies to every Go, TypeScript, CSS and Nix file written or edited in this plan. Names and tests carry the meaning. (Markdown docs are exempt.)
- **TypeScript never uses `any`.** ESLint enforces `@typescript-eslint/no-explicit-any: error`; use `unknown` and narrow. The type generator maps `any`/`interface{}`/`json.RawMessage` to `unknown`.
- Go `1.26.0`; `CGO_ENABLED=0`; one static binary; module name `music-manager`.
- Nix: `nix build` runs the Go tests in the sandbox, so Go tests must not need a browser, network, or a built front end. Nix only sees git-tracked files: `git add` new files before `nix build`. When Go dependencies change set `vendorHash = lib.fakeHash;`, build once, paste the printed hash. When npm dependencies change do the same with `npmDepsHash`.
- The web server listens on loopback only; `guard` stays as is: Host must be loopback; a request that changes anything needs the `X-MM` header and an `Origin` equal to the Host. The Vite dev proxy must not rewrite Host (`changeOrigin: false`).
- The library folder is always rendered with the built-in `generic` profile; no profile choice applies to it.
- Existing golden outputs `internal/app/testdata/sync-generic.golden` and `sync-wolfgang.golden` must not change at any point.
- Names in Arabic or any other right-to-left script are laid out in their own direction without reordering the text around them (`<bdi dir="auto">` for every displayed name). Below 800 px width one pane shows at a time.
- UI tests assert DOM and layout metrics (rects, line counts, order), never pixel snapshots: font rendering differs across machines.
- The legacy API fields (`wolfgang` in the sync request, `info.wolfgang`, the Wolfgang tab, `WithWolfgang`) stay until Task 22 so the old page keeps working until the cutover.
- Commit after every task on branch `claude/optimistic-thompson-eignis`. Do not open a pull request unless asked.

## Review Focus

Inputs the spec implies but no feature task would obviously test; each has its test in the task named.

1. An invalid `devices.toml` (unknown or misspelled key, duplicate or reserved name, zero or negative width, unknown enum) must refuse to start and name the file, the profile and the key; it must never fall back silently to writing the wrong format to a card. (Tasks 4, 6, 7)
2. Typing in an input, IME composition (`isComposing`) and modifier combinations must not trigger single-key shortcuts; `ctrl+r`, `ctrl+a`, `ctrl+u/d` must still work outside inputs. (Task 13)
3. Arabic and mixed-direction names in rows, dialogs, the inspector and the Latin tab keep their own direction, never overlap the count column, and sort under their Latin form. (Task 21)
4. A binary built without the front end (`go run` before `npm run build`) answers HTTP 503 with a page saying what to run, not a 404 or a panic. (Task 10)
5. Empty states: an empty library, a library with no playlists (the `p` dialog goes straight to naming), an unidentified track, an empty playlist. (Task 21)

## File Structure

**Delete:** `internal/cli/`, `internal/prompt/`, `internal/device/wolfgangpl.go` (+ test), `internal/web/static/`, `internal/web/static_test.go`, `internal/web/layout_test.go`, `internal/web/testdata/playlist_picker_probe.js`.

**Create (Go):**
- `internal/launch/launch.go` — `Run`, flags, defaults; replaces the CLI.
- `internal/device/profile.go` — `Profile`, parsing, validation.
- `internal/device/target.go` — `Profile` as a `Target`.
- `internal/device/builtin.toml` — embedded built-in profiles.
- `internal/device/registry.go` — `Registry`, `ConfigPath`, `LoadRegistry`.
- `internal/tsgen/tsgen.go` — reflect-based TypeScript generator; `cmd/gentypes/main.go` — writes `web/src/api/types.gen.ts`.
- `internal/web/contract.go` — the list of types that cross the wire.
- `internal/web/webtest/` — `Seed`, `FakeSource`, `FakeDownloader` shared by Go tests and the e2e server; `cmd/e2e-server/main.go`.

**Create (front end, `web/`):** `package.json`, `package-lock.json`, `tsconfig.json`, `vite.config.ts`, `vitest.config.ts`, `playwright.config.ts`, `eslint.config.js`, `index.html`, and under `src/`: `main.tsx`, `App.tsx`, `api/{client,events}.ts`, `api/types.gen.ts`, `state/{store,actions}.ts`, `keys/{keymap,dispatch}.ts`, `components/{Header,Footer,Split,Tree,Inspector,Fields,Cover}.tsx`, `overlay/{OverlayHost,Dialog}.tsx`, `dialogs/{Ask,Help,SortMenu,PlaylistPicker,CopyList,BulkForm,Identify,Add,Doctor,Sync}.tsx`, `styles/{tokens,app}.css`; tests beside sources (`*.test.ts` node, `*.browser.test.tsx` browser); end-to-end in `web/e2e/`.

**Create (Nix):** `nix/frontend.nix`, `nix/hm-module.nix`.

**Modify:** `cmd/mm/main.go`, `go.mod`, `go.sum`, `flake.nix`, `nix/package.nix`, `.gitignore`, `README.md`, and the `app`, `browse`, `web` files named in the tasks.

---

# Phase 1 — Remove the CLI

### Task 1: Preserve the behaviour only the CLI tests pin

**Skill:** superpowers:test-driven-development (characterization variant: the "watch it fail" step is a deliberate mutation).

**Files:**
- Create: `docs/superpowers/notes/cli-test-map.md`
- Test: new tests in `internal/app/*_test.go` and `internal/web/*_test.go` as the map requires

**Interfaces:** Produces: a map that Task 3 consults before deleting `internal/cli`.

- [ ] **Step 1:** List the tests: `grep -hn "^func Test" internal/cli/*_test.go`. In `cli-test-map.md` write one row per test: `test name | dropped (CLI-only: flags, help, parsing) | covered by <existing test> | NEW: <test to write>`.
- [ ] **Step 2:** These behaviours must not end as "dropped" without an equivalent at `app` or `web` level; check each explicitly: sync dry-run then apply; prune only with confirmation; remove then sync collects blobs; removing with a repeated ID removes nothing; a cover that is not an image changes nothing; volume and fade values are validated with a message that says what is wrong; a Latin form on several lines is refused; ReplayGain tags are written when a meter is present and none when it is absent; a playlist's Latin name resolves per target; sorting by every field; release date plus the three dates; genre filter; source link edit; doctor; ingest is idempotent.
- [ ] **Step 3:** For every `NEW` row write the test at `app` level (`internal/app`) when it exercises an `App` method, or at `web` level through the HTTP API when it exercises a handler. Name tests after the behaviour (e.g. `TestRemovingWithARepeatedIDRemovesNothing`).
- [ ] **Step 4:** Run `go test ./internal/app ./internal/web -run '<each new test>' -v`. Expected: PASS (the behaviour exists). Then for each new test comment out the one line it guards, run it, expect FAIL, restore the line.
- [ ] **Step 5:** Commit: `test: pin the behaviour that only the CLI tests covered`.

### Task 2: A server-only entry point

**Skill:** superpowers:test-driven-development.

**Files:**
- Create: `internal/launch/launch.go`
- Test: `internal/launch/launch_test.go`
- Modify: `cmd/mm/main.go`

**Interfaces:**
- Produces:
```go
type Options struct {
	Library string
	Addr    string
	AddrSet bool
	NoOpen  bool
}
type Env struct {
	Args   []string
	Getenv func(string) string
	Home   string
	Out    io.Writer
	Err    io.Writer
	Serve  func(ctx context.Context, o Options) error
}
func Run(ctx context.Context, e Env) int
func DefaultLibrary(getenv func(string) string, home string) string
```

- [ ] **Step 1: Write the failing tests** in `launch_test.go` (port of the CLI tests that survive):
  - `TestTheDefaultLibraryIsMusicLibrary`: `DefaultLibrary(noEnv, "/home/me") == "/home/me/Music/Library"`; with `MM_LIBRARY=/x` returns `/x`.
  - `TestNoArgumentsStartsTheServerOnTheDefaultAddress`: `Serve` called once with `Addr == "127.0.0.1:7878"`, `AddrSet == false`, `NoOpen == false`; exit 0.
  - `TestAddrAndNoOpenFlagsArePassedOn`: `--addr 127.0.0.1:9000 --no-open` gives `AddrSet == true`, `NoOpen == true`.
  - `TestLibraryFlagBeatsTheEnvironment`.
  - `TestVersionFlagPrintsTheVersion`: output `"mm " + version.String() + "\n"`, `Serve` not called.
  - `TestUnknownFlagFails`: `--wolfgang` and `--bogus` give exit 2, stderr names the flag, `Serve` not called.
  - `TestAServerErrorIsReported`: `Serve` returns an error → exit 1, stderr `mm: <error>`.
- [ ] **Step 2: Run** `go test ./internal/launch -v`. Expected: FAIL (package does not exist).
- [ ] **Step 3: Implement** `Run` with the stdlib `flag` package (`--library`, `--addr`, `--no-open`, `--version`). Rewrite `cmd/mm/main.go` to build `Env` and a `Serve` that does what `RunWeb` did: `app.Open`, `UseLoudness`, `web.Listen(o.Addr, !o.AddrSet)`, print `mm is at URL  (Ctrl+C stops it)`, `web.OpenBrowser` unless `NoOpen`, `web.New(a, web.WithSource(lookup), web.WithDownloader(dl), web.WithWolfgang()).Serve(ctx, ln)`; keep the `MM_MUSICBRAINZ_URL`, `MM_COVERART_URL`, `MM_CONTACT` handling. `WithWolfgang()` is always passed for now (removed in Task 22).
- [ ] **Step 4: Run** `go test ./internal/launch ./... ` and `go run ./cmd/mm --library /tmp/lib --no-open --version`. Expected: PASS; prints `mm <version>`.
- [ ] **Step 5: Commit:** `mm starts the web server directly: a launch package replaces the command line parser`.

### Task 3: Delete the CLI and everything only it used

**Skill:** superpowers:test-driven-development.

**Files:**
- Delete: `internal/cli/`, `internal/prompt/`, `internal/device/wolfgangpl.go`, `internal/device/wolfgangpl_test.go`
- Modify: `internal/app/add.go`, `internal/app/identify.go`, `internal/browse/edit.go` (remove the `Prompter` parameter and `ErrAborted` if unused), `go.mod`, `go.sum`, `nix/package.nix`
- Test: `internal/launch/modules_test.go`

**Interfaces:** Consumes: Task 1's map (every `NEW` row already has a test). Produces: `app.Add(ctx, req, dl)` and `app.Identify(ctx, ids, src, opts)` without a prompter argument.

- [ ] **Step 1: Write the failing test** `TestTheModuleHasNoCommandLineFramework` in `internal/launch/modules_test.go`: read `../../go.mod`; fail if it contains `spf13/cobra`, `spf13/pflag`, `charmbracelet/`, or `golang.org/x/term`.
- [ ] **Step 2: Run** `go test ./internal/launch -run TestTheModuleHasNoCommandLineFramework`. Expected: FAIL naming `spf13/cobra`.
- [ ] **Step 3: Delete** the directories and files above; remove the `Prompter` parameter and its callers (the web passes nil today); `go mod tidy`. Then run `go run golang.org/x/tools/cmd/deadcode@latest ./cmd/mm` and delete every function it reports that only the CLI used (review-file code, playlist-import code, CLI-only helpers) together with the tests that only exercised them. In `nix/package.nix` remove the `mkdir -p …/completions` line and the three `mm completion …` lines from `postInstall`; keep `wrapProgram` with `yt-dlp`, `ffmpeg` and `xdg-utils` on the PATH (`OpenBrowser` uses `xdg-open`).
- [ ] **Step 4: Run** `go build ./... && go vet ./... && go test ./...`. Expected: PASS, goldens untouched. If `nix` is available: set `vendorHash = lib.fakeHash;`, `git add -A`, `nix build`, paste the printed hash, `nix build` again.
- [ ] **Step 5: Commit:** `The command line is gone: mm is the web interface`.

---

# Phase 2 — Device profiles

### Task 4: Profile, parsing and validation

**Skill:** superpowers:test-driven-development.

**Files:**
- Create: `internal/device/profile.go`
- Test: `internal/device/profile_test.go`
- Modify: `go.mod`, `go.sum` (add `github.com/pelletier/go-toml/v2`)

**Interfaces:**
- Produces:
```go
type Profile struct {
	Name          string `toml:"name"`
	Label         string `toml:"label"`
	LatinNames    bool   `toml:"latin_names"`
	ComponentMax  int    `toml:"component_max"`
	StemMax       int    `toml:"stem_max"`
	TagWidth      int    `toml:"tag_width"`
	LatinGenre    bool   `toml:"latin_genre"`
	CoverFormat   string `toml:"cover_format"`
	CoverMaxEdge  int    `toml:"cover_max_edge"`
	CoverQuality  int    `toml:"cover_quality"`
	SingleCovers  bool   `toml:"single_covers"`
	ReplayGain    bool   `toml:"replaygain"`
	PlaylistExt   string `toml:"playlist_ext"`
	PlaylistStyle string `toml:"playlist_style"`
}
func ParseProfiles(data []byte) ([]Profile, error)
func (p Profile) Validate() error
```
  Document format: an array of tables, `[[profile]]`. Omitted keys take the generic defaults: `component_max=200`, `cover_format="keep"`, `replaygain=true`, `playlist_ext=".m3u8"`, `playlist_style="relative"`, everything else zero/false. Enums: `cover_format` ∈ {`keep`, `baseline-jpeg`}; `playlist_ext` ∈ {`.m3u`, `.m3u8`}; `playlist_style` ∈ {`relative`, `root-backslash`}. Rules: `name` matches `^[a-z0-9][a-z0-9-]*$`; `component_max ≥ 20`; `0 ≤ stem_max < component_max`; `tag_width ≥ 0`; `tag_width > 0` needs `latin_names`; `baseline-jpeg` needs `cover_max_edge ≥ 1` and `1 ≤ cover_quality ≤ 100`.

- [ ] **Step 1: Write the failing tests:**
  - `TestValidateRejects` (table; each case asserts the error contains the profile name and the key): empty name; `"My Player"`; `component_max = 0`; `stem_max = component_max`; `tag_width = -1`; `tag_width = 12` without `latin_names`; `cover_format = "png"`; baseline-jpeg with `cover_max_edge = 0`; `cover_quality = 101`; `playlist_ext = ".pls"`; `playlist_style = "absolute"`.
  - `TestParseRejectsAnUnknownKey`: `latin_name = true` (typo) → error contains `latin_name`.
  - `TestParseAppliesTheGenericDefaults`: a profile with only `name = "x"` has `ComponentMax == 200`, `CoverFormat == "keep"`, `ReplayGain == true`, `PlaylistExt == ".m3u8"`.
  - `TestParseReadsTheWolfgangShape`: a full `[[profile]]` with `latin_names=true, component_max=100, stem_max=20, tag_width=12, latin_genre=true, cover_format="baseline-jpeg", cover_max_edge=480, cover_quality=85, single_covers=true, replaygain=false, playlist_ext=".m3u", playlist_style="root-backslash"` parses and validates.
- [ ] **Step 2: Run** `go test ./internal/device -run 'TestValidate|TestParse' -v`. Expected: FAIL (undefined `Profile`).
- [ ] **Step 3: Implement** `ParseProfiles` with `toml.NewDecoder(...).DisallowUnknownFields()` decoding into a struct pre-filled with the defaults; call `Validate` per profile; errors read `profile "<name>": <key> <what is wrong>`.
- [ ] **Step 4: Run** the same command. Expected: PASS.
- [ ] **Step 5: Commit:** `A device profile is data: the fields, their defaults and their rules`.

### Task 5: A profile is a Target; the built-ins are data

**Skill:** superpowers:test-driven-development.

**Files:**
- Create: `internal/device/target.go`, `internal/device/builtin.toml`, `internal/device/testdata/targets.golden.json`
- Test: `internal/device/equivalence_test.go`
- Modify: `internal/device/device.go`, `internal/device/names.go`, `internal/device/diagnose.go`

**Interfaces:**
- Consumes: `Profile` (Task 4).
- Produces:
```go
func (p Profile) Target() Target
func ArtistName(name string, width int) string
func AlbumName(name, artist string, width int) string
func PlayerTags(t manifest.Track, width int) (artist, album string)
var Generic Target
```
  `Generic` is built from the `generic` entry of `builtin.toml`. `Wolfgang`, `WolfgangStemWidth` and `PlayerFieldWidth` are removed once the equivalence test passes (the 12 and the 20 come from the profile). `Diagnose` derives its checks from the profile: genre longer than `tag_width` when `latin_genre` and `tag_width > 0`; a name with no Latin letters when `latin_names`; two file names equal only because of the cut when `stem_max > 0`; an artist that looks like several when `tag_width > 0`; a generic profile warns about nothing.

- [ ] **Step 1: Write the failing tests** in `equivalence_test.go`, comparing the profile targets built from `builtin.toml` with the existing hard-coded `generic{}` and `wolfgang{}` (kept until this task ends). Fixture tracks: identified album track; single; unidentified; Arabic artist/album/title with and without hand-set Latin forms; a 80-character title; "The Band (Live)"; "Various Artists"; "Mohamed Abdel Wahab"; album "Name - EP"; a 20-character genre; two tracks whose names collide only after the cut. Tests `TestBuiltinGenericMatchesTheHardCodedGeneric` and `TestBuiltinWolfgangMatchesTheHardCodedWolfgang` assert, for every fixture: equal `Name()`; `Tags` deep-equal; `Layout().ComponentMax/StemMax/Latin` equal and `Fold` equal on sample strings; `CoverFile` equal for `image/jpeg` and `image/png`; `SingleCovers`, `ReplayGain` equal; `PlaylistFile` and `Playlist` bytes equal; `Diagnose` equal (messages and order); `Cover` equal for a PNG, a JPEG, a 3000 px JPEG and a non-image (same error).
- [ ] **Step 2: Run** `go test ./internal/device -run Equivalence -v`. Expected: FAIL (`Profile.Target` and `builtin.toml` missing).
- [ ] **Step 3: Implement** `Profile.Target()` mapping each field as the old structs did (Latin names → `layout.Rules{Latin, Fold: translit.ToASCII}`; `baseline-jpeg` → `coverimg.Baseline(stored, edge, quality)` and `cover.jpg`; `root-backslash` → backslash paths with CRLF and no `#EXTM3U`, `relative` → the existing generic M3U8 writer). Embed `builtin.toml` with `//go:embed`. Thread `width` through `names.go` functions.
- [ ] **Step 4: Run** the equivalence tests until they pass. While the hard-coded structs still exist, make the test write every output it compared into `internal/device/testdata/targets.golden.json` (a `-update` test flag; run it once), and add `TestBuiltinTargetsMatchTheRecordedOutputs`, which compares the profile targets with that file. Only then delete the hard-coded structs, the constants and the two comparison tests, fix callers, and run `go test ./...`. Expected: PASS with `sync-generic.golden` and `sync-wolfgang.golden` unchanged (`git diff --stat internal/app/testdata` empty). The recorded file keeps the old behaviour pinned after its reference implementations are gone.
- [ ] **Step 5: Commit:** `The generic and Wolfgang targets are built from data`.

### Task 6: The registry and the config file

**Skill:** superpowers:test-driven-development.

**Files:**
- Create: `internal/device/registry.go`, `internal/device/testdata/devices_ok.toml`, `devices_typo.toml`, `devices_duplicate.toml`
- Test: `internal/device/registry_test.go`

**Interfaces:**
- Produces:
```go
type ProfileInfo struct {
	Name    string
	Label   string
	Builtin bool
}
type Registry struct{ profiles []Profile }
func Builtin() *Registry
func LoadRegistry(path string) (*Registry, error)
func (r *Registry) Get(name string) (Target, bool)
func (r *Registry) Profiles() []ProfileInfo
func (r *Registry) Targets() []Target
func ConfigPath(getenv func(string) string, home string) string
```
  Order is `generic`, `wolfgang-wmp01`, then user profiles in file order. `LoadRegistry` wraps `fs.ErrNotExist` when the file is missing; callers decide whether that is fine.

- [ ] **Step 1: Write the failing tests:**
  - `TestBuiltinListsGenericThenWolfgang`.
  - `TestAUserProfileComesAfterTheBuiltIns` (from `devices_ok.toml`, `Get("my-player")` works, `Builtin == false`).
  - `TestDuplicateNamesAreRejected` (a user profile named `wolfgang-wmp01`; two user profiles both named `my-player`).
  - `TestGenericIsReserved` (a user profile named `generic` is an error).
  - `TestTheErrorNamesTheFileTheProfileAndTheKey`: `devices_typo.toml` error contains the file path, `my-player`, and the key.
  - `TestMissingFileWrapsErrNotExist` (`errors.Is(err, fs.ErrNotExist)`).
  - `TestConfigPathPrefersMMConfigThenXDGThenHome`.
- [ ] **Step 2: Run** `go test ./internal/device -run 'Registry|Config|Builtin|Reserved|Duplicate' -v`. Expected: FAIL (undefined).
- [ ] **Step 3: Implement** the registry over `ParseProfiles`; reuse `Profile.Target()`.
- [ ] **Step 4: Run** the same. Expected: PASS.
- [ ] **Step 5: Commit:** `Profiles load from a file and are listed after the built-in ones`.

### Task 7: Profiles through launch, app, browse and web (additive)

**Skill:** superpowers:test-driven-development.

**Files:**
- Modify: `internal/launch/launch.go`, `cmd/mm/main.go`, `internal/app/app.go`, `internal/app/playlist.go`, `internal/web/server.go`, `internal/web/sync.go`, `internal/web/api.go`, `internal/browse/track.go`, `internal/browse/fields.go`, `internal/browse/sync.go`
- Test: `internal/launch/launch_test.go`, `internal/app/playlist_test.go`, `internal/web/sync_test.go`, `internal/web/web_test.go`, `internal/browse/views_test.go`

**Interfaces:**
- Consumes: `device.Registry` (Task 6).
- Produces:
```go
func (a *App) UseProfiles(r *device.Registry)
func (a *App) Profiles() *device.Registry
func WithProfiles(r *device.Registry) Option
type ProfileView struct {
	Name    string `json:"name"`
	Label   string `json:"label"`
	Builtin bool   `json:"builtin"`
}
```
  `launch.Options` gains `Config string` and `CheckConfig bool`; flags `--config PATH` and `--check-config`. HTTP: `GET /api/profiles` → `[]ProfileView`; `POST /api/sync/plan` accepts `profile` (a name) next to the legacy `wolfgang` boolean; `GET /api/tracks/{id}?profile=NAME` selects which profile the device tab previews; `browse.TrackOptions` gains `Profile string`; the tab view type is renamed `DeviceView`/`DeviceRow` (same fields) and appears as tab id `device`, while the legacy `wolfgang` tab payload is still emitted when `Wolfgang` is set. A `App` with no registry uses `device.Builtin()`.

- [ ] **Step 1: Write the failing tests:**
  - launch: `TestAnInvalidConfigRefusesToStart` (`--config` pointing at a typo file: exit 1, stderr names the file, `Serve` not called); `TestCheckConfigPrintsProfilesAndExits` (valid: exit 0, stdout lists `generic`, `wolfgang-wmp01`, `my-player`; invalid: exit 1); `TestAMissingDefaultConfigIsFine`; `TestAMissingExplicitConfigFails`.
  - app: `TestPlaylistNamesCollideOnAUserProfile` (user profile with `latin_names`, `playlist_ext=".m3u"`: two playlists whose Latin names give the same file are refused).
  - web: `TestProfilesEndpointListsBuiltinsThenUser`; `TestSyncPlanWithAnUnknownProfileIs400`; `TestSyncPlanForTheLibraryFolderIgnoresTheProfile`; `TestSyncPlanWithAProfileUsesItsRules` (user profile `stem_max=8`: planned file names are cut at 8); `TestTheDeviceTabFollowsTheProfileParameter`.
  - browse: `TestTheDeviceViewMatchesTheLegacyWolfgangViewForTheWolfgangProfile` (same rows and warnings for the same track).
- [ ] **Step 2: Run** `go test ./internal/launch ./internal/app ./internal/web ./internal/browse -v`. Expected: FAIL (undefined `UseProfiles`, missing endpoint, missing flags).
- [ ] **Step 3: Implement.** `launch` resolves `Config` via `ConfigPath` when not given, treats `fs.ErrNotExist` as fine only for the default path, builds the registry, calls `a.UseProfiles` and `web.WithProfiles`. `checkPlaylist` iterates `a.Profiles().Targets()` instead of the package `targets` slice. Sync and track handlers resolve a profile name through the registry. Do not remove `WithWolfgang`, the `wolfgang` request field or `info.wolfgang` yet.
- [ ] **Step 4: Run** `go test ./...`. Expected: PASS.
- [ ] **Step 5: Commit:** `Sync and the device preview take a profile; the config file is read and checked at start`.

---

# Phase 3 — A typed contract

### Task 8: Name every response

**Skill:** superpowers:test-driven-development.

**Files:**
- Modify: `internal/web/api.go:56-59`, `internal/web/server.go:164` and `:181`, `internal/web/sync.go:90`, `internal/web/add.go:68`, `internal/web/edit.go:127`, `internal/web/edit.go:237`
- Test: `internal/web/web_test.go`

**Interfaces:** Produces named structs whose JSON is exactly what the anonymous maps produce today:
```go
type InfoView struct {
	Root     string `json:"root"`
	Version  string `json:"version"`
	Wolfgang bool   `json:"wolfgang"`
	CanAdd   bool   `json:"canAdd"`
	Lookup   bool   `json:"lookup"`
}
type UnsyncedView struct {
	N     int  `json:"n"`
	Known bool `json:"known"`
}
type LibraryResponse struct {
	Stamp    string             `json:"stamp"`
	View     browse.LibraryView `json:"view"`
	Unsynced UnsyncedView       `json:"unsynced"`
	History  browse.History     `json:"history"`
}
type SyncTargetsView struct {
	Targets  []browse.TargetView `json:"targets"`
	Wolfgang bool                `json:"wolfgang"`
}
type PathView struct{ Path string `json:"path"` }
type HintView struct{ Hint string `json:"hint"` }
type ErrorView struct{ Error string `json:"error"` }
```

- [ ] **Step 1: Write the failing test** `TestEveryResponseDecodesIntoItsNamedType`: for each of `/api/info`, `/api/library`, `/api/sync/targets`, the add path preview, the cover path preview, the group hint, and an error response, decode with `json.Decoder.DisallowUnknownFields()` into the named type and assert known values.
- [ ] **Step 2: Run** `go test ./internal/web -run TestEveryResponseDecodesIntoItsNamedType`. Expected: FAIL (types undefined).
- [ ] **Step 3: Replace** each `map[string]any`/`map[string]string` literal with the named struct, keeping every JSON key and shape unchanged.
- [ ] **Step 4: Run** `go test ./...`. Expected: PASS; the old page still works (`go run ./cmd/mm --library /tmp/lib --no-open` and open it).
- [ ] **Step 5: Commit:** `Every response the page reads is a named type`.

### Task 9: Generate the TypeScript types from the Go structs

**Skill:** superpowers:test-driven-development.

**Files:**
- Create: `internal/tsgen/tsgen.go`, `cmd/gentypes/main.go`, `internal/web/contract.go`, `web/src/api/types.gen.ts` (generated)
- Test: `internal/tsgen/tsgen_test.go`, `internal/web/contract_test.go`

**Interfaces:**
- Produces:
```go
func Generate(roots ...any) (string, error)
func Contract() []any
```
  `Contract()` returns a zero value of every type that crosses the wire: all `browse` view types the handlers return, the Task 7/8 types, `JobView`, `SyncData`, the SSE payloads (`library` event data is a JSON string stamp; `job` event data is `JobView`).
  Mapping: struct → `export interface`; `json:"x,omitempty"` → `x?:`; `*T` → `T | null`; `[]T` → `T[]`; `map[string]T` → `Record<string, T>`; `string`/numbers/`bool` → primitives; `time.Time` → `string`; `any`/`json.RawMessage` → `unknown`; embedded structs are flattened; fields tagged `json:"-"` are skipped; output sorted by name, deterministic.

- [ ] **Step 1: Write the failing tests:**
  - tsgen: `TestAStructBecomesAnInterface`, `TestOmitemptyIsOptional`, `TestPointersAreNullable`, `TestSlicesAndMaps`, `TestEmbeddedStructsAreFlattened`, `TestUnknownReplacesAny` (asserts the output never contains the word `any`), `TestOutputIsDeterministic` (generate twice, equal), `TestAFunctionFieldIsAnError`.
  - web: `TestGeneratedTypesAreCurrent`: `Generate(Contract()...)` equals the contents of `../../web/src/api/types.gen.ts`; failure message says `run: go run ./cmd/gentypes > web/src/api/types.gen.ts`.
- [ ] **Step 2: Run** `go test ./internal/tsgen ./internal/web -run 'TestA|TestOmit|TestPointers|TestSlices|TestEmbedded|TestUnknown|TestOutput|TestGenerated'`. Expected: FAIL (undefined / file missing).
- [ ] **Step 3: Implement** `Generate` with `reflect`; `cmd/gentypes` writes `Generate(web.Contract()...)` to stdout; generate the file.
- [ ] **Step 4: Run** the same. Expected: PASS.
- [ ] **Step 5: Commit:** `The page's types are generated from the Go structs and a test says when they drift`.

---

# Phase 4 — Front-end foundation

### Task 10: Scaffold `web/` and let the server take a front end

**Skill:** superpowers:test-driven-development.

**Files:**
- Create: `web/` scaffold (see File Structure), `internal/web/dist/.gitkeep`
- Modify: `internal/web/server.go`, `.gitignore`
- Test: `internal/web/frontend_test.go`, `web/src/App.browser.test.tsx`

**Interfaces:**
- Produces:
```go
func WithFrontend(fsys fs.FS) Option
```
  `//go:embed all:dist` gives `distFiles`. Until Task 22 the default front end is the old `static`. `WithFrontend` replaces it; when the given FS has no `index.html`, `GET /` answers `503` with a page saying: the front end is not built; run `npm --prefix web run build`. `.gitignore` adds `web/node_modules`, `internal/web/dist/*` and `!internal/web/dist/.gitkeep`.
  `web/package.json` scripts: `dev` (`vite`), `build` (`tsc -b && vite build --outDir ../internal/web/dist --emptyOutDir false`, preceded by removing the old `assets/` and `index.html` there), `build:nix` (`tsc -b && vite build --outDir dist`), `check` (`tsc --noEmit && eslint . && vitest run --project unit`), `test:unit`, `test:browser` (`vitest run --project browser`), `test:e2e` (`npm run build && playwright test`), `test` (all three), `gen:types` (`cd .. && go run ./cmd/gentypes > web/src/api/types.gen.ts`). Vite dev server proxies `/api` to `127.0.0.1:7878` with `changeOrigin: false`. ESLint: `@typescript-eslint/no-explicit-any: error`, strict type-checked config.

- [ ] **Step 1: Write the failing tests:**
  - Go: `TestWithoutAFrontendTheServerSays503AndWhatToRun` (empty `fstest.MapFS` → status 503, body contains `npm --prefix web run build`); `TestWithFrontendServesItsIndex` (MapFS with `index.html` → 200, `Cache-Control: no-cache`); `TestAPIRoutesStillWinOverTheFrontend`.
  - Browser: `App.browser.test.tsx`: `renders the application shell` (mount `<App/>`, expect a `banner` landmark with the text `mm`).
- [ ] **Step 2: Run** `go test ./internal/web -run 'Frontend|503'` and `npm --prefix web ci && npm --prefix web run test:browser`. Expected: FAIL (undefined `WithFrontend`; no `App`).
- [ ] **Step 3: Implement** `WithFrontend`, the 503 page, the scaffold, a minimal `App` with a `<header role="banner">`, and pin `@playwright/test` to exactly the version `nix eval --raw nixpkgs#playwright-driver.version` prints (write the version into `web/package.json`; Task 23 relies on it).
- [ ] **Step 4: Run** the same commands plus `npm --prefix web run check`. Expected: PASS.
- [ ] **Step 5: Commit:** `A web/ project, and a server that can take its front end from outside`.

### Task 11: The seeded end-to-end server and the Playwright harness

**Skill:** superpowers:test-driven-development.

**Files:**
- Create: `internal/web/webtest/seed.go`, `internal/web/webtest/fakes.go`, `cmd/e2e-server/main.go`, `web/e2e/fixtures.ts`, `web/e2e/fixtures/devices.toml`, `web/e2e/smoke.spec.ts`
- Modify: `internal/web/web_test.go`, `internal/web/identify_test.go`, `internal/web/add_test.go` (use the shared fakes), `web/playwright.config.ts`
- Test: `internal/web/webtest/seed_test.go`

**Interfaces:**
- Produces:
```go
func Seed(dir string) (*app.App, error)
type FakeSource struct{}
type FakeDownloader struct{}
```
  `Seed` builds the library every UI test uses: Miles Davis / Kind of Blue (So What 1, Blue in Green 3); John Coltrane / Giant Steps (Giant Steps 1); the Arabic artist فيروز / album Songs / title وطني with Latin form `Watani`; one single with no album; one unidentified track; playlists `Late night` (1 track) and `A rather long playlist name that goes on and on and on for a while` (2 tracks). `cmd/e2e-server --addr 127.0.0.1:7979 --dist internal/web/dist --config web/e2e/fixtures/devices.toml` (the fixture defines `my-player` with `latin_names = true`, `tag_width = 12`) serves it with `FakeSource` (two candidates for "So What") and `FakeDownloader` (a silent MP3). Playwright projects: `desktop` (1200×800) and `narrow` (500×700); `webServer` runs the e2e server.
  `web/e2e/fixtures.ts` exports `test` with a fresh server state per test file and helpers `press(key)`, `tree()`, `dialog()`.

- [ ] **Step 1: Write the failing tests:** Go `TestSeedBuildsTheUILibrary` (tracks, the Arabic track's Latin form, both playlists, one unidentified track); e2e `smoke.spec.ts` › `the seeded server answers /api/library with four artists, two playlists and one unidentified track` and `/api/profiles lists generic, wolfgang-wmp01 and my-player` (Playwright's `request` fixture; no page is needed).
- [ ] **Step 2: Run** `go test ./internal/web/webtest` and `npm --prefix web run test:e2e`. Expected: FAIL (package missing; server command missing).
- [ ] **Step 3: Implement** `Seed`, the fakes (moved out of the `_test.go` files so both Go tests and the e2e server share them), the server command, the Playwright config and fixtures.
- [ ] **Step 4: Run** `go test ./... && npm --prefix web run test:e2e`. Expected: PASS.
- [ ] **Step 5: Commit:** `A seeded server for end-to-end tests, and the fakes shared with the Go tests`.

### Task 12: Typed client, store and event stream

**Skill:** superpowers:test-driven-development.

**Files:**
- Create: `web/src/api/client.ts`, `web/src/api/events.ts`, `web/src/state/store.ts`, `web/src/state/actions.ts`
- Test: `web/src/api/client.test.ts`, `web/src/api/events.test.ts`, `web/src/state/store.test.ts`

**Interfaces:**
- Produces:
```ts
export class ApiError extends Error { readonly status: number }
export function get<T>(path: string, init?: { signal?: AbortSignal }): Promise<T>
export function send<T>(method: 'POST' | 'PATCH' | 'PUT' | 'DELETE', path: string, body?: unknown): Promise<T>
export function query(params: Record<string, string | number | boolean | null | undefined>): string
export function subscribe(h: { library(stamp: string): void; job(e: JobView): void; open(): void }): () => void
export type Focus = 'tree' | 'inspector'
export type OverlayState =
  | { kind: 'ask'; head: string; body: string[]; yes: string; onYes: () => void }
  | { kind: 'help' } | { kind: 'sort' } | { kind: 'playlist'; ids: string[] }
  | { kind: 'copy'; id: string; title: string } | { kind: 'bulk'; ids: string[] }
  | { kind: 'identify' } | { kind: 'add' } | { kind: 'doctor' } | { kind: 'sync' }
export interface AppState {
  info: InfoView | null
  lib: LibraryResponse | null
  filter: string
  expanded: Set<string>
  cursor: string | null
  marks: Set<string>
  focus: Focus
  field: number
  tab: number
  detail: TrackView | PeekView | null
  edit: { field: number; text: string } | null
  playing: { id: string; field: string } | null
  status: { text: string; kind: '' | 'bad' | 'good' }
  overlay: OverlayState | null
  busy: boolean
}
export const useApp: UseBoundStore<StoreApi<AppState>>
```
  Behaviour mirrors `internal/web/static/js/api.js` and `state.js`: `X-MM: 1` on every request; JSON bodies; `Content-Type` only when sending JSON; an error body `{error}` becomes `ApiError.message`; a network failure becomes `ApiError("Cannot reach mm. Is it still running?")`; `AbortError` passes through; `expanded` and `tab` persist in `localStorage` guarded by try/catch.

- [ ] **Step 1: Write the failing tests** (node project, `fetch` stubbed): `sends X-MM on every request`; `sets Content-Type only for a JSON body`; `turns an error body into the message`; `turns a failed fetch into the unreachable message`; `rethrows AbortError`; `query skips empty, null and false and writes true as 1`; events: `reconnect after open reloads the library`, `a library event with the same stamp does nothing`; store: `expanded survives a reload`, `a thrown localStorage does not break the store`.
- [ ] **Step 2: Run** `npm --prefix web run test:unit -- src/api src/state`. Expected: FAIL (modules missing).
- [ ] **Step 3: Implement** the client and store over generated types only (no hand-written response types).
- [ ] **Step 4: Run** the same plus `npm --prefix web run check`. Expected: PASS.
- [ ] **Step 5: Commit:** `A typed API client, an event stream and the application state`.

### Task 13: One declarative keymap

**Skill:** superpowers:test-driven-development.

**Files:**
- Create: `web/src/keys/keymap.ts`, `web/src/keys/dispatch.ts`
- Test: `web/src/keys/keymap.test.ts`, `web/src/keys/dispatch.test.ts`

**Interfaces:**
- Produces:
```ts
export type Scope = 'common' | 'tree' | 'inspector'
export type KeyEventLike = Pick<KeyboardEvent, 'key' | 'ctrlKey' | 'metaKey' | 'altKey' | 'shiftKey' | 'isComposing'>
export interface Binding {
  keys: readonly string[]
  scope: Scope
  action: ActionName
  hint?: { label: string; when?: (s: AppState) => boolean }
  help?: { page: HelpPage; text: string }
}
export const bindings: readonly Binding[]
export function keyName(e: KeyEventLike): string
export function resolve(scope: 'tree' | 'inspector', e: KeyEventLike): ActionName | undefined
export function hintsFor(s: AppState): ReadonlyArray<{ keys: string; label: string }>
export function helpPages(): ReadonlyArray<{ page: HelpPage; rows: ReadonlyArray<{ keys: string; text: string }> }>
export function installKeys(target: Document, run: (a: ActionName) => void): () => void
```
  Every key from the README table is a binding (`↑↓ j k`, `PageUp/Down ctrl+u/d`, `Home g End G`, `z`, `→ l ← h`, `enter`, `+ = -`, `space`, `ctrl+a`, `e c v d Delete`, `J K n p`, `i I o`, `a s D`, `u ctrl+r`, `[ ] { } 0`, `x`, `1–4`, `/`, `S`, `?`, `Tab`, `Escape`). `p` means "add to playlist" in the tree and "hear it" in the inspector. The footer hints and the `?` pages are produced from `bindings`; there is no second list.

- [ ] **Step 1: Write the failing tests:**
  - `resolve picks the tree binding in the tree and the inspector binding in the inspector for p`.
  - `common bindings work in both scopes`.
  - `every binding with a hint or help appears in hintsFor / helpPages` (iterate `bindings`).
  - `no two bindings in one scope share a key`.
  - Review Focus 2: `a key typed in an INPUT, TEXTAREA or SELECT is ignored`; `isComposing ignores the key`; `meta and alt combinations are ignored`; `ctrl+r, ctrl+a, ctrl+u, ctrl+d resolve when focus is not in an input`; `the filter box handles only Enter, ArrowDown and Escape`; `keys wait while busy`.
- [ ] **Step 2: Run** `npm --prefix web run test:unit -- src/keys`. Expected: FAIL.
- [ ] **Step 3: Implement** the table and `installKeys`; `ActionName` is a string-literal union defined next to `bindings`.
- [ ] **Step 4: Run** the same plus `npm --prefix web run check`. Expected: PASS.
- [ ] **Step 5: Commit:** `One keymap: the keys, the footer hints and the help come from the same table`.

---

# Phase 5 — Port the interface

Each task here is finished only when its browser tests and e2e tests pass in both Playwright projects, and its behaviour matches the README's description of the old page (`README.md`, "The web interface" and its key table; the old code in `internal/web/static/js/` is the reference for edge cases).

### Task 14: Shell, theme, typography, narrow layout

**Skill:** superpowers:test-driven-development.

**Files:**
- Create: `web/src/components/{Header,Footer,Split}.tsx`, `web/src/styles/tokens.css`, `web/src/styles/app.css`
- Modify: `web/src/App.tsx`
- Test: `web/src/components/Shell.browser.test.tsx`, `web/e2e/shell.spec.ts`

**Interfaces:** Produces `<Header/>` (root path, summary `N tracks`, sync chip `N to sync · press s`, theme button), `<Footer/>` (status line + hints from `hintsFor`), `<Split/>` (draggable divider, width persisted under `mm.left`). Fonts: `system-ui` with `"Noto Sans Arabic", "Noto Naskh Arabic"` fallbacks for text; a monospace face only for times and IDs. Light and dark themes via tokens; dark default; the choice persists under `mm.theme`.

- [ ] **Step 1: Write the failing tests:** browser: `the footer shows the hints for the focused pane`; `the theme button toggles data-theme and remembers it`; `the sync chip is hidden when nothing waits and shows the count when it does`; e2e: `below 800px one pane shows at a time: → into the fields shows the right pane and ← returns to the tree`; `dragging the divider changes the left width and survives a reload`; `no horizontal scrollbar at 500px`.
- [ ] **Step 2: Run** `npm --prefix web run test:browser -- Shell` and `npx --prefix web playwright test shell`. Expected: FAIL.
- [ ] **Step 3: Implement** the three components and styles with CSS custom properties; use logical properties (`margin-inline`, `inset-inline`) throughout.
- [ ] **Step 4: Run** the same. Expected: PASS.
- [ ] **Step 5: Commit:** `The shell: header, footer, panes, theme, a proportional font`.

### Task 15: The tree and the filter

**Skill:** superpowers:test-driven-development.

**Files:**
- Create: `web/src/components/Tree.tsx`
- Test: `web/src/components/Tree.browser.test.tsx`, `web/e2e/tree.spec.ts`

**Interfaces:** Consumes `useApp`, `get<LibraryResponse>('/api/library' + query(...))`. `S` opens the sort dialog built in Task 17. Behaviour: rows from `lib.rows`; cursor; open/close (`enter`, `→ l`, `← h`, twist click); `+ = -` open one level / close all; `space` mark, `ctrl+a` mark all visible; `PageUp/Down`, `Home/End`, `z` centre; `/` focuses the filter (same query words as the server: `status:unidentified`, `genre:folk`); the Playlists group at the top; the Needs work group; a single click puts the cursor on a row, a second opens; marks shown by a marker column.

- [ ] **Step 1: Write the failing tests:** browser: `moves the cursor with j and k and the arrow keys`; `enter opens an artist and shows its albums`; `space marks and an album marks all its tracks`; `the filter box narrows the tree through the server`; `escape clears the filter`. e2e: `the tree lists Playlists, John Coltrane, Miles Davis and فيروز`; `status:unidentified shows only the unidentified track`; `the Arabic artist row is a bdi with dir=auto`.
- [ ] **Step 2: Run** `npm --prefix web run test:browser -- Tree` and `npx --prefix web playwright test tree smoke`. Expected: FAIL.
- [ ] **Step 3: Implement** `Tree` (roles `tree`/`treeitem`, `aria-level`, `aria-selected`); no virtualization (hundreds of rows).
- [ ] **Step 4: Run** the same. Expected: PASS.
- [ ] **Step 5: Commit:** `The tree: rows, cursor, marks, filter`.

### Task 16: The inspector

**Skill:** superpowers:test-driven-development.

**Files:**
- Create: `web/src/components/{Inspector,Fields,Cover}.tsx`
- Test: `web/src/components/Inspector.browser.test.tsx`, `web/e2e/inspector.spec.ts`

**Interfaces:** Consumes `get<TrackView>('/api/tracks/' + id + query({ profile }))`, `get<PeekView>('/api/peek...')`, `send('PATCH', '/api/tracks/' + id, …)`, `send('POST', '/api/tracks/' + id + '/nudge' | '/clear')`, `/api/tracks/{id}/preview` (audio). Right pane shows a track (tabs Details, Audio, Latin, Device), an artist, an album, a playlist or the Needs work group as the old page did. Device tab: a profile `<select>` defaulting to the first non-generic profile from `GET /api/profiles`, then the preview rows and warnings. `→`/`←`/`1–4` switch tabs; `↑↓` choose a field; `enter` edits; `enter` saves, `esc` cancels; `(mixed)` fields open empty (blank keeps, `-` clears); `[ ] { }` nudge cuts and gains, `0` clears, `p` plays the 8 s preview through an `<audio>` element; `x` removes the cover.

- [ ] **Step 1: Write the failing tests:** browser: `enter edits a field and escape cancels`; `a mixed field opens empty and blank keeps every value`; `the device tab previews with the chosen profile`; `p on a cut starts the preview audio`. e2e: `editing the title saves it and the tree row follows`; `the Latin tab shows a hand-set form as normal text and a worked-out one as dim`; `the cover is shown for a track that has one`.
- [ ] **Step 2: Run** `npm --prefix web run test:browser -- Inspector` and `npx --prefix web playwright test inspector`. Expected: FAIL.
- [ ] **Step 3: Implement.** Field rendering and editing state live in the store (`edit`), the server owns validation; show its error text under the field.
- [ ] **Step 4: Run** the same. Expected: PASS.
- [ ] **Step 5: Commit:** `The inspector: tabs, fields, editing, cuts, preview`.

### Task 17: Overlays, the simple dialogs and the layout regression

**Skill:** superpowers:test-driven-development.

**Files:**
- Create: `web/src/overlay/{OverlayHost,Dialog}.tsx`, `web/src/dialogs/{Ask,Help,SortMenu,PlaylistPicker,CopyList}.tsx`
- Test: `web/src/dialogs/Dialogs.browser.test.tsx`, `web/e2e/dialogs.spec.ts`

**Interfaces:** Produces `<Dialog title hints onKey>` (focus trap, `role="dialog"`, `aria-modal`, title in the border, backdrop click closes, focus returns to where it was) and `<OverlayHost/>` switching on `useApp().overlay`. `PlaylistPicker` behaviour as the old one: a list of playlists with counts plus `New playlist…`; `n` names a new one; with no playlists it opens straight on the name input; `esc` goes back from naming to the list, else closes. A shared `MenuList` renders every menu row with the same three cells (marker, label, detail) so a row can never be short a cell again.

- [ ] **Step 1: Write the failing tests:**
  - e2e `the playlist picker rows line up` (the regression for the bug fixed in commit 474abd5, ported from `internal/web/layout_test.go`), at both Playwright projects, with the seeded `Late night` (1) and the long-named playlist (2): press `p` on a track; every row's cells share one top and height; columns line up from row to row; no cell overlaps its neighbour; each cell lies inside the dialog; the label column is at least half the dialog width; the count is right of the label; `Late night` and `New playlist…` are one line each; the long name may wrap; the dialog lies inside the window.
  - browser: `ask goes ahead only on y or enter`; `escape closes and focus returns`; `the sort menu picks by number`; `the copy list cannot tick an attribute the track lacks`; `n turns the picker into a name input`.
  - e2e: `the help dialog lists every binding by page and ← → turns the pages`; `S then 3 sorts by title into one flat list and choosing the tree goes back to the tree`.
- [ ] **Step 2: Run** `npm --prefix web run test:browser -- Dialogs` and `npx --prefix web playwright test dialogs`. Expected: FAIL.
- [ ] **Step 3: Implement** the overlay host, the dialogs and `MenuList`; the Help pages come from `helpPages()`.
- [ ] **Step 4: Run** the same. Expected: PASS.
- [ ] **Step 5: Commit:** `Dialogs: one shell, one menu list, and the playlist picker pinned by a layout test`.

### Task 18: Editing operations

**Skill:** superpowers:test-driven-development.

**Files:**
- Create: `web/src/dialogs/BulkForm.tsx`
- Modify: `web/src/state/actions.ts`, `web/src/components/Tree.tsx`, `web/src/components/Inspector.tsx`
- Test: `web/src/state/edits.test.ts`, `web/e2e/editing.spec.ts`

**Interfaces:** Consumes `/api/bulk/preview`, `/api/bulk`, `/api/group`, `/api/group/hint`, `/api/copy`, `/api/paste`, `/api/remove`, `/api/renumber`, `/api/move`, `/api/playlists`, `/api/playlists/add`, `/api/playlists/takeout`, `/api/playlists/delete`, `/api/undo`, `/api/redo`, `PUT/DELETE /api/cover`. Behaviour: `e` edits marked tracks in one form with a live summary (debounced 150 ms, only the latest answer shown); `c` on a track opens the copy list, `v` pastes on marks or the cursor target, `ctrl+v` with an image on the clipboard and a dropped picture file set the cover; `d` asks first; `J K` move a track in its album or playlist, `n` renumbers; `u` / `ctrl+r`; `p` adds the marked tracks (or the cursor's track, album or artist) to a playlist; in a playlist `d` removes a track from it, `d` on the playlist deletes it after asking; renaming an artist or album that equals another joins them and the pane says so while typing.

- [ ] **Step 1: Write the failing tests:** unit: `a stale bulk preview answer is dropped`; `busy blocks keys until the change returns`; e2e: `marking two tracks and e sets the album on both with one undo step`; `d asks and y removes the track and it disappears from its playlists`; `J moves a track down and renumbers`; `u then ctrl+r undoes and redoes`; `pasting a copied cover`; `p adds the selected album to Late night and the count becomes 3`; `renaming an artist to another artist's name says it joins them`; `dropping a non-image on the page is refused with a message`.
- [ ] **Step 2: Run** `npm --prefix web run test:unit -- src/state` and `npx --prefix web playwright test editing`. Expected: FAIL.
- [ ] **Step 3: Implement** the actions and bulk form.
- [ ] **Step 4: Run** the same. Expected: PASS.
- [ ] **Step 5: Commit:** `Editing: bulk, copy and paste, move, remove, undo, playlists`.

### Task 19: Identify, adopt, add, doctor, ingest and jobs

**Skill:** superpowers:test-driven-development.

**Files:**
- Create: `web/src/dialogs/{Identify,Add,Doctor}.tsx`
- Modify: `web/src/state/actions.ts`
- Test: `web/e2e/identify.spec.ts`, `web/e2e/add.spec.ts`

**Interfaces:** Consumes `/api/tracks/{id}/candidates`, `/api/identify/apply`, `/api/identify-all`, `/api/adopt`, `/api/add/fetch|path|finish`, `DELETE /api/add/{draft}`, `/api/ingest`, `/api/doctor`, `/api/jobs`, `/api/jobs/{id}` and the SSE `job` events. Keys `i I o a D`. Dropped MP3 files are ingested as unidentified tracks; the add form is pre-filled from the match and shows its path preview and duplicates; progress bars follow `JobView`.

- [ ] **Step 1: Write the failing tests:** e2e (with `FakeSource` / `FakeDownloader`): `i on the unidentified track lists candidates, shows what would change, and applying identifies it`; `I identifies all with a progress bar`; `o adopts the file's old tags`; `a adds from a link, pre-filled, and the path preview shows where it will be written`; `a duplicate is flagged`; `dropping two mp3 files adds two unidentified tracks`; `D lists the problems and a click moves the cursor to the track`.
- [ ] **Step 2: Run** `npx --prefix web playwright test identify add`. Expected: FAIL.
- [ ] **Step 3: Implement** the dialogs; job progress uses the store and `subscribe`.
- [ ] **Step 4: Run** the same. Expected: PASS.
- [ ] **Step 5: Commit:** `Identify, add, doctor and drop-to-ingest`.

### Task 20: The sync dialog with a profile choice

**Skill:** superpowers:test-driven-development.

**Files:**
- Create: `web/src/dialogs/Sync.tsx`
- Test: `web/e2e/sync.spec.ts`

**Interfaces:** Consumes `GET /api/sync/targets`, `GET /api/profiles`, `POST /api/sync/plan {library, folder, profile}`, `POST /api/sync/{plan}/apply`, `/prune`, `DELETE /api/sync/{plan}`. Steps as before: choose the library folder, a mounted drive or a typed folder; a Format row (profile `<select>`; `w` cycles) shown only for a folder other than the library, default `generic`, never remembered; review the plan with counts and warnings; apply; strays are deleted only after typing `delete`.

- [ ] **Step 1: Write the failing tests:** e2e: `s opens the targets and the library folder has no format row`; `a typed folder offers every profile, generic first`; `choosing wolfgang-wmp01 shows its warnings in the review`; `a profile from the user's devices.toml appears` (the e2e server gets a `--config` fixture with `my-player`); `strays are removed only after typing delete`; `the header says N to sync · press s and the chip opens the dialog`.
- [ ] **Step 2: Run** `npx --prefix web playwright test sync`. Expected: FAIL.
- [ ] **Step 3: Implement** the dialog; send `profile`, never the legacy `wolfgang` field.
- [ ] **Step 4: Run** the same. Expected: PASS.
- [ ] **Step 5: Commit:** `Sync from the new page, with a choice of device profile`.

### Task 21: Parity audit, right-to-left and empty states

**Skill:** superpowers:test-driven-development.

**Files:**
- Create: `web/e2e/parity.spec.ts`, `web/e2e/rtl.spec.ts`, `web/e2e/empty.spec.ts`, `docs/superpowers/notes/ui-parity.md`
- Modify: whatever the audit finds

**Interfaces:** Consumes everything above. `ui-parity.md` is a checklist with one line per row of the README key table and per paragraph of "The web interface"; each line names the test that pins it or `NEW`.

- [ ] **Step 1: Write the failing tests** for every `NEW` line, plus: Review Focus 3 — `Arabic rows keep dir=auto and the count column does not move` (compare the count's x in an Arabic row and a Latin row), `the Latin tab and the Device tab show the Latin form for the Arabic track`, `sorting by artist puts فيروز among the F`, `a mixed Latin and Arabic title does not reorder its neighbours` (the bdi's text content order is preserved); Review Focus 5 — `an empty library shows an empty tree and keys do nothing harmful`, `a library without playlists makes p open on the name input`, `an unidentified track shows its fields and the Needs work group lists it`, `an empty playlist opens with no rows and d deletes it after asking`; mouse — `click selects, double-click opens, click on a field then again edits, the twist arrow opens at once`.
- [ ] **Step 2: Run** `npx --prefix web playwright test parity rtl empty`. Expected: FAIL for exactly the gaps.
- [ ] **Step 3: Fix** each gap in the component the test points at.
- [ ] **Step 4: Run** `npm --prefix web run test`. Expected: PASS (unit, browser and e2e).
- [ ] **Step 5: Commit:** `The new page matches the old one, and right-to-left and empty cases are pinned`.

---

# Phase 6 — Cutover and Nix

### Task 22: Cut over and delete the old page

**Skill:** superpowers:test-driven-development.

**Files:**
- Delete: `internal/web/static/`, `internal/web/static_test.go`, `internal/web/layout_test.go`, `internal/web/testdata/playlist_picker_probe.js`
- Modify: `internal/web/server.go`, `internal/web/sync.go`, `internal/web/api.go`, `internal/browse/track.go`, `internal/launch/launch.go`, `cmd/mm/main.go`, `internal/web/contract.go`, `web/src/api/types.gen.ts`
- Test: `internal/web/web_test.go`, `internal/browse/views_test.go`

**Interfaces:** Produces: the default front end is `distFiles`; `WithWolfgang`, `Server.wolfgang`, `info.wolfgang`, `SyncData.Wolfgang`, `TrackOptions.Wolfgang`, the legacy `wolfgang` tab payload and `SyncTargetsView.Wolfgang` no longer exist; `go:embed static` is gone.

- [ ] **Step 1: Write the failing tests:** `TestTheLegacyWolfgangFieldIsGone` (a sync plan body with `wolfgang: true` and no `profile` is planned with `generic`, the unknown field ignored; `/api/info` and `/api/sync/targets` have no `wolfgang` key); `TestWithoutOptionsTheServerServesTheEmbeddedDist` (a server without `WithFrontend` answers `/` with 200 when `fs.Stat(distFiles, "dist/index.html")` succeeds and with 503 otherwise, so it passes both in the Nix sandbox and after a local build); `TestGeneratedTypesAreCurrent` keeps passing after the removals.
- [ ] **Step 2: Run** `go test ./internal/web ./internal/browse`. Expected: FAIL.
- [ ] **Step 3: Delete and remove** as listed; regenerate `types.gen.ts`; fix compile errors; remove `web.WithWolfgang()` from `cmd/mm/main.go`.
- [ ] **Step 4: Run** `go vet ./... && go test ./... && npm --prefix web run test`. Expected: PASS; then `npm --prefix web run build && go run ./cmd/mm --library /tmp/lib` and use the page by hand: browse, edit a field, `p`, `s` to a temp folder with each profile.
- [ ] **Step 5: Commit:** `The new page is the page; the old one and the legacy Wolfgang fields are gone`.

### Task 23: Nix packaging for the TypeScript front end

**Skill:** superpowers:test-driven-development (the failing test is `nix flake check` failing before the derivations exist).

**Files:**
- Create: `nix/frontend.nix`
- Modify: `nix/package.nix`, `flake.nix`
- Test: flake `checks`

**Interfaces:** Produces `packages.<system>.frontend` (the built `dist`), `packages.<system>.music-manager` (Go with the dist embedded), `checks.<system>.{frontend,build}`, `devShells.default` with `go`, `nodejs`, `yt-dlp`, `ffmpeg`, `playwright-driver.browsers`, env `PLAYWRIGHT_BROWSERS_PATH`, `PLAYWRIGHT_SKIP_VALIDATE_HOST_REQUIREMENTS=true`. `frontend.nix` uses `buildNpmPackage` with `src = ../web`, `npmBuildScript = "build:nix"`, `doCheck = true` with `checkPhase = "npm run check"` (type-check, ESLint, node unit tests; no browser), and installs `dist` to `$out`. `package.nix` copies the frontend into `internal/web/dist` in `postPatch`, keeps `vendorHash`, and its `lib.fileset` additionally includes `../web/src/api/types.gen.ts` so `TestGeneratedTypesAreCurrent` runs in the sandbox.

- [ ] **Step 1: Write the failing check:** add `checks.<system>.frontend = self.packages.<system>.frontend;` to `flake.nix` and run `nix flake check`. Expected: FAIL (attribute missing).
- [ ] **Step 2: Implement** `nix/frontend.nix` (`npmDepsHash = lib.fakeHash;`, build once, paste the hash printed; `nix run nixpkgs#prefetch-npm-deps web/package-lock.json` also prints it) and the `package.nix` wiring. `git add` every new file first.
- [ ] **Step 3: Run** `nix flake check` and `nix build`. Expected: PASS; `result/bin/mm --version` prints the version; `result/bin/mm --library /tmp/lib --no-open` serves a page whose `/` returns 200.
- [ ] **Step 4: Run** the dev-shell tests: `nix develop -c npm --prefix web ci && nix develop -c npm --prefix web run test`. Expected: PASS (unit, browser, e2e). If Playwright rejects the browsers, the cause is a version mismatch between `@playwright/test` and `playwright-driver`; the pin from Task 10 must equal `nix eval --raw nixpkgs#playwright-driver.version`.
- [ ] **Step 5: Commit:** `Nix builds the front end and embeds it; the dev shell runs the browser tests`.

### Task 24: The home-manager module

**Skill:** superpowers:test-driven-development (failing checks first).

**Files:**
- Create: `nix/hm-module.nix`
- Modify: `flake.nix`
- Test: flake `checks.<system>.hm-module`

**Interfaces:** Produces `homeManagerModules.default` with options under `programs.music-manager`: `enable`, `package` (default `self.packages.${system}.default`), `library` (`null` or a path string → `home.sessionVariables.MM_LIBRARY`), `devices` (`attrsOf` a submodule whose options mirror the `Profile` fields: `label`, `latinNames`, `componentMax`, `stemMax`, `tagWidth`, `latinGenre`, `coverFormat`, `coverMaxEdge`, `coverQuality`, `singleCovers`, `replaygain`, `playlistExt`, `playlistStyle`, every one `null` by default and dropped from the output when `null`, so the Go defaults apply). It writes `xdg.configFile."mm/devices.toml"` from `(pkgs.formats.toml {}).generate` as `{ profile = [ { name = <attr name>; … } ]; }` only when `devices != {}`.

- [ ] **Step 1: Write the failing checks** in `flake.nix`: `hm-module` evaluates the module with `lib.evalModules` and stub `home.packages`, `home.sessionVariables`, `xdg.configFile` options and a sample config (`my-player` with `latinNames = true; tagWidth = 12; coverFormat = "baseline-jpeg"; coverMaxEdge = 480; coverQuality = 85`), builds the generated file, and runs `${mm}/bin/mm --config <file> --check-config`; and a second derivation `hm-module-rejects-bad` that generates a profile with `coverFormat = "png"` and asserts `! mm --config <file> --check-config`. Run `nix flake check`. Expected: FAIL (module missing).
- [ ] **Step 2: Implement** the module and export it from `flake.nix`.
- [ ] **Step 3: Run** `nix flake check`. Expected: PASS (valid config accepted, invalid rejected with the file and key named).
- [ ] **Step 4: Commit:** `A home-manager module that writes devices.toml`.

### Task 25: Documentation and the final verification

**Skill:** superpowers:test-driven-development (documentation claims are checked against the program, see Step 2).

**Files:**
- Modify: `README.md`
- Create: none

- [ ] **Step 1:** Rewrite `README.md`: remove every CLI command and the `mm ls/edit/review/playlist` sections; document `mm` (flags `--library --addr --no-open --config --check-config --version`), the web interface and its keys (generated list equals `helpPages()`), device profiles (the full field table from Task 4 with the Wolfgang profile as the worked example and `devices.toml` location), the home-manager module, the new Development section (`go test ./...`, `npm --prefix web run test`, `go run ./cmd/e2e-server`, `go run ./cmd/gentypes`, `nix flake check`, `nix develop`), and installing with Nix. Keep the sections on the record/render model, covers, Latin forms, the Wolfgang limits, volume and fades, ReplayGain, dates and sorting, playlists in the web UI.
- [ ] **Step 2:** Verify each claim: run `mm --check-config` against the README's `devices.toml` example (copy it into a temp file); `grep -n "mm add\|mm ls\|mm edit\|mm playlist\|mm review\|--wolfgang" README.md` must print nothing; compare the key table with `bindings` by eye once.
- [ ] **Step 3: Run the whole verification:** `go vet ./... && go test ./... && npm --prefix web run check && npm --prefix web run test && nix flake check`. Expected: all PASS; `git status --short` shows only intended files.
- [ ] **Step 4: Commit:** `The README describes the web-only program, device profiles and the new development workflow`.

---

## Self-review (completed)

- **Spec coverage:** D1 → Tasks 1–3; D2 → unchanged, guarded by the goldens in Tasks 5, 22; D3 → Tasks 13, 14, 17; D4 → Task 2 plus the guard constraint and Task 10 proxy note; D5 → Tasks 4–7, 20; D6 → Task 16 keeps `p` preview, no player task by design; D7 → Tasks 8–22; D8 → Task 23; D9 → Task 24. Review Focus 1 → Tasks 4, 6, 7; 2 → 13; 3 → 21; 4 → 10; 5 → 21.
- **Type consistency:** `Profile`/`Registry`/`ProfileInfo`/`ProfileView`/`UseProfiles`/`WithProfiles`/`WithFrontend`/`Contract`/`Generate`, `useApp`/`OverlayState`/`bindings`/`hintsFor`/`helpPages` are defined once and reused with the same names.
- **Known judgement calls for review:** Task 1 uses mutation as the "watch it fail" step because the behaviour already exists; Task 5 records the old targets' outputs as a golden file before deleting them; the in-repo TypeScript generator replaces a library on purpose (see Assumptions).
