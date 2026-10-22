# mm: a music library manager for an MP3 player

`mm` keeps one record per track in `library.json` and **renders** the music
folder from those records. You edit records, never files; `mm sync` makes the
folder match.

```
Artist/Album/03 - Title.mp3     identified tracks (ASCII names, fresh ID3v2.4 tags, cover.jpg per album)
Artist/Title.mp3                identified singles
_Inbox/<id> - old name.mp3      tracks not identified yet
.mm/                            stored audio and covers (content-addressed), library.json lives beside it
```

Every track keeps both forms of its names: `title`/`artist`/`album` are ASCII
(Arabic is romanised, accents are stripped) and `original_*` hold the text as
it was. Files carry the ASCII form in the normal tags and the original in
`ORIGINAL_TITLE` / `ORIGINAL_ARTIST` / `ORIGINAL_ALBUM` tags.

## Everyday use

```sh
mm add https://youtu.be/…          # download, look up on MusicBrainz, ask for each detail
mm add URL --no-input --artist X --title Y --album Z --track 3 --of 12 --trim-start 0:12
mm sync                            # write it into the library folder
mm sync --target /run/media/me/PLAYER
```

`mm add` asks for title, artist, album, track number, tracks on the album, the
release date and how much to cut off the start and end, with defaults filled in. Anything given
as a flag is not asked. Without a terminal (or with `--no-input`) it never
asks and fails if the title or artist is unknown.

## The full-screen interface

Run `mm` in a terminal (or `mm tui`) to browse and edit the library on one screen: artists, albums and
tracks on the left, the selected track's details on the right. Every change is saved at once to
`library.json`; the music folder only changes when you sync (`s`), and the header says how many tracks
are waiting (`3 to sync`). Press `?` for the list of keys; the bottom line always shows the ones that
matter right now. Without a terminal, plain `mm` prints help as before.

| | |
|---|---|
| `↑↓` `→←` `/` | move, open and close, filter (same words as `mm ls`, e.g. `status:unidentified`) |
| `z` | centre the cursor in the list, like `zz` in vim |
| `S` | sort the list by artist, album, title, track number, length or any of the three dates (see below); the menu also reverses the order and goes back to the tree |
| `+` `-` | open the tree one more level (artists, then albums) / close all of it |
| `→` `enter` | from a track into its fields; `enter` edits a field, `enter` saves, `esc` cancels |
| `tab` | while editing a name: switch between the text and its ASCII form |
| `[` `]` `{` `}` `0` `p` | on a cut: nudge by 0.1 s / 1 s, clear, hear the 8 s around it (needs `ffplay` or `mpv`) |
| `space` `ctrl+a` `e` | mark tracks (an album marks all of its tracks); edit the marked ones, or rename an artist or album |
| `J` `K` `n` | move a track down or up in its album; number the album 1…N |
| `i` `I` `o` | find a track on MusicBrainz and pick the match; do all unidentified tracks; use the file's old tags |
| `a` | add from a YouTube link: one form, pre-filled from the match |
| `s` | sync: choose the folder (or a mounted player), see what will change, apply. The header says `N to sync · press s` when something is waiting; clicking it works too |
| `d` `u` `ctrl+r` `D` | remove (asks first: `y` or `enter` agrees), undo, redo, check the library for damage |

Covers are shown in full resolution in terminals that speak the kitty graphics protocol (kitty, Ghostty), and as coloured half-blocks everywhere else. `MM_COVERS=blocks` or `MM_COVERS=kitty` overrides the guess; inside tmux or screen, which do not pass pictures on, blocks are used.

The mouse works too: click to select, click again to open, double-click a folder row to open it, wheel
to scroll. Colours follow your terminal's light or dark background and honour `NO_COLOR`. If another `mm`
changes the library while the interface is open, the change appears within a couple of seconds.

## Commands

| | |
|---|---|
| `mm ingest <file\|dir>…` | add existing MP3s as unidentified tracks (the files are only read) |
| `mm identify [id…] [--yes]` | look tracks up on MusicBrainz; pick from a list, or `--yes` for confident matches |
| `mm adopt [id…]` | identify tracks by what their old tags said, for what MusicBrainz does not know |
| `mm add <url>` | download with yt-dlp and add |
| `mm edit <id> --title … --artist … --album … --track N --of N --released DATE --trim-start T --trim-end T --cover FILE` | change a track; `--title-ascii` etc. override the derived ASCII |
| `mm ls [words] [status:identified\|unidentified] [--sort KEY] [--reverse]` / `mm show <id>` | look at records; `--sort` takes `artist`, `album`, `title`, `track`, `length`, `released`, `added` or `edited` |
| `mm sync [--target DIR] [--dry-run] [--prune --yes]` | render the folder; `--prune` deletes files the records don't account for |
| `mm rm <id>…` | remove records (audio is deleted by the next sync) |
| `mm doctor` | check the stores for missing or damaged files |
| `mm tui` | the full-screen interface (also what plain `mm` opens in a terminal) |

## Dates and sorting

Every track has three dates:

- **released**: when the track first came out, as precisely as is known: `1999`, `1999-05` or `1999-05-17`. You set it (`mm edit --released`, the `Released` field in the interface, or in the bulk-edit form for many tracks at once). It is also filled in from a MusicBrainz match, from yt-dlp's `release_date`, and from the old `TDRC` or `TYER` tag when a file is adopted; and it is written into the synced file as its `TDRC` tag, so players show the year.
- **added**: when the track entered the library. It never changes.
- **edited**: when its record was last changed.

The last two are kept by `mm` and shown in `mm show` and under the track in the interface. Any of the three, and
artist, album, title, track number and length, can order the list: `mm ls --sort added`, or `S` in the interface, which shows
all tracks in one flat list in that order (with the date beside each) until you choose the tree again. Text and numbers sort
lowest first and dates newest first; `--reverse` (or `r` in the menu) turns that around. Tracks that have no value, such as
no release date, always come last, and ties are ordered by artist, album, track number and title.

Trims are stored on the record and applied when rendering, on MP3 frame
boundaries (about 26 ms), without re-encoding. The stored audio is never
changed, so a trim can always be redone.

## Moving an existing folder over

The library folder is also where the old music lives, so ingest first, check
the result with `--dry-run`, and only then prune:

```sh
cp -r ~/Music/mp3-player ~/mp3-player.backup
mm ingest ~/Music/mp3-player
mm identify --yes          # confident MusicBrainz matches; `mm identify` lets you pick for the rest
mm adopt                   # whatever is left: take the old tags as they are
mm doctor
mm sync --dry-run --prune
mm sync --prune --yes
```

## Settings

* `--library DIR` or `MM_LIBRARY`: the library folder (default `~/Music/mp3-player`).
* `MM_CONTACT`: an email or URL added to the User-Agent sent to MusicBrainz, which they ask for.
* `ACCESSIBLE=1` or `TERM=dumb`: plain line-based questions instead of the full-screen form.

Needs `yt-dlp` (and `ffmpeg`, which yt-dlp uses) on `PATH` for `mm add`. Hearing a cut in the interface needs `ffplay` (part of ffmpeg) or `mpv`.

## Installing with Nix

The repository is a flake. It provides `packages.<system>.music-manager` (the
program is `mm`) and an overlay that adds `pkgs.music-manager`. The Nix package
puts `yt-dlp` and `ffmpeg` at the end of the program's `PATH`, so ones you
already have take precedence.

```sh
nix run github:scuba-plaza/music-manager -- --help        # try it
nix profile install github:scuba-plaza/music-manager      # or install it
```

With home-manager, add the flake as an input of your system flake, apply its
overlay, and list the package:

```nix
inputs.music-manager = {
  url = "github:scuba-plaza/music-manager";   # or "git+file:///path/to/music-manager"
  inputs.nixpkgs.follows = "nixpkgs";
};
# in the overlays list:   music-manager.overlays.default
# in home.packages:       music-manager
```

When the Go dependencies change, set `vendorHash = lib.fakeHash;` in
`nix/package.nix`, build once, and paste in the hash Nix prints. Nix only sees
files git knows about, so `git add` new files before `nix build`.

## Development

```sh
go test ./...
go test -tags pty ./internal/tui                          # runs the real binary in a pseudo-terminal
go test ./internal/tui -update                               # accept changed screens in internal/tui/testdata
nix build                                                    # builds the package and runs the tests in the sandbox
go test -tags live ./internal/musicbrainz ./internal/ytdlp   # real MusicBrainz, real yt-dlp
```
