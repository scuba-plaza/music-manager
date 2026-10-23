# mm: a music library manager for an MP3 player

`mm` keeps one record per track in `library.json` and **renders** the music
folder from those records. You edit records, never files; `mm sync` makes the
folder match.

```
Artist/Album/03 - Title.mp3     identified tracks (ASCII names, fresh ID3v2.4 tags, cover.jpg per album)
Artist/Title.mp3                identified singles (with Title.jpg, their cover, beside them)
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
tracks on the left, and on the right whatever the cursor is on: a track's details, or what is inside an album (its tracks), an artist (its albums and singles), a playlist (its tracks in play order) or the Needs work group (what each track lacks), as the preview column of a file manager does. Every change is saved at once to
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
| `→` `←` `1` `2` `3` | in the fields: the other tab (Details, Audio, Player); `←` on the first tab goes back to the tree. The tab you are on stays when you select another track |
| `tab` | while editing a name: switch between the text and its ASCII form |
| `[` `]` `{` `}` `0` `p` | on a cut: nudge by 0.1 s / 1 s, clear, hear the 8 s around it (needs `ffplay` or `mpv`) |
| `space` `ctrl+a` `e` | mark tracks (an album marks all of its tracks); edit the marked ones, or rename an artist or album |
| `J` `K` `n` | move a track down or up in its album; number the album 1…N |
| `p` | add the marked tracks (or the selected track, album or artist) to a playlist, or start a new one. The **Playlists** group at the top of the tree opens into each playlist and its tracks in play order; there `J` `K` reorder, `d` takes a track out of the playlist (never out of the library), `d` on the playlist itself deletes it after asking, and `e` renames it |
| `i` `I` `o` | find a track on MusicBrainz and pick the match; do all unidentified tracks; use the file's old tags |
| `a` | add from a YouTube link: one form, pre-filled from the match |
| `s` | sync: choose the folder (or a mounted player), see what will change, apply. The header says `N to sync · press s` when something is waiting; clicking it works too |
| `d` `u` `ctrl+r` `D` | remove (asks first: `y` or `enter` agrees), undo, redo, check the library for damage |

Covers are shown in full resolution in terminals that speak the kitty graphics protocol (kitty, Ghostty), and as coloured half-blocks everywhere else. `MM_COVERS=blocks` or `MM_COVERS=kitty` overrides the guess; inside tmux or screen, which do not pass pictures on, blocks are used. The cover takes the room the fields leave below them, so in a short window (a large font does that) the dim ASCII forms under the names are left out to give it room; they are back when the window is taller, and the field you are editing always shows its ASCII form.

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
| `mm edit <id> --title … --artist … --album … --track N --of N --released DATE --genre TEXT --player-artist TEXT --player-album TEXT --source-url LINK --trim-start T --trim-end T --gain DB --fade-in T --fade-out T --cover FILE` | change a track; `--title-ascii` etc. override the derived ASCII |
| `mm ls [words] [status:identified\|unidentified] [--sort KEY] [--reverse]` / `mm show <id>` | look at records; `--sort` takes `artist`, `album`, `title`, `track`, `length`, `released`, `added` or `edited` |
| `mm sync [--target DIR] [--dry-run] [--prune --yes]` | render the folder; `--prune` deletes files the records don't account for |
| `mm rm <id>…` | remove records (audio is deleted by the next sync) |
| `mm cover normalize [--dry-run]` | make every stored cover a JPEG the player opens (see Covers below) |
| `mm review apply <file> [--dry-run] [--include-low]` | set a field for many tracks from a file of proposals you have corrected (see Review files below) |
| `mm playlist <new, add, rm, move, rename, delete, import, ls, show>` | make and change playlists (see Playlists below) |
| `mm doctor` | check the stores for missing or damaged files |
| `mm tui` | the full-screen interface (also what plain `mm` opens in a terminal) |

## Covers

The player's photo viewer opens baseline JPEG files of modest size. It fails on PNG and progressive files, on 4:4:4 colour and on very large pictures, and does not list PNG files at all. So every cover is stored as a baseline JPEG with a JFIF header, 4:2:0 colour, at most 480 pixels along its longer side (a transparent background becomes white), whatever file it was given; a cover that already is such a file is kept as it is. `mm sync` writes it as `cover.jpg` in an album folder and as `Title.jpg` beside a single. Covers stored by an earlier version are converted when they are synced, and `mm cover normalize` converts them in the library itself. The `cover.png` files they leave on the player are strays that `mm sync --prune` removes.

Reusing a picture costs nothing in the library: covers are kept once each, under `.mm/covers/<sha256>.img`, however many tracks, albums or artists use them, and the same image file always becomes the same stored cover. (The same picture saved in another format or size is a different file and counts as a different cover.) On the player each album folder and each single still gets its own `cover.jpg` or `Title.jpg`, and every MP3 embeds its cover, on purpose: the card cannot link one file into several folders, the photo viewer is browsed by folder, and a shared folder of hash-named files would tell you nothing about which album a picture belongs to (the player cuts names at 26 characters, so a full SHA-256 would not even fit). The price is small: in a library of 177 tracks the repeated pictures come to about 6 MB on a 2.6 GB card.

## Genre

A track may have a genre, free text such as `Folk`, `Anthem` or `Neoclassical`. `mm sync` writes it to the file as the genre tag, which the player shows. Its index has room for 12 characters of a genre, as for an artist and an album (judging from the layout of `MUSIC.LIB`, not yet seen on the device), so short names such as `Tarab` or `Arabic Pop` are safer than long ones. Set it with `mm edit <id> --genre Folk`, in the TUI (the Genre field and the bulk edit form, `e`) or `mm add --genre`; find tracks with `mm ls genre:folk` (any part of the genre, case ignored). The tags of an ingested file are kept as hints, the genre among them, and `mm adopt` takes them.

## Review files

When a field has to be filled in for many tracks at once, a review file lists a proposal for each artist, album or track, with how sure the proposal is, so that you can read and correct it before anything changes:

```
## ARTISTS -> genre

Fairouz  (10 tracks)  [high ; the old tags]
    Arabic

## TRACKS -> genre

k7f2qd  Fairouz · Songs (Live) · Wtni  [low ; a guess]
    Anthem
```

The sections are `## ARTISTS`, `## ALBUMS` and `## TRACKS`, each followed by `-> field` (`genre`, `player-artist` or `player-album`). An entry is a heading and, indented under it, the value; the heading names the artist or album as the library does, or starts with the track's ID, and may end with `[high|medium|low ; note]` (an entry without one counts as high). A value of `-` or nothing skips the entry. A track's value wins over its album's, which wins over its artist's. Lines starting with `#` are comments.

`mm review apply <file>` checks the whole file first, so that one invalid value changes nothing, then sets all the values in one save. Entries marked `low` are left out unless you pass `--include-low`, `--dry-run` only reports, and entries that match no track are listed. Applying a file twice changes nothing the second time.
## Names on the player

The player keeps only 12 characters of an artist and of an album, and 26 of a file name (the `.mp3` included), and cuts the rest wherever that falls: "Abdel Halim Hafez" showed up as "Abdel Halim H". So `mm sync` shortens the names itself, in the file's tags and in the file names, at a whole word where it can: parentheses and a leading "The" go first, then the name is cut after the last word that leaves at least half of it, and never ends on a small word such as "of" or "and". The library keeps the full names. A single has no album, which the player shows as "Unknown Album", so its title is written there instead.

To choose a name yourself, set `--player-artist` or `--player-album` (`mm edit <id> --player-artist "A. H. Hafez"`; `""` goes back to the automatic one), or use the Player tab of the TUI (`3`), which shows what the player will show, where it comes from and how many characters it counts. `mm show` has an "On the player" row when the names differ. For many tracks at once use a review file with `## ARTISTS -> player-artist` and `## ALBUMS -> player-album`, each entry with its own shortening.

File names are `NN - Title.mp3` within 22 characters before the extension (a title is shortened the same way), so a track's file name is also whole on the player. This renames the files of long titles at the next sync; folders keep their full names.

## Volume and fades

A track can be made louder or quieter and can fade in and out, without being decoded or encoded again: an MP3 stores a gain for every 13 ms granule of each channel, and `mm sync` changes it (one step is 1.5 dB, a fourth of an octave of amplitude), so the sound is not touched and the file does not grow. A volume is therefore rounded to the nearest 1.5 dB and limited to 24 dB either way; boosting a track whose peaks are near full scale can clip. A fade is a ramp of amplitude from silence to full or back, in steps of 1.5 dB down to 60 dB (so a very short fade may sound stepped), and the track is silent before a fade-in starts and after a fade-out ends.

```
mm edit <id> --gain 3 --fade-in 2 --fade-out 4      # +3 dB, 2 s in, 4 s out
mm edit <id> --fade-out 3:50-3:58                    # a stretch of the original
mm edit <id> --gain 0 --fade-in "" --fade-out ""     # all away
```

A length counts from where the track now starts (fade-in) or back from where it now ends (fade-out); a stretch is in the time of the original, as it is stored, so cutting the track again later does not move a fade. In the TUI the Audio tab (`2`) has Volume, Fade in and Fade out below the cuts, with the same `[ ]` and `{ }` nudge keys (1.5 dB and 6 dB for the volume, 0.1 s and 1 s for the length of a fade), `0` to clear, and `p` to hear the start or the end with the change applied. A line under the fields draws the ramps.

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

## Playlists

A playlist is a named list of tracks in a chosen order; a track can be in as many playlists as you like, but only once in each.

```sh
mm playlist new Study 3k2fq7 9xw4ap     # make one, with these tracks (ids from mm ls)
mm playlist add Study m2d8cz            # put tracks at the end
mm playlist move Study m2d8cz 1         # move a track to position 1
mm playlist rm Study 9xw4ap             # take a track out (it stays in the library)
mm playlist show Study
mm playlist rename Study Exams
mm playlist delete Exams
mm playlist import USERPL1.PL Resistance --dry-run   # bring in a playlist made on the player itself
```

`mm sync` writes each playlist as `Playlists/<name>.m3u`, with the player-friendly form found by experiment on a Wolfang WMP01: paths from the root of the card with backslashes (`\Artist\Album\01 - Title.mp3`), one per line, plain UTF-8, Windows line endings. The player lists every `.m3u` on the card as a playlist; it ignores paths written with slashes or relative to the playlist. Removing a track from the library takes it out of its playlists. Playlist files count in the `N to sync` of the interface, and a deleted playlist's file is a stray that `mm sync --prune` removes.

The playlists made on the player (`USERPL1.PL` to `USERPL3.PL` on the card) cannot be written by `mm`, but they can be read: `mm playlist import` takes the file names in one and finds the tracks they were by title, ignoring the track number, case and punctuation. It lists what it found (`~` matched on the start of a title only, `?` fits several tracks, `-` not found), and with `--dry-run` makes nothing. Fix the rest with `mm playlist add` and `mm playlist move`.

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
