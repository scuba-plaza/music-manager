# mm: web-only, device profiles, TypeScript front end — Design

**Status:** for review. **Date:** 2026-10-30. **Branch:** `claude/optimistic-thompson-eignis`.
**Implemented by:** [`docs/superpowers/plans/2026-10-30-web-only-profiles-ts-frontend.md`](../plans/2026-10-30-web-only-profiles-ts-frontend.md). Where the two disagree, this spec wins.

## 1. Purpose

`mm` is a music library manager. It keeps one record per track and renders a music folder from those records; the folder is output. It feeds two players: Gapless (GNOME, on the laptop) and the Wolfgang WMP01 (a small MP3 player with hard display limits).

Three problems motivate this work:

1. **The page cannot be tested properly.** It is 3.1k lines of untyped vanilla JavaScript with no toolchain. The first browser test in the repo was written on 2026-10-30, after a playlist dialog shipped visibly broken (menu rows had two cells for a three-column grid).
2. **The CLI duplicates the web page.** About 20 commands, 1.4k lines of source and 1.5k of tests, for a program whose interface is the web page.
3. **Devices are code, not data.** The Wolfgang's rules are a hard-coded Go type, so a second player means editing Go.

**Success** means: one binary that starts the local web server; devices described as data with the Wolfgang as a built-in profile; a typed, tested React front end that matches today's behaviour; first-class Nix packaging; and no change to what a sync writes for the existing targets.

## 2. Scope

**In scope:** removing the CLI; device profiles read from `devices.toml`; named JSON responses and generated TypeScript types; a new front end in `web/`; Nix packaging for it and a home-manager module for the profiles; the README.

**Out of scope (decided, not forgotten):** remembering a card by label or UUID; an in-app player (Gapless stays the player; the 8-second cut preview stays); an always-on service or systemd unit; SQLite or any change to the record model; an OpenSubsonic API; authentication (the server stays loopback-only); any change to MP3 editing (`internal/mp3`), loudness, covers, Latin forms or MusicBrainz/yt-dlp integration.

## 3. Decisions

| # | Decision | Why | Rejected |
|---|---|---|---|
| D1 | Drop the CLI entirely, including `review apply`, `playlist import`, the interactive `mm add` form and shell completions. `mm` starts the server. | The web page covers everything else; one interface to keep correct. | Keep a thin CLI; port the two CLI-only features. |
| D2 | Keep records plus rendered folder. | It makes trims, gain, fades and undo non-destructive and lets one library render several targets. | SQLite; files-as-truth like beets. |
| D3 | Drop the terminal look. Proportional UI font with Arabic fallbacks. Keep the keyboard shortcuts as one declarative keymap that also produces the footer hints and the help pages. | The look was a workaround for the TUI; shortcuts are the fast path. The keys are currently listed in three hand-synced places. | Conventional toolbar UI; keep the look. |
| D4 | Keep the on-demand local server on `127.0.0.1:7878`. | Simplest to package and the safest. | Always-on service; desktop shell. |
| D5 | Device profiles as data: built-in `generic` and `wolfgang-wmp01`, plus user profiles in `devices.toml`. The library folder is always `generic`. | Adds a device without writing Go. | Hard-coded targets; remembering cards. |
| D6 | No in-app player. | Gapless does gapless playback well. | Browser playback. |
| D7 | Keep the Go core and rebuild only the front end, in TypeScript. | The core is the asset: about 13.9k lines of source with about 13.7k lines of tests, goldens and measured device limits. UI testing is a front-end tooling question, not a back-end language question. | Full TypeScript, Rust, Python or desktop rewrites. |
| D8 | Front end: React 19, Vite, zustand, Vitest 4 (node unit project and Browser Mode project), Playwright Test. | Most mature test tooling and the most familiar stack. | Svelte, Solid, plain TypeScript. |
| D9 | Browser tests run in the dev shell only; Go tests, `tsc`, ESLint and node unit tests run in `nix flake check`. | There is no verified recipe for a headless browser in the Nix sandbox. | Browser tests inside `nix flake check`. |
| D10 | **pnpm 10**, pinned. Not npm, not Bun. | See below. | npm; Bun. |
| D11 | A home-manager module writes `devices.toml` from Nix options. | First-class Nix support. | Config file only. |
| D12 | TypeScript types are generated from the Go structs by a small in-repo reflect-based tool. | The only generator found (`tygo`) last tagged a release in 2022; the type set is about 40 structs. | A third-party generator. |
| D13 | Config is TOML, decoded strictly with `github.com/pelletier/go-toml/v2`. | Comments and readable defaults for a hand-edited file; strictness catches typos. | JSON; YAML. |

**D10, pnpm over Bun.** Bun would have to offer something we need that pnpm cannot. It does not:

- *Nix.* nixpkgs has first-class pnpm support: `pnpm_10`, `fetchPnpmDeps` and `pnpmConfigHook` (older `pnpm.fetchDeps` and `pnpm.configHook` are deprecated), with a `fetcherVersion` that must be set. Bun relies on the third-party `bun2nix`, which needs a generated `bun.nix` kept in sync with `bun.lock`.
- *Toolchain.* Vite and Vitest are the toolchain, and they run on Node. Vitest's browser-mode docs show Bun only as an install command; nothing found confirms Vite or Vitest run under Bun's runtime. Bun's own strengths (a bundled test runner, a bundler, single-file compile) are things we do not need.
- *pnpm 10 behaviour to design for.* Dependency lifecycle scripts do not run by default, so `esbuild` must be listed in `pnpm.onlyBuiltDependencies`. pnpm 10 may also try to switch to the version named in `packageManager`; the Nix build must switch that off. pnpm 11 is reported to move this setting to `allowBuilds`; upgrading is out of scope.

## 4. Starting point

- Go 1.26, module `music-manager`, `CGO_ENABLED=0`. Packages: `app` (operations), `browse` (the views a screen needs; "no idea how it is drawn", values marshal to JSON; 174 JSON-tagged fields), `device` (targets), `layout`, `manifest`, `mp3`, `coverimg`, `loudness`, `musicbrainz`, `ytdlp`, `web` (HTTP: API, server-sent events, embedded page), `cli`, `prompt`.
- `device.Target` is already the seam: 11 methods (`Name`, `Layout`, `Tags`, `Cover`, `CoverFile`, `SingleCovers`, `ReplayGain`, `PlaylistFile`, `Playlist`, `Diagnose`), implemented by `Generic` and `Wolfgang`. `app.SyncOptions.Device` takes a `Target`; `app/playlist.go` checks playlist-name collisions across a hard-coded list of two targets.
- The JavaScript does rendering and key handling only; sorting, filtering, Latin forms and the Wolfgang rules are in Go. The README's key table is the behavioural reference for the page.
- Nix: `nix/package.nix` uses `buildGoModule` with a `vendorHash` and `lib.fileset` over `go.mod`, `go.sum`, `cmd`, `internal`; `nix build` runs the Go tests in the sandbox.
- The server's guard: Host must be loopback; a request that changes anything needs the `X-MM` header and an `Origin` equal to the Host.

## 5. Architecture

```
                     devices.toml (optional)
                            |
  mm  ->  launch.Run  ->  device.Registry (built-ins + user)  ->  app.App (records, blobs, sync)
              |                                                        |
              +-----------------> web.Server  <-----------------------+
                                  |  JSON API + SSE  (named structs)
                                  |  embedded internal/web/dist
                                  v
   browser: React SPA (web/) -- types generated from the Go structs (internal/tsgen)
```

## 6. Design

### 6.1 Entry point

`mm [--library DIR] [--addr ADDR] [--no-open] [--config FILE] [--check-config] [--version]`. No subcommands. Defaults: library `$MM_LIBRARY` else `~/Music/Library`; address `127.0.0.1:7878`, falling back to a free port unless `--addr` was given; config per 6.2. Unknown flags exit 2 and name the flag; a server error exits 1 as `mm: <error>`. `--check-config` validates the config, prints the profile names and exits (0 valid, 1 invalid). The `MM_MUSICBRAINZ_URL`, `MM_COVERART_URL` and `MM_CONTACT` variables keep working. `launch.Run` takes an `Env` (args, getenv, home, streams, a `Serve` function) so it is testable without a network.

### 6.2 Device profiles

A profile is `device.Profile`; `Profile.Target()` returns a `Target`, so `app` and `browse` do not change shape. Document format: an array of tables, `[[profile]]`. Omitted keys take the generic defaults, so a user profile states only what differs.

| key | type | default (generic) | meaning | Wolfgang WMP01 |
|---|---|---|---|---|
| `name` | string | required | `^[a-z0-9][a-z0-9-]*$`, unique | `wolfgang-wmp01` |
| `label` | string | the name | shown in the UI | `Wolfgang WMP01` |
| `latin_names` | bool | false | file, folder and playlist names use Latin forms (folded to ASCII) | true |
| `component_max` | int | 200 | longest path component | 100 |
| `stem_max` | int | 0 (off) | longest file name before the extension | 20 |
| `tag_width` | int | 0 (off) | width of artist and album tags; shortened by the existing rules (initials, particles, brackets, "Various") | 12 |
| `latin_genre` | bool | false | genre tag in Latin letters | true |
| `cover_format` | `keep` or `baseline-jpeg` | `keep` | `baseline-jpeg` re-encodes to baseline JFIF 4:2:0 | `baseline-jpeg` |
| `cover_max_edge` | int | 0 | pixels, longer side | 480 |
| `cover_quality` | int | 0 | 1–100 | 85 |
| `single_covers` | bool | false | a single gets a cover file beside it | true |
| `replaygain` | bool | true | write ReplayGain tags | false |
| `playlist_ext` | `.m3u8` or `.m3u` | `.m3u8` | playlist file extension | `.m3u` |
| `playlist_style` | `relative` or `root-backslash` | `relative` | `relative`: UTF-8, `#EXTM3U`, `#EXTINF`, paths relative to the playlist. `root-backslash`: root-relative paths with backslashes, CRLF, no header | `root-backslash` |

**Validation (every rule has a test):** name matches the pattern; `component_max ≥ 20`; `0 ≤ stem_max < component_max`; `tag_width ≥ 0`; `tag_width > 0` requires `latin_names`; `baseline-jpeg` requires `cover_max_edge ≥ 1` and `1 ≤ cover_quality ≤ 100`; enums as listed; unknown keys are errors. Errors read `profile "<name>": <key> <what is wrong>`, and a file error also names the file.

**Registry.** Order: `generic`, `wolfgang-wmp01`, then user profiles in file order. `generic` is reserved for the library folder and cannot be redefined; duplicate names (including a built-in's) are errors. Built-ins are an embedded `builtin.toml` parsed by the same loader. Config path: `--config`, else `$MM_CONFIG`, else `$XDG_CONFIG_HOME/mm/devices.toml`, else `~/.config/mm/devices.toml`. A missing default file means built-ins only; a missing explicit file, or any invalid file, refuses to start (writing the wrong format to a card is worse than not starting).

**Diagnostics** derive from the fields: genre longer than `tag_width` (when `latin_genre`); a name with no Latin letters (when `latin_names`); two file names equal only because of the cut (when `stem_max > 0`); an artist that looks like several (when `tag_width > 0`). A profile with none of these set warns about nothing.

**Equivalence guarantee.** The built-in profiles must produce byte-identical results to the hard-coded targets they replace. `sync-generic.golden` and `sync-wolfgang.golden` never change; the old targets' outputs over a fixture set are recorded in a golden JSON before the old code is deleted.

**API.** `GET /api/profiles` returns `[{name, label, builtin}]`. `POST /api/sync/plan` takes `profile` (a name); the library folder ignores it; an unknown name is 400. `GET /api/tracks/{id}?profile=NAME` selects the profile the Device tab previews. During the migration the legacy `wolfgang` boolean and `info.wolfgang` stay, so the old page keeps working; they are removed at the cutover.

### 6.3 API contract and generated types

Every response is a named Go struct (six anonymous maps are replaced; the JSON stays identical). `web.Contract()` lists every type that crosses the wire, including the server-sent event payloads (`library`: a JSON string stamp; `job`: `JobView`). `internal/tsgen.Generate(roots ...any)` emits deterministic TypeScript: struct → `interface`; `omitempty` → optional; pointer → `T | null`; slice → `T[]`; map → `Record<string, T>`; `time.Time` → `string`; `any` and `json.RawMessage` → `unknown`; embedded structs flattened; `json:"-"` skipped; a function-typed field is an error. A Go test fails when `web/src/api/types.gen.ts` differs from the generator's output. The front end uses only generated types for server data.

### 6.4 Front end

- **Layout:** `web/` with `src/{api,state,keys,components,overlay,dialogs,styles}`, unit tests as `*.test.ts`, browser tests as `*.browser.test.tsx`, end-to-end tests in `web/e2e/`. Builds into `internal/web/dist/`, embedded with `//go:embed all:dist`; only `.gitkeep` is tracked there, so Go builds without the front end.
- **State:** one zustand store mirroring today's state (`info`, `lib`, `filter`, `expanded`, `cursor`, `marks`, `focus`, `field`, `tab`, `detail`, `edit`, `playing`, `status`, `overlay`, `busy`). `expanded`, `tab`, the left-pane width and the theme persist in `localStorage`, always inside try/catch.
- **Client:** `X-MM: 1` on every request; JSON bodies; `{error}` bodies become `ApiError.message`; a failed fetch becomes "Cannot reach mm. Is it still running?"; aborts pass through; an `EventSource` stream with reconnect-then-reload.
- **Keymap:** one table of bindings (`keys`, `scope` of common/tree/inspector, `action`, optional footer hint, optional help entry). Resolution, hints and the `?` pages all come from it. Keys are ignored while focus is in an input, textarea or select, during IME composition, and with meta or alt held; `ctrl+r`, `ctrl+a`, `ctrl+u` and `ctrl+d` work outside inputs; keys wait while a change is in flight.
- **Overlays:** `OverlayState` is a discriminated union; one `OverlayHost` renders it; every dialog is a `Dialog` (focus trap, `role="dialog"`, title in the border, backdrop click closes, focus returns). Every menu is rendered by one `MenuList` that always emits the same three cells per row, so a short row cannot recur.
- **Look:** `system-ui` with `Noto Sans Arabic` and `Noto Naskh Arabic` fallbacks; a monospace face only for times and IDs; dark and light themes through CSS custom properties; logical CSS properties throughout; below 800 px one pane at a time. Every displayed name is a `<bdi dir="auto">`.
- **Behaviour reference:** the README's key table and "The web interface" section; `internal/web/static/js/` is the reference for edge cases until it is deleted at the cutover.

### 6.5 Testing

| Layer | Tool | Runs in |
|---|---|---|
| Go: profiles, registry, launch, API, generator, goldens | `go test` | `nix flake check` (no browser, network or built front end needed) |
| TypeScript: types, lint, node unit tests (client, store, keymap) | `tsc`, ESLint, Vitest `unit` project | `nix flake check` |
| Components in a real browser | Vitest 4 Browser Mode, Playwright provider | dev shell |
| End to end against the real Go server with a seeded library | Playwright Test, projects `desktop` (1200×800) and `narrow` (500×700) | dev shell |

Rules: tests assert DOM and layout metrics (rects, line counts, order), never pixel snapshots, because font rendering differs across machines. The seeded library (`webtest.Seed`) contains Latin and Arabic artists, an album track, a single, an unidentified track and two playlists, one with a long name. The playlist-picker layout regression (rows on one line, columns aligned, no overlap, counts right of names, long names wrap) is ported to Playwright at both widths. Browser configs use `$MM_CHROMIUM` as the executable when set, which lets a machine with a Chromium of a different revision run them.

### 6.6 Nix and tooling

- **pnpm:** `web/package.json` has `"packageManager": "pnpm@<exact 10.x>"` and `"pnpm": {"onlyBuiltDependencies": ["esbuild"]}`; `pnpm-lock.yaml` is committed; `package-lock.json` never is. From the repository root the commands are `pnpm --dir web run <script>` and `pnpm --dir web exec <bin>`. Outside Nix, `corepack enable` provides the pinned version.
- **Scripts:** `dev`, `build` (into `../internal/web/dist`), `build:nix` (into `dist`), `check` (`tsc --noEmit`, ESLint, unit tests), `test:unit`, `test:browser`, `test:e2e` (builds, then Playwright), `test`, `gen:types`.
- **`nix/frontend.nix`:** `stdenvNoCC.mkDerivation` with `fetchPnpmDeps { pnpm = pnpm_10; fetcherVersion = 2; hash }` (fall back to `pnpm.fetchDeps` and `pnpm.configHook` if the locked nixpkgs lacks the new names), `pnpmConfigHook`, `nodejs`, `pnpm_10`; `PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1`; the pnpm version switch disabled; `checkPhase` runs `pnpm run check`; installs `dist`. A hash change is handled with `lib.fakeHash`, one build, paste.
- **`nix/package.nix`:** copies the frontend into `internal/web/dist` in `postPatch`; `lib.fileset` additionally includes `web/src/api/types.gen.ts` so the drift test runs in the sandbox; completions are removed; `wrapProgram` keeps `yt-dlp`, `ffmpeg` and `xdg-utils`.
- **Dev shell:** `go`, `nodejs`, `pnpm_10`, `yt-dlp`, `ffmpeg`, `playwright-driver.browsers`; `PLAYWRIGHT_BROWSERS_PATH` set; `PLAYWRIGHT_SKIP_VALIDATE_HOST_REQUIREMENTS=true`. `@playwright/test` is pinned to exactly the version of nixpkgs' `playwright-driver`, and `packageManager` to the version of `pnpm_10` the shell provides. The shell is extended in Task 10 (not at the end) so that every later task runs inside `nix develop`.
- **Home-manager module** (`homeManagerModules.default`): `programs.music-manager.{enable, package, library, devices}`; `devices` is an attribute set of submodules mirroring the profile keys, every key `null` by default and dropped when `null` so the Go defaults apply; it writes `xdg.configFile."mm/devices.toml"` only when `devices` is non-empty. Checked by generating a file from a sample config and running `mm --config FILE --check-config` (accepts a valid profile, rejects `coverFormat = "png"`).

### 6.7 Sequencing and compatibility

Six phases, each leaving the repository working: (1) remove the CLI, (2) profiles (additive API), (3) typed contract, (4) front-end foundation, (5) port the interface, (6) cutover, Nix, module, docs. The old page keeps working until the cutover because the legacy API fields stay until then; the new front end is served through `web.WithFrontend`, which overrides the old embedded page until the default flips.

## 7. Error handling and edge cases

| Situation | Behaviour |
|---|---|
| Invalid, misspelled, duplicate or reserved-name `devices.toml`; missing explicit `--config` | Refuse to start; message names file, profile and key. |
| Binary built without the front end | `/` answers 503 with a page saying `pnpm --dir web run build`; the API still works. |
| Unknown profile name in a request | 400 with a message. |
| Empty library; no playlists; unidentified track; empty playlist | Empty states render; `p` with no playlists opens on the name input. |
| Arabic or mixed-direction names | Own direction, neighbours not reordered, count column does not move, sorted under the Latin form. |
| Typing in an input; IME composition | Shortcuts do not fire. |
| Library changed elsewhere | SSE `library` event reloads; reconnect reloads. |
| Cross-origin or rebinding request | Rejected by the unchanged guard; the dev proxy must keep the Host (`changeOrigin: false`). |
| Tests or tools run against the real library | Forbidden: tests use temporary libraries and every manual run passes `--library /tmp/...`. |

## 8. Risks and open questions

- **Execution environment.** The implementing agent runs on the maintainer's local NixOS machine with full network access, and every command runs inside `nix develop`. All Nix steps are therefore verified, and a Nix failure is a failure, not an "unverified" note. (A restricted cloud sandbox would not do: GitHub tarballs outside its scope return 403, so the flake's `github:NixOS/nixpkgs` input cannot be fetched there.)
- **Playwright version pin.** `@playwright/test` must equal nixpkgs' `playwright-driver` version; the dev-shell browser tests fail otherwise.
- **Lost coverage.** About 58 CLI tests pin behaviour that must survive as app- or web-level tests before the CLI is deleted (plan Task 1).
- **pnpm versions.** The `packageManager` pin and nixpkgs' `pnpm_10` must be the same major; a mismatch of minor versions is tolerated, a mismatch of lockfile format is not.
- **Hashes.** `vendorHash` (Go) and the pnpm `hash` change whenever dependencies change and need the fake-hash round trip.
- **Open, deliberately:** upgrading to pnpm 11; screenshot tests; running browser tests inside `nix flake check`.

## 9. Acceptance criteria

1. `mm` has no subcommands; `--version` prints `mm <version>`; unknown flags exit 2.
2. `go vet ./...` and `go test ./...` pass with no browser, no network and no built front end.
3. `sync-generic.golden` and `sync-wolfgang.golden` are unchanged; the recorded old-target outputs match the built-in profiles.
4. An invalid `devices.toml` prevents start and names file, profile and key; `--check-config` reports it.
5. `GET /api/profiles` lists `generic`, `wolfgang-wmp01` and user profiles; a sync plan with a user profile uses its rules; the library folder ignores profiles.
6. The generated-types drift test passes; no hand-written server data types exist in `web/`.
7. Every row of the README's key table has a passing test (the parity checklist is complete); the playlist-picker layout test passes at 1200 and 500 px; Arabic and empty-state tests pass.
8. ESLint enforces no `any` and no comments in new code; `pnpm --dir web run check` and `pnpm --dir web run test` pass.
9. `pnpm-lock.yaml` is committed, `package-lock.json` does not exist, `packageManager` is pinned.
10. The old page, its tests and every legacy Wolfgang field are gone.
11. `nix flake check` passes (`frontend`, `build`, `hm-module`), `nix build` produces a working `mm`, and `nix develop -c pnpm --dir web run test` passes.
12. The README describes the web-only program, profiles (with the field table), the module and the new development workflow, and mentions no removed command.

## 10. Plan map

Phase 1 Tasks 1–3 · Phase 2 Tasks 4–7 · Phase 3 Tasks 8–9 · Phase 4 Tasks 10–13 · Phase 5 Tasks 14–21 · Phase 6 Tasks 22–25.
