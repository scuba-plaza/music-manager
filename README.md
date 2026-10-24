# mm: a music library manager for an MP3 player

`mm` keeps one record per track in `library.json` and **renders** the music
folder from those records. You edit records, never files; `mm sync` makes the
folder match.

```
Artist/Album/03 - Title.mp3     identified tracks (names in their own script, fresh ID3v2.4 tags, a cover file per album)
Artist/Title.mp3                identified singles (their cover is in the tag)
Playlists/<name>.m3u8           playlists
_Inbox/<id> - old name.mp3      tracks not identified yet
.mm/                            stored audio and covers (content-addressed), library.json lives beside it
```

A record holds the truth and nothing for any one device: `title`, `artist` and
`album` as they really are, in any script, one artist per track, and the cover
exactly as it was given, and optionally how a name reads in Latin letters (see Latin forms). A plain `mm sync` writes that truth in standard forms
(true names in the tags and the file names, original covers, M3U8 playlists),
which suits a phone, a computer or any player without odd limits. A player that
does have limits gets what it needs worked out from the record while syncing:
`mm sync --wolfgang --target …` does it for the Wolfgang WMP01 (see below), and
nothing of it is stored.

## Everyday use

```sh
mm add https://youtu.be/…          # download, look up on MusicBrainz, ask for each detail
mm add URL --no-input --artist X --title Y --album Z --track 3 --of 12 --trim-start 0:12
mm sync                            # write it into the library folder
mm sync --target /run/media/me/PLAYER
mm sync --wolfgang --target /run/media/me/PLAYER   # the same for the Wolfgang WMP01
```

`mm add` asks for title, artist, album, track number, tracks on the album, the
release date and how much to cut off the start and end, with defaults filled in. Anything given
as a flag is not asked. Without a terminal (or with `--no-input`) it never
asks and fails if the title or artist is unknown.

## The full-screen interface

Run `mm` in a terminal (or `mm tui`) to browse and edit the library on one screen: artists, albums and
tracks on the left, and on the right whatever the cursor is on: a track's details, or what is inside an album (its tracks), an artist (its albums and singles), a playlist (its tracks in play order) or the Needs work group (what each track lacks), as the preview column of a file manager does. Every change is saved at once to
`library.json`; the music folder only changes when you sync (`s`), and the header says how many tracks
are waiting (`3 to sync`). Press `?` for the list of keys, one page for each kind (`←` `→` or `1`-`4` turn the pages); the bottom line always shows the ones that
matter right now. Without a terminal, plain `mm` prints help as before.

| | |
|---|---|
| `↑↓` `/` | move, filter (same words as `mm ls`, e.g. `status:unidentified`) |
| `z` | centre the cursor in the list, like `zz` in vim |
| `S` | sort the list by artist, album, title, track number, length or any of the three dates (see below); the menu also reverses the order and goes back to the tree |
| `+` `-` | open the tree one more level (artists, then albums) / close all of it |
| `enter` `←` | open and close the artist, album or playlist under the cursor (`←` closes it, or goes up to its parent) |
| `→` `e` | into the fields in the right-hand pane of what the cursor is on: a track, an artist, an album or a playlist; `↑↓` choose a field, `enter` edits it, `enter` saves, `esc` cancels, `←` goes back to the tree (see below for artists, albums and playlists). On a track `enter` goes in too |
| `→` `←` `1` `2` `3` | in the fields: the other tab (Details, Audio, Latin: the forms of the names in Latin letters, see below); `←` on the first tab goes back to the tree. The tab you are on stays when you select another track. Under `mm --wolfgang` there is a fourth, `4`: a preview of what a sync for the Wolfgang writes for the track (see below) |
| `[` `]` `{` `}` `0` `p` | on a cut: nudge by 0.1 s / 1 s, clear, hear the 8 s around it (needs `ffplay` or `mpv`) |
| `space` `ctrl+a` `e` | mark tracks (an album marks all of its tracks); `e` edits the marked ones in one form, which has the Latin forms of the artist, album and genre too (with nothing marked, `e` is `→`) |
| `J` `K` `n` | move a track down or up in its album; number the album 1…N |
| `p` | add the marked tracks (or the selected track, album or artist) to a playlist, or start a new one. The **Playlists** group at the top of the tree opens into each playlist and its tracks in play order; there `J` `K` reorder, `d` takes a track out of the playlist (never out of the library), `d` on the playlist itself deletes it after asking, and `→` edits its name and its Latin name |
| `i` `I` `o` | find a track on MusicBrainz and pick the match; do all unidentified tracks; use the file's old tags |
| `a` | add from a YouTube link: one form, pre-filled from the match |
| `s` | sync: choose the folder (or a mounted player), see what will change, apply. A drive has a Format row, Generic or Wolfgang WMP01, switched with `w` and never remembered (under `mm --wolfgang` a drive starts on the Wolfgang); the library folder is always written the standard way. The header says `N to sync · press s` when something is waiting; clicking it works too |
| `d` `u` `ctrl+r` `D` | remove (asks first: `y` or `enter` agrees), undo, redo, check the library for damage |

Covers are shown in full resolution in terminals that speak the kitty graphics protocol (kitty, Ghostty), and as coloured half-blocks everywhere else. `MM_COVERS=blocks` or `MM_COVERS=kitty` overrides the guess; inside tmux or screen, which do not pass pictures on, blocks are used. The cover takes the room the fields leave below them.

The mouse works too: click to select, click the selected track, artist, album or playlist again to go into its fields (a click on a field chooses it, another edits it), double-click a folder row to open or close it, wheel
to scroll. Colours follow your terminal's light or dark background and honour `NO_COLOR`. If another `mm`
changes the library while the interface is open, the change appears within a couple of seconds.

## Commands

| | |
|---|---|
| `mm ingest <file\|dir>…` | add existing MP3s as unidentified tracks (the files are only read) |
| `mm identify [id…] [--yes]` | look tracks up on MusicBrainz; pick from a list, or `--yes` for confident matches |
| `mm adopt [id…]` | identify tracks by what their old tags said, for what MusicBrainz does not know |
| `mm add <url>` | download with yt-dlp and add |
| `mm edit <id> --title … --artist … --album … --track N --of N --released DATE --genre TEXT --title-latin … --artist-latin … --album-latin … --genre-latin … --source-url LINK --trim-start T --trim-end T --gain DB --fade-in T --fade-out T --cover FILE` | change a track; title, artist and album are the true names, in any script; the `-latin` ones are the forms in Latin letters, `""` takes one away (see Latin forms) |
| `mm ls [words] [status:identified\|unidentified] [--sort KEY] [--reverse]` / `mm show <id> [--wolfgang]` | look at records; `--sort` takes `artist`, `album`, `title`, `track`, `length`, `released`, `added` or `edited` |
| `mm sync [--target DIR] [--wolfgang] [--dry-run] [--prune --yes]` | render the folder; `--prune` deletes files the records don't account for; `--wolfgang` (needs `--target`) writes for the Wolfgang WMP01 |
| `mm rm <id>…` | remove records (audio is deleted by the next sync) |
| `mm review apply <file> [--dry-run] [--include-low]` | set a field for many tracks from a file of proposals you have corrected (see Review files below) |
| `mm playlist <new, add, rm, move, rename, latin, delete, import, ls, show>` | make and change playlists (see Playlists below) |
| `mm migrate [--dry-run]` | bring a library of version 1 up to date (see Upgrading below) |
| `mm restore-latin [--dry-run]` | put back the Latin forms that an earlier migration dropped (see Upgrading below) |
| `mm doctor` | check the stores for missing or damaged files, and point out artist names that look like several artists |
| `mm tui [--wolfgang]` | the full-screen interface (also what plain `mm` opens in a terminal) |

## Covers

A cover is stored as it was given: a JPEG or a PNG, byte for byte (a file that is not a readable image is refused). `mm sync` writes it as it is: embedded in the MP3 with its own type, and as `cover.jpg` or `cover.png` in an album folder; a single has none beside it, its cover is in its tag. (The Wolfgang is different, see below.)

Reusing a picture costs nothing in the library: covers are kept once each, under `.mm/covers/<sha256>.img`, however many tracks, albums or artists use them, and the same image file always becomes the same stored cover. (The same picture saved in another format or size is a different file and counts as a different cover.) In the synced folder each album folder gets its own cover file and every MP3 embeds its cover, on purpose: players show the tag, file managers show the file, and a card cannot link one file into several folders. The price is small: in a library of 177 tracks the repeated pictures come to about 6 MB.

## Genre

A track may have a genre, free text such as `Folk`, `Anthem` or `Neoclassical`. `mm sync` writes it to the file as the genre tag. Set it with `mm edit <id> --genre Folk`, in the TUI (the Genre field and the bulk edit form, `e`) or `mm add --genre`; find tracks with `mm ls genre:folk` (any part of the genre, case ignored). The tags of an ingested file are kept as hints, the genre among them, and `mm adopt` takes them. (The Wolfgang shows 12 characters of a genre; `--wolfgang` warns about a longer one.)

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

The sections are `## ARTISTS`, `## ALBUMS` and `## TRACKS`, each followed by `-> field`; `genre` is the field a review file sets. An entry is a heading and, indented under it, the value; the heading names the artist or album as the library does, or starts with the track's ID, and may end with `[high|medium|low ; note]` (an entry without one counts as high). A value of `-` or nothing skips the entry. A track's value wins over its album's, which wins over its artist's. Lines starting with `#` are comments.

`mm review apply <file>` checks the whole file first, so that one invalid value changes nothing, then sets all the values in one save. Entries marked `low` are left out unless you pass `--include-low`, `--dry-run` only reports, and entries that match no track are listed. Applying a file twice changes nothing the second time.

## Editing an artist, an album or a playlist

To rename an artist or fix an album you do not go from track to track: press `→` (or `e`) on its row and the right-hand pane shows its fields, which are edited like a track's. `enter` opens the tree instead.

| | fields |
|---|---|
| artist | Name, Latin name, Genre, Latin genre |
| album | Name, Latin name, Released, Genre, Latin genre, Tracks on album, Cover |
| playlist | Name, Latin name |

* **An edit reaches all the tracks of the artist or album**, whatever the filter shows, so that an artist is never split in two; an album means that artist's tracks of that name only. The pane says `all 3 tracks · one undo step`: the whole edit is one step for `u`, and `ctrl+r` redoes it. The *Needs work* group, *Singles* and the *Playlists* group have no fields.
* **What a field shows** is what all the tracks agree on. If they differ it says `(mixed)` and opens empty: leaving it empty keeps every track as it is, `-` clears the field on all of them, and any other text is set on all of them. A field that all the tracks agree on is cleared by emptying it. A Latin form is dim while it is worked out and normal once it was set by hand; it is only written on the tracks that have a name to write.
* **Names cannot be empty**, and a name that is already another artist's (or the same artist's album) joins the two: the pane says `joins the artist Phoria (4 tracks)` while you type. To turn tracks into singles, mark them and use `e` with `-` in Album. The cursor and the open albums follow the new name.
* **The cover of an album** is a JPEG or PNG file, set on every track of the album; an empty path or `x` takes the covers away. A playlist's name must not be another playlist's or give the same file name; the pane says so and nothing changes.

## Latin forms

Names are kept as they really are, in any script. Some places can only show Latin letters: the Wolfgang WMP01 (below) and the order of the list, which puts فيروز among the F. For these, each name has a **Latin form**: how it reads in Latin letters. Unless you set one, it is worked out from the name, word by word. That is rough for Arabic, which is written without its short vowels: فيروز comes out as "Firuz" and جوليا بطرس as "Julya Btrs". So a form can be set by hand, for the title, the artist, the album and the genre of a track and for the name of a playlist, and it is used instead of the worked-out one:

* **In the full-screen interface**, the **Latin** tab (`3`) of a track lists the four. The form shown is the one in use: dim when it is worked out, normal when it was set by hand, `—` when there is no name to write. `enter` edits it starting from what is shown; saving what was there changes nothing, and saving it empty goes back to the worked-out form. The change is one step for `u` to undo, like any other. An artist and an album have a Latin name and a Latin genre among their fields, a playlist a Latin name, and these are set on all their tracks at once (see above). For marked tracks, `e` has rows for the Latin form of the artist, the album and the genre (`-` takes it away).
* **On the command line**, `mm edit <id> --title-latin "Watani" --artist-latin "Fairouz" --album-latin … --genre-latin …`, with `""` to take one away, and `mm playlist latin <name> [latin name]`. `mm show <id>` lists the forms that were set.

A form belongs to the name it was written for, and changing the name does not touch it, so a corrected spelling keeps its form; a name that becomes empty loses its form. Forms are searched (`mm ls`, `/` in the interface finds "watani" as well as وطني) and put the track in order (`S` and `--sort`). The library folder and a plain sync never use them: the tags and the file names there are the true names, and setting a form leaves nothing to sync. `mm sync --wolfgang` uses them for the folders, the file names, the artist, album and genre tags and the name of the playlist file; the title tag stays the true title.

## The Wolfgang WMP01

`mm sync --wolfgang --target DIR` writes for the Wolfgang WMP01 player. Pass the flag every time (nothing remembers it); it needs `--target`, so that the library folder is always the standard tree. Everything below is worked out from the records while syncing and stored nowhere: the library holds no name, cover or setting for the player. The limits are the ones measured on the device.

* **Names.** The player shows 24 characters of a file name (it keeps 26), the `.mp3` included, and scrolls a longer name along when it is hovered; it shows 12 of an artist, an album and a genre, cuts the rest wherever that falls, and does not join Arabic letters. So folders and files are named in Latin letters, using the Latin forms (see above): the ones set by hand, else the ones worked out, which lose the short vowels of Arabic (فيروز becomes "Firuz"); accents are stripped. The title tag keeps the true title, since the player shows file names and not titles.
* **File names** are `NN - Title.mp3` for the MP3 files only, hard-cut at 20 characters before the extension, so that the whole name, `.mp3` included, fits in the 24 the player shows and does not scroll; the track number and the ` (2)` of a collision count in the 20 (`01 - Munajat Amir al.mp3`). Names are not padded (a padded name scrolls). Folders, playlists and photos keep their full names, up to 100 bytes each. The first sync renames the files of long titles.
* **Artist and album tags** are worked out from the Latin form by rules, and the genre tag is its Latin form. An artist that fits in 12 characters is kept; otherwise "The" and what is in brackets go, "Various Artists" becomes "Various", and a name of several words becomes the initials of all but the last and the surname ("Mohamed Abdel Wahab" is "M. A. Wahab"), or the first initial and the surname, or the surname alone, or is cut at a word. An album that fits is kept; otherwise brackets, a trailing "EP" or "Single" and a leading artist name go, and then it is cut at a word. A single has no album, which the player shows as "Unknown Album", so its title (shortened the same way) is written there. `mm show <id> --wolfgang` and the fourth tab of `mm --wolfgang` show the result; only the name and its Latin form can change it.
* **Covers.** The photo viewer opens baseline JPEG files of modest size: it fails on PNG and progressive files, on 4:4:4 colour and on very large pictures, and does not list PNG files at all. So the cover is made into a baseline JFIF JPEG, 4:2:0, at most 480 pixels along its longer side (a transparent background becomes white), when syncing: in the MP3 tag, as `cover.jpg` in an album folder and as `Title.jpg` beside a single. The stored cover is not touched.
* **Playlists** are `Playlists/<name>.m3u`, named by the Latin name of the playlist: paths from the root of the card with backslashes (`\Artist\Album\01 - Title.mp3`), one per line, Windows line endings; the player lists every `.m3u` on the card as a playlist and ignores slashes and relative paths. (The player lists an `.m3u8` too, but shows its extension, which it hides for an `.m3u`; so it gets the `.m3u`. Entries with names longer than 26 characters play from a playlist.)
* **Warnings.** `mm sync --wolfgang` (also with `--dry-run`), the review screen of the sync dialog and `mm show --wolfgang` say what the player will not show well: a genre longer than 12 characters, a name with no Latin letters (it becomes "Unknown"), two file names that are the same only because they are cut, and an artist name that looks like several artists.

The player's own playlist files can be read too: `mm playlist import` (see Playlists).

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
mm playlist latin دراسة Study         # the name a device that needs Latin letters gets; leave it out to work it out again
mm playlist delete Exams
mm playlist import USERPL1.PL Resistance --dry-run   # bring in a playlist made on the player itself
```

`mm sync` writes each playlist as `Playlists/<name>.m3u8`: `#EXTM3U`, then for each track a `#EXTINF` line with its length and "Artist - Title" and the path of the file relative to the playlist (`../Artist/Album/01 - Title.mp3`), in UTF-8 with Unix line endings, which players and phones list as a playlist. With `--wolfgang` it is the `.m3u` that player reads instead, named by the Latin name of the playlist (see above). Removing a track from the library takes it out of its playlists. Playlist files count in the `N to sync` of the interface, and a deleted playlist's file is a stray that `mm sync --prune` removes.

The playlists made on the Wolfgang itself (`USERPL1.PL` to `USERPL3.PL` on the card) cannot be written by `mm`, but they can be read: `mm playlist import` takes the file names in one and finds the tracks they were by title, ignoring the track number, case and punctuation. It lists what it found (`~` matched on the start of a title only, `?` fits several tracks, `-` not found), and with `--dry-run` makes nothing. Fix the rest with `mm playlist add` and `mm playlist move`.

Trims are stored on the record and applied when rendering, on MP3 frame
boundaries (about 26 ms), without re-encoding. The stored audio is never
changed, so a trim can always be redone.

## Upgrading from version 1

A library written by an earlier version kept every name twice (in Latin letters, maybe set by hand, and as it really is) and the 12-character names of the Wolfgang per track. `mm` will not open it until `mm migrate` has run: `mm migrate --dry-run` lists the Latin forms that were set by hand, which are kept as the Latin forms of the names (and the player names that go, which are worked out again) and writes nothing; `mm migrate` copies the old `library.json` byte for byte to `.mm/library.json.v1`, writes version 2 and refuses to run twice. Afterwards the first plain `mm sync` renames the library folder to the true names (read `mm sync --dry-run` first, then apply with `--prune --yes`), and the first `mm sync --wolfgang` rewrites the whole card once. An `mm` of version 1 cannot read a version 2 library. A library that was migrated by an earlier build of version 2, which dropped the Latin forms set by hand, gets them back with `mm restore-latin --dry-run` and then `mm restore-latin`: it reads them from `.mm/library.json.v1`, puts one back only where the track still has the name it was set for and has no form of its own, and lists what it left alone.

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
