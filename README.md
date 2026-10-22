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

`mm add` asks for title, artist, album, track number, tracks on the album and
how much to cut off the start and end, with defaults filled in. Anything given
as a flag is not asked. Without a terminal (or with `--no-input`) it never
asks and fails if the title or artist is unknown.

## Commands

| | |
|---|---|
| `mm ingest <file\|dir>…` | add existing MP3s as unidentified tracks (the files are only read) |
| `mm identify [id…] [--yes]` | look tracks up on MusicBrainz; pick from a list, or `--yes` for confident matches |
| `mm add <url>` | download with yt-dlp and add |
| `mm edit <id> --title … --artist … --album … --track N --of N --trim-start T --trim-end T --cover FILE` | change a track; `--title-ascii` etc. override the derived ASCII |
| `mm ls [words] [status:identified\|unidentified]` / `mm show <id>` | look at records |
| `mm sync [--target DIR] [--dry-run] [--prune --yes]` | render the folder; `--prune` deletes files the records don't account for |
| `mm rm <id>…` | remove records (audio is deleted by the next sync) |
| `mm doctor` | check the stores for missing or damaged files |

Trims are stored on the record and applied when rendering, on MP3 frame
boundaries (about 26 ms), without re-encoding. The stored audio is never
changed, so a trim can always be redone.

## Moving an existing folder over

The library folder is also where the old music lives, so ingest first, check
the result with `--dry-run`, and only then prune:

```sh
cp -r ~/Music/mp3-player ~/mp3-player.backup
mm ingest ~/Music/mp3-player
mm identify --yes          # then `mm identify` for the rest, or `mm edit`
mm sync --dry-run --prune
mm sync --prune --yes
```

## Settings

* `--library DIR` or `MM_LIBRARY`: the library folder (default `~/Music/mp3-player`).
* `MM_CONTACT`: an email or URL added to the User-Agent sent to MusicBrainz, which they ask for.
* `ACCESSIBLE=1` or `TERM=dumb`: plain line-based questions instead of the full-screen form.

Needs `yt-dlp` (and `ffmpeg`, which yt-dlp uses) on `PATH` for `mm add`.

## Development

```sh
go test ./...
go test -tags live ./internal/musicbrainz ./internal/ytdlp   # real MusicBrainz, real yt-dlp
```
