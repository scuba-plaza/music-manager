# mm: a music library manager for an MP3 player

`mm` is one program that serves a web page on your own computer. It keeps one
record per track in `library.json` and **renders** the music folder from those
records. You edit records in the page, never files; a sync makes the folder match.

```
Artist/Album/03 - Title.mp3     identified tracks (names in their own script, fresh ID3v2.4 tags, a cover file per album)
Artist/Title.mp3                identified singles (their cover is in the tag)
Playlists/<name>.m3u8           playlists
_Inbox/<id> - old name.mp3      tracks not identified yet
.mm/                            stored audio and covers (content-addressed), library.json lives beside it
```

A record holds the truth and nothing for any one device: `title`, `artist` and
`album` as they really are, in any script, one artist per track, and the cover
exactly as it was given, and optionally how a name reads in Latin letters (see Latin forms). Syncing the library folder writes that truth in standard forms
(true names in the tags and the file names, original covers, M3U8 playlists),
which suits a phone, a computer or any player without odd limits. A player that
does have limits gets what it needs worked out from the record while syncing,
by the rules of a **device profile** (see Device profiles): the Wolfgang WMP01 has
one built in, and a file of your own describes any other. Nothing of it is stored.

## What plays the library

On the desktop, the library folder is played as it is by [Gapless](https://gitlab.gnome.org/neithern/g4music). With its music folder set to `~/Music/Library` it groups the tracks by artist and album, shows the embedded covers, lists `Playlists/*.m3u8` as playlists and evens out the volume with the ReplayGain tags (see below). Gapless skips hidden files and folders, which is why `.mm/` (the stored audio and covers) is never played twice. `library.json` and `.mm/` are what `mm` keeps; everything else is rendered from them by a sync.

That makes the folder output: a playlist file is not the playlist. If a playlist is edited inside Gapless (added to or renamed), Gapless rewrites its `.m3u8`, and the next sync puts back the one the records describe. Change playlists in `mm`.

A player with limits is another target: sync to its card, folder or drive and choose its profile, and `mm` writes the names, tags, covers and playlists that player can show (see Device profiles). The library folder always stays the standard form.

## Running it

```sh
mm                                   # serve the page, open it in the browser
mm --library /run/media/me/Music     # another library folder
mm --addr 127.0.0.1:9000 --no-open   # another port, and leave the browser alone
mm --check-config                    # check devices.toml, list the profiles, exit
mm --version
```

| flag | |
|---|---|
| `--library DIR` | the library folder (default `$MM_LIBRARY`, else `~/Music/Library`) |
| `--addr ADDR` | where to listen: this computer only (`127.0.0.1`, `localhost` or `::1`); default `127.0.0.1:7878` |
| `--no-open` | do not open the page in a browser |
| `--config FILE` | the device profiles file (default `$MM_CONFIG`, else `$XDG_CONFIG_HOME/mm/devices.toml`, else `~/.config/mm/devices.toml`) |
| `--check-config` | check the device profiles file, print the names of the profiles and exit (0 when valid, 1 when not) |
| `--version` | print the version and exit |

`mm` has no commands: it serves the page on this computer only, always on the same port (`7878`) so that the page can be bookmarked; if that port is taken, a free one is used, unless you asked for one with `--addr`. It opens the page in your browser with `xdg-open` and stops with Ctrl+C. It needs no terminal of its own and no service: it runs when you start it. Other web pages cannot make it change anything: it answers only requests for its own address, and a change must come from its own page. An unknown flag or a stray argument stops it with an exit code of 2 and names what it did not understand.

Settings:

* `MM_LIBRARY`: the library folder, the default of `--library`.
* `MM_CONFIG`: the device profiles file, the default of `--config`.
* `MM_CONTACT`: an email or URL added to the User-Agent sent to MusicBrainz, which they ask for.

Adding from a link needs `yt-dlp` (and `ffmpeg`, which yt-dlp uses) on `PATH`; ReplayGain needs `ffmpeg` too. Hearing a cut needs nothing more than the browser.

## The web interface

Browse and edit the library: artists, albums and tracks on the left, and on the right whatever the cursor is on: a track's details and its cover, or what is inside an album (its tracks), an artist (its albums and singles), a playlist (its tracks in play order) or the Needs work group (what each track lacks), as the preview column of a file manager does. The page keeps a terminal's look and its keys. Every change is saved at once to `library.json`; the music folder only changes when you sync (`s`), and the header says how many tracks are waiting (`3 to sync`; click it to sync). Press `?` for the list of keys, one page for each kind (`←` `→` or `1`-`4` turn the pages); the bottom line always shows the ones that matter right now.

Several `mm` can work on one library at once, two windows say. A change reads `library.json` as it is at that moment, changes it and writes it back while holding a lock on `library.json.lock`, so no change undoes another; the page notices what the others did within a second or two and shows it. A waiting `mm` carries on within a moment, or says that another `mm` has the library after 30 seconds. If an `mm` is killed, the system lets go of its lock by itself. `library.json.lock` is only a place to lock and can be deleted when no `mm` is running.

The keys, as `?` lists them:

**Moving**

| | |
|---|---|
| `↑↓` `k` `j` | move |
| `PgUp` `PgDn` `ctrl+u` `ctrl+d` | a page at a time |
| `Home` `End` `g` `G` | top / bottom |
| `z` | centre the cursor |
| `+` `=` | open a level |
| `-` | close all |
| `→` `l` | into the fields of a track, artist, album or playlist |
| `enter` | open / close; on a track, into its fields |
| `←` `h` | close, or up a level |
| `tab` | the other pane |
| `/` | filter (esc clears it) |
| `S` | sort the list |
| `esc` | back to the tree, or clear the filter, or the marks |

**Editing**

| | |
|---|---|
| `↑↓` `k` `j` | choose a field |
| `←→` `h` `l` | a track's tabs: Details, Audio, Latin, Device; ← on the first goes back to the tree |
| `1`-`4` | a track's tab by its number (4: Device, when a device profile is chosen) |
| `enter` | edit it; enter again saves (the cover: choose a file) |
| `x` | remove the cover |
| `[` `]` | nudge 0.1 s or 1.5 dB |
| `{` `}` | nudge 1 s or 6 dB |
| `0` | no cut, fade or change |
| `p` | hear 8 s beside it |

**Marking**

| | |
|---|---|
| `space` | mark a track (an album marks all its tracks) |
| `ctrl+a` | mark everything shown |
| `e` | edit the marked tracks together; with none marked, into the fields |
| `d` `Del` | remove (asks first); in a playlist, take a track out of it |
| `J` `K` | move a track down / up |
| `n` | number an album 1…N |
| `p` | add to a playlist |
| `c` | copy the cover, artist, album… of a track |
| `v` | paste them; ctrl+v with a picture sets the cover |

**Library**

| | |
|---|---|
| `I` | identify everything |
| `a` | add from a YouTube link, or drop files on the page |
| `i` | find it on MusicBrainz |
| `o` | use the file's old tags |
| `s` | sync: write the folder |
| `u` `ctrl+r` | undo / redo |
| `D` | check the library |
| `?` | this help |

What the keys do in more detail:

* **Filter.** `/` takes words, which find a track by any of its names, in their own script or in Latin letters (`watani` finds وطني), and `status:unidentified`, `status:identified` or `genre:folk` (any part of the genre, case ignored).
* **Sort.** `S` opens a menu: artist, album, title, track number, length or any of the three dates (see Dates and sorting). It shows all tracks in one flat list in that order, with the date beside each, until you choose the tree again from the same menu; the menu also reverses the order.
* **Fields.** `→` or `e` on a track, an artist, an album or a playlist goes into its fields in the right-hand pane; on a track `enter` goes in too. Of a track, the Details, Audio and Latin tabs hold the fields; the tab you are on stays when you select another track. Above the tabs, a Profile menu chooses a device profile, and the fourth tab, Device, then previews what a sync for that device writes for the track (see Device profiles). The Audio tab has the cuts, the volume and the fades (see Volume and fades); `[` `]` `{` `}` nudge them, `0` clears them and `p` plays the 8 s around the cut or at the start or end of a fade.
* **Marks.** `space` marks tracks (an album marks all of its tracks) and `e` edits the marked ones in one form, which has the Latin forms of the artist, album and genre too. `J` `K` move a track down or up in its album; `n` numbers the album 1…N.
* **Playlists.** `p` adds the marked tracks (or the selected track, album or artist) to a playlist, or starts a new one. The **Playlists** group at the top of the tree opens into each playlist and its tracks in play order; there `J` `K` reorder, `d` takes a track out of the playlist (never out of the library), `d` on the playlist itself deletes it after asking, and `→` edits its name and its Latin name.
* **Identify and add.** `i` finds a track on MusicBrainz and lists the matches with what each would change; `I` does all unidentified tracks as a job with a progress bar, taking confident matches only; `o` uses the file's old tags for what MusicBrainz does not know. `a` adds from a YouTube link: one form, pre-filled from the match, which shows where the file will be written. MP3 files dropped on the page are added as unidentified tracks (the files are only read).
* **Sync.** `s` opens the sync: choose the folder (or a mounted player), see what will change and apply. A folder other than the library folder has a Format row, one of the device profiles with Generic first, switched with `w` and never remembered; the library folder is always written the standard way. Files are only deleted when you ask for the strays to go (`p`) and type `delete`. The header says `N to sync · press s` when something is waiting.
* **Remove, undo, check.** `d` asks first (`y` or `enter` agrees); `u` undoes and `ctrl+r` redoes; `D` checks the library for damaged or missing files and for artist names that look like several artists, and a click on a problem moves the cursor to the track.

The covers are shown as pictures. A picture file dropped on the page, or a picture pasted with `ctrl+v`, becomes the cover of the marked tracks, or of the track, album or artist under the cursor; the Cover field's `enter` chooses a file instead.

The mouse works too: click to select, double-click a row to open or close it (on a track, to go into its fields), click the arrow of a folder row to open or close it at once, click a field to choose it and click it again to edit it, and drag the line between the panes to give one more room. The page is dark, like the terminal it stands in for; the `◐` button switches to a light theme, which is remembered. On a window narrower than 800 pixels it shows one pane at a time. Names in Arabic or any other right-to-left script are laid out in their own direction, without reordering the text around them.

## Device profiles

A device profile says what a sync writes for one player: how names are made, which tags, how the cover is encoded, what a playlist looks like. Two are built in, **`generic`** (the standard form, which the library folder always uses) and **`wolfgang-wmp01`** (see The Wolfgang WMP01). More are described in `devices.toml`, in `$MM_CONFIG`, else `$XDG_CONFIG_HOME/mm/devices.toml`, else `~/.config/mm/devices.toml`, or in the file `--config` names. A missing default file means the built-in profiles only; a missing `--config` file, or a file that is not valid, stops `mm` from starting, since writing the wrong format to a card is worse than not starting. The message names the file, the profile and the key, and `mm --check-config` shows it without starting the page.

The file is an array of tables, `[[profile]]`. A key left out takes the default below, so a profile states only what differs from the standard form. This file describes two players; the second is the Wolfgang's rules written out in full, under another name since a built-in name cannot be used again:

```toml
[[profile]]
name = "my-player"
label = "My player"
latin_names = true
tag_width = 12
cover_format = "baseline-jpeg"
cover_max_edge = 480
cover_quality = 85

[[profile]]
name = "my-wolfgang"
label = "Wolfgang copy"
latin_names = true
component_max = 100
stem_max = 20
tag_width = 12
latin_genre = true
cover_format = "baseline-jpeg"
cover_max_edge = 480
cover_quality = 85
single_covers = true
replaygain = false
playlist_ext = ".m3u"
playlist_style = "root-backslash"
```

| key | default | meaning |
|---|---|---|
| `name` | (required) | lower-case letters, digits and hyphens, starting with a letter or digit; unique, and not `generic` or the name of a built-in |
| `label` | the name | how the profile is called in the menus |
| `latin_names` | `false` | folders, file names and the playlist file name use the Latin form of each name (see Latin forms) |
| `component_max` | `200` | longest folder or file name, in bytes, at least 20 |
| `stem_max` | `0` | longest file name before its extension, in characters, the track number and a ` (2)` included; `0` means no limit, otherwise below `component_max` |
| `tag_width` | `0` | longest artist and album tag, in characters, worked out from their Latin forms by rules (see The Wolfgang WMP01); `0` means the true names; needs `latin_names` |
| `latin_genre` | `false` | the genre tag is the Latin form of the genre |
| `cover_format` | `"keep"` | `"keep"` writes the cover as it was given; `"baseline-jpeg"` makes it a baseline JPEG |
| `cover_max_edge` | `0` | with `baseline-jpeg`, the longest side in pixels, at least 1 |
| `cover_quality` | `0` | with `baseline-jpeg`, the JPEG quality from 1 to 100 |
| `single_covers` | `false` | a single gets a cover file beside it too, `Title.jpg` |
| `replaygain` | `true` | write the ReplayGain tags (see ReplayGain) |
| `playlist_ext` | `".m3u8"` | `".m3u8"` or `".m3u"` |
| `playlist_style` | `"relative"` | `"relative"`: `#EXTM3U`, `#EXTINF` lines and paths relative to the playlist; `"root-backslash"`: paths from the root with backslashes, one per line, Windows line endings |

An unknown key is an error, and so is a duplicate name or a value outside what the table allows. What a sync will not write well is worked out from the keys: a genre longer than `tag_width` (with `latin_genre`), a name with no Latin letters (with `latin_names`), two file names that are the same only because they are cut (with `stem_max`) and an artist name that looks like several artists (with `tag_width`). The sync dialog's review and the Device tab list them; a profile with none of the keys set warns about nothing.

The same file can be written by home-manager (see Installing with Nix).

## Covers

A cover is stored as it was given: a JPEG or a PNG, byte for byte (a file that is not a readable image is refused). A sync of a profile that keeps covers writes it as it is: embedded in the MP3 with its own type, and as `cover.jpg` or `cover.png` in an album folder; a single has none beside it, its cover is in its tag. (A profile with `cover_format = "baseline-jpeg"`, the Wolfgang's, is different, see below.)

Reusing a picture costs nothing in the library: covers are kept once each, under `.mm/covers/<sha256>.img`, however many tracks, albums or artists use them, and the same image file always becomes the same stored cover. (The same picture saved in another format or size is a different file and counts as a different cover.) In the synced folder each album folder gets its own cover file and every MP3 embeds its cover, on purpose: players show the tag, file managers show the file, and a card cannot link one file into several folders. The price is small: in a library of 177 tracks the repeated pictures come to about 6 MB.

## Genre

A track may have a genre, free text such as `Folk`, `Anthem` or `Neoclassical`. A sync writes it to the file as the genre tag. Set it in the Genre field of a track, an artist or an album, in the bulk edit form (`e`) or in the form of `a`; find tracks with the filter `genre:folk`. The tags of an added file are kept as hints, the genre among them, and `o` takes them. (The Wolfgang shows 12 characters of a genre; its sync warns about a longer one.)

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

## Copying and pasting

`c` copies attributes of a track, `v` pastes them on another: a cover that one song of an album lacks is copied from its neighbour in two steps.

* **What is copied.** In the fields of a track, `c` copies the field the cursor is on: the cover, the artist, the album, the number of tracks on the album, the release date or the genre. In the tree, `c` on a track opens a checklist of what it has, all ticked: `space` ticks and unticks, `enter` copies what is ticked. An attribute the track has none of cannot be ticked, so a paste never wipes anything. A name takes the form in Latin letters that was set by hand along with it, since that form belongs to the name. The title, the track number and the audio settings belong to the track and are never copied.
* **Where it is pasted.** `v` pastes on the marked tracks if any are marked, otherwise on the track under the cursor, or on all the tracks of the album or artist under the cursor (whatever the filter shows). It is one step for `u` to undo and says what it did; if the tracks have the same values already it changes nothing. What was copied stays until something else is copied or `mm` is left.
* **A picture from outside.** Copy a PNG or JPEG file in a file manager, or a picture in a browser or an image viewer, then press `ctrl+v` on the page with a track, an album, an artist or marked tracks selected: the picture becomes their cover. Dropping a picture file on the page does the same. Other picture formats (WebP, GIF…) are refused with a message, since a cover is a JPEG or a PNG.

## Latin forms

Names are kept as they really are, in any script. Some places can only show Latin letters: a device profile with `latin_names` (the Wolfgang WMP01, below) and the order of the list, which puts فيروز among the F. For these, each name has a **Latin form**: how it reads in Latin letters. Unless you set one, it is worked out from the name, word by word. That is rough for Arabic, which is written without its short vowels: فيروز comes out as "Firuz" and جوليا بطرس as "Julya Btrs". So a form can be set by hand, for the title, the artist, the album and the genre of a track and for the name of a playlist, and it is used instead of the worked-out one:

* The **Latin** tab (`3`) of a track lists the four. The form shown is the one in use: dim when it is worked out, normal when it was set by hand, `—` when there is no name to write. `enter` edits it starting from what is shown; saving what was there changes nothing, and saving it empty goes back to the worked-out form. The change is one step for `u` to undo, like any other. An artist and an album have a Latin name and a Latin genre among their fields, a playlist a Latin name, and these are set on all their tracks at once (see above). For marked tracks, `e` has rows for the Latin form of the artist, the album and the genre (`-` takes it away).

A form belongs to the name it was written for, and changing the name does not touch it, so a corrected spelling keeps its form; a name that becomes empty loses its form. Forms are searched (`/` finds "watani" as well as وطني) and put the track in order (`S`). The library folder and a sync with the generic profile never use them: the tags and the file names there are the true names, and setting a form leaves nothing to sync. A profile with `latin_names` uses them for the folders, the file names and the name of the playlist file, and with `tag_width` for the artist and album tags and with `latin_genre` for the genre tag; the title tag stays the true title.

## The Wolfgang WMP01

Choose **Wolfgang WMP01** (`wolfgang-wmp01`) in the Format row of the sync dialog, for a folder that is not the library folder, so that the library folder is always the standard tree. Nothing remembers the choice. Everything below is worked out from the records while syncing and stored nowhere: the library holds no name, cover or setting for the player. The limits are the ones measured on the device, and they are the values of its built-in profile (the second profile of the example under Device profiles).

* **Names.** The player shows 24 characters of a file name (it keeps 26), the `.mp3` included, and scrolls a longer name along when it is hovered; it shows 12 of an artist, an album and a genre, cuts the rest wherever that falls, and does not join Arabic letters. So folders and files are named in Latin letters, using the Latin forms (see above): the ones set by hand, else the ones worked out, which lose the short vowels of Arabic (فيروز becomes "Firuz"); accents are stripped. The title tag keeps the true title, since the player shows file names and not titles.
* **File names** are `NN - Title.mp3` for the MP3 files only, hard-cut at 20 characters before the extension (`stem_max = 20`), so that the whole name, `.mp3` included, fits in the 24 the player shows and does not scroll; the track number and the ` (2)` of a collision count in the 20 (`01 - Munajat Amir al.mp3`). Names are not padded (a padded name scrolls). Folders, playlists and photos keep their full names, up to 100 bytes each (`component_max = 100`).
* **Artist and album tags** are worked out from the Latin form by rules (`tag_width = 12`), and the genre tag is its Latin form (`latin_genre`). An artist that fits in 12 characters is kept; otherwise "The" and what is in brackets go, "Various Artists" becomes "Various", and a name of several words becomes the initials of all but the last and the surname ("Mohamed Abdel Wahab" is "M. A. Wahab"), or the first initial and the surname, or the surname alone, or is cut at a word. An album that fits is kept; otherwise brackets, a trailing "EP" or "Single" and a leading artist name go, and then it is cut at a word. A single has no album, which the player shows as "Unknown Album", so its title (shortened the same way) is written there. The Device tab shows the result with this profile chosen; only the name and its Latin form can change it.
* **Covers.** The photo viewer opens baseline JPEG files of modest size: it fails on PNG and progressive files, on 4:4:4 colour and on very large pictures, and does not list PNG files at all. So the cover is made into a baseline JFIF JPEG, 4:2:0, at most 480 pixels along its longer side, quality 85 (a transparent background becomes white), when syncing: in the MP3 tag, as `cover.jpg` in an album folder and as `Title.jpg` beside a single (`single_covers`). The stored cover is not touched.
* **Playlists** are `Playlists/<name>.m3u`, named by the Latin name of the playlist: paths from the root of the card with backslashes (`\Artist\Album\01 - Title.mp3`), one per line, Windows line endings; the player lists every `.m3u` on the card as a playlist and ignores slashes and relative paths. (The player lists an `.m3u8` too, but shows its extension, which it hides for an `.m3u`; so it gets the `.m3u`. Entries with names longer than 26 characters play from a playlist.)
* **Warnings.** The review screen of the sync dialog and the Device tab say what the player will not show well: a genre longer than 12 characters, a name with no Latin letters (it becomes "Unknown"), two file names that are the same only because they are cut, and an artist name that looks like several artists.
* **No ReplayGain.** The Wolfgang sync writes no ReplayGain tags, since the player has no volume setting that could use them.

## Volume and fades

A track can be made louder or quieter and can fade in and out, without being decoded or encoded again: an MP3 stores a gain for every 13 ms granule of each channel, and a sync changes it (one step is 1.5 dB, a fourth of an octave of amplitude), so the sound is not touched and the file does not grow. A volume is therefore rounded to the nearest 1.5 dB and limited to 24 dB either way; boosting a track whose peaks are near full scale can clip. A fade is a ramp of amplitude from silence to full or back, in steps of 1.5 dB down to 60 dB (so a very short fade may sound stepped), and the track is silent before a fade-in starts and after a fade-out ends.

A length counts from where the track now starts (fade-in) or back from where it now ends (fade-out), and is typed as seconds or as `m:ss`; a stretch such as `3:50-3:58` is in the time of the original, as it is stored, so cutting the track again later does not move a fade. The Audio tab (`2`) has Volume, Fade in and Fade out below the cuts, with the same `[ ]` and `{ }` nudge keys (1.5 dB and 6 dB for the volume, 0.1 s and 1 s for the length of a fade), `0` to clear, and `p` to hear the start or the end with the change applied. A line under the fields draws the ramps.

## ReplayGain

A sync with a profile that has `replaygain = true` (the default, and the library folder) writes how loud each track is into its tags as ReplayGain (`REPLAYGAIN_TRACK_GAIN`, `REPLAYGAIN_TRACK_PEAK` and `REPLAYGAIN_REFERENCE_LOUDNESS`), so that a player that evens out volume plays a quiet piano piece and a loud rap track at the same level. In Gapless that is Preferences, ReplayGain, Track. The sound itself is never changed.

The loudness is measured with `ffmpeg` (EBU R128 integrated loudness) on the track as it is played, with its volume, fades and cuts applied, and the gain brings it to -18 LUFS, the ReplayGain 2.0 level. Most music is louder than that, so a player that uses the tags plays quieter than one that does not: turn the volume up. A measurement is remembered under `.mm/loudness`, named by the SHA-256 of the audio, so a track is only measured again when its sound changes; the first sync of a long library takes a few minutes (about 4 for 30 hours), the next ones none.

A track with nothing audible has no loudness and gets no tags. A track `ffmpeg` cannot read is synced without them and listed with the warnings of the sync. If `ffmpeg` is not installed the sync stops, rather than strip the tags from every file. Only the track gain is written, not an album gain, because Gapless is used here with singles and mixed playlists.

## Dates and sorting

Every track has three dates:

- **released**: when the track first came out, as precisely as is known: `1999`, `1999-05` or `1999-05-17`. You set it (the `Released` field, or the bulk-edit form for many tracks at once). It is also filled in from a MusicBrainz match, from yt-dlp's `release_date`, and from the old `TDRC` or `TYER` tag when a file's old tags are used; and it is written into the synced file as its `TDRC` tag, so players show the year.
- **added**: when the track entered the library. It never changes.
- **edited**: when its record was last changed.

The last two are shown under the track in the page. Any of the three, and artist, album, title, track number and length, can order the list with `S`, which shows all tracks in one flat list in that order (with the date beside each) until you choose the tree again. Text and numbers sort lowest first and dates newest first; the menu reverses that. Tracks that have no value, such as no release date, always come last, and ties are ordered by artist, album, track number and title.

## Playlists

A playlist is a named list of tracks in a chosen order; a track can be in as many playlists as you like, but only once in each. Make and change them in the page (see The web interface): `p` adds tracks to one or starts a new one, `J` `K` move a track in it, `d` takes a track out of it or deletes the playlist, and the name and the Latin name are fields of the playlist.

A sync writes each playlist as `Playlists/<name>.m3u8` with the generic profile: `#EXTM3U`, then for each track a `#EXTINF` line with its length and "Artist - Title" and the path of the file relative to the playlist (`../Artist/Album/01 - Title.mp3`), in UTF-8 with Unix line endings, which players and phones list as a playlist. Another profile writes what its `playlist_ext` and `playlist_style` say, named by the Latin name of the playlist when it has `latin_names` (see The Wolfgang WMP01). Removing a track from the library takes it out of its playlists. Playlist files count in the `N to sync` of the header, and a deleted playlist's file is a stray that the sync removes when you ask.

Trims are stored on the record and applied when rendering, on MP3 frame
boundaries (about 26 ms), without re-encoding. The stored audio is never
changed, so a trim can always be redone.

## Bringing in an existing folder

If the music already lives in a folder, drop its MP3 files on the page (they are only read; copy the folder somewhere safe first if you are unsure), press `I` to take the confident MusicBrainz matches and `i` on the tracks it left, `o` to take the old tags for what is still left, and `D` to check the result. Press `s` and read the review before applying: files in the library folder that the records do not account for are only listed as strays, and are deleted only when you ask and type `delete`.

## Installing with Nix

The repository is a flake. It provides `packages.<system>.music-manager` (the
program is `mm`, with its web page built in), `packages.<system>.frontend` (the
built page alone), an overlay that adds `pkgs.music-manager` and a home-manager
module. The Nix package puts `yt-dlp`, `ffmpeg` and `xdg-open` at the end of the
program's `PATH`, so ones you already have take precedence.

```sh
nix run github:scuba-plaza/music-manager                  # try it
nix profile install github:scuba-plaza/music-manager      # or install it
```

With home-manager, add the flake as an input of your system flake and import its module:

```nix
inputs.music-manager = {
  url = "github:scuba-plaza/music-manager";   # or "git+file:///path/to/music-manager"
  inputs.nixpkgs.follows = "nixpkgs";
};
# in the home-manager modules:   music-manager.homeManagerModules.default
```

```nix
programs.music-manager = {
  enable = true;
  library = "/home/me/Music/Library";
  devices.my-player = {
    label = "My player";
    latinNames = true;
    tagWidth = 12;
    coverFormat = "baseline-jpeg";
    coverMaxEdge = 480;
    coverQuality = 85;
  };
};
```

`enable` installs the package (`package` chooses another), `library` sets `MM_LIBRARY` for your sessions, and each attribute of `devices` becomes a profile named after it, written to `~/.config/mm/devices.toml` (only when `devices` is not empty). The options of a profile are its keys in camel case (`latinNames` for `latin_names`, `coverMaxEdge` for `cover_max_edge`); every one is `null` by default and left out of the file when `null`, so the defaults of the table above apply.

The package needs two hashes that Nix checks: when the Go dependencies change, set `vendorHash = lib.fakeHash;` in `nix/package.nix`, and when the packages of the page change, set `hash = "";` in `nix/frontend.nix`; build once, and paste in the hash Nix prints. Nix only sees files git knows about, so `git add` new files before `nix build`.

## Development

```sh
go test ./...                                  # the program
pnpm --dir web run test                        # the page: unit, browser and end-to-end tests
go run ./cmd/e2e-server                        # the seeded library the end-to-end tests run against
go run ./cmd/gentypes                          # print the TypeScript types of the API
nix flake check                                # builds the package and the page, runs their tests and checks the module
nix develop                                    # a shell with go, node, pnpm, yt-dlp, ffmpeg and the test browsers
go test -tags live ./internal/musicbrainz ./internal/ytdlp   # real MusicBrainz, real yt-dlp
```

The page is in `web/`: React and TypeScript, built with Vite, tested with Vitest and Playwright, with pnpm 10 (`pnpm --dir web install --frozen-lockfile` once). The program serves what is in `internal/web/dist`, which `pnpm --dir web run build` fills; until it is built, the page says so and the API at `/api` works. For work on the page, run `mm --library /tmp/lib --no-open` and `pnpm --dir web run dev`, which serves the page with live reload and sends `/api` to that `mm`. The API types in `web/src/api/types.gen.ts` come from the Go code: `pnpm --dir web run gen:types` rewrites them, and a test fails when they are out of date. `pnpm --dir web run check` is the type check, the lint and the unit tests, which `nix build` runs too.
