package pocket

import (
	"cmp"
	"encoding/binary"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"music-manager/internal/manifest"
)

const (
	Folder = "PLAYER"
	Major  = 1
	Minor  = 0
	none   = 0xFFFF
)

type Library struct {
	Tracks    []manifest.Track
	Playlists []manifest.Playlist
	Paths     map[string]string
}

type song struct {
	title, sort        string
	file               string
	artist, album, gen int
	number, total      int
	length             uint32
	year               int
	added              int
}

type group struct {
	name, sort string
	songs      []int
	artist     int
	albums     []int
}

type index struct {
	songs                    []song
	albums, artists, genres  []*group
	playlists                []*group
	albumOf, artistOf, genOf map[string]int
}

func Files(lib Library) map[string][]byte {
	x := build(lib)
	w := writer{strings: map[string]uint32{}}
	files := map[string][]byte{}
	put := func(name string, data []byte) { files[Folder+"/"+name] = data }

	var tracks, albums, artists, genres, playlists, lists []byte
	list := func(items []int) []byte {
		at := uint32(len(lists) / 2)
		for _, i := range items {
			lists = binary.LittleEndian.AppendUint16(lists, uint16(i))
		}
		b := binary.LittleEndian.AppendUint32(nil, at)
		return binary.LittleEndian.AppendUint16(b, uint16(len(items)))
	}
	for _, s := range x.songs {
		b := w.ref(nil, s.title)
		b = w.ref(b, s.file)
		b = u16(b, s.artist, s.album, s.gen)
		b = append(b, byte(s.number), byte(s.total))
		b = binary.LittleEndian.AppendUint32(b, s.length)
		tracks = append(tracks, u16(b, s.year, s.added)...)
	}
	for _, g := range x.albums {
		b := w.ref(nil, g.name)
		b = u16(b, g.artist, x.mainGenre(g.songs), x.year(g.songs), x.added(g.songs))
		b = binary.LittleEndian.AppendUint32(b, x.length(g.songs))
		b = append(b, list(g.songs)...)
		albums = append(albums, u16(b, 0)...)
	}
	for _, g := range x.artists {
		b := w.ref(nil, g.name)
		b = u16(b, x.mainGenre(g.songs), len(g.albums))
		b = binary.LittleEndian.AppendUint32(b, x.length(g.songs))
		b = append(b, list(g.songs)...)
		b = u16(b, 0)
		b = append(b, list(g.albums)[:4]...)
		artists = append(artists, b...)
	}
	for _, g := range x.genres {
		b := w.ref(nil, g.name)
		b = u16(b, x.count(g.songs, func(s song) int { return s.artist }), x.count(g.songs, func(s song) int { return s.album }))
		b = binary.LittleEndian.AppendUint32(b, x.length(g.songs))
		b = append(b, list(g.songs)...)
		genres = append(genres, u16(b, 0)...)
	}
	for _, g := range x.playlists {
		b := w.ref(nil, g.name)
		b = binary.LittleEndian.AppendUint32(b, x.length(g.songs))
		b = append(b, list(g.songs)...)
		playlists = append(playlists, u16(b, 0)...)
	}

	header := append([]byte("PKPL"), u16(nil, Major, Minor, len(x.songs), len(x.albums), len(x.artists), len(x.genres), len(x.playlists))...)
	put("HEADER.BIN", header)
	put("TRACKS.BIN", tracks)
	put("ALBUMS.BIN", albums)
	put("ARTISTS.BIN", artists)
	put("GENRES.BIN", genres)
	put("PLAYLIST.BIN", playlists)
	put("LISTS.BIN", lists)

	songs := func(less func(a, b song) int) []int {
		return order(len(x.songs), func(i, j int) int {
			return cmp.Or(less(x.songs[i], x.songs[j]), az(x.songs[i].sort, x.songs[j].sort))
		})
	}
	groups := func(gs []*group, less func(a, b *group) int) []int {
		return order(len(gs), func(i, j int) int { return cmp.Or(less(gs[i], gs[j]), az(gs[i].sort, gs[j].sort)) })
	}
	most := func(a, b *group) int { return cmp.Compare(len(b.songs), len(a.songs)) }
	sortOf := func(gs []*group, i int) string {
		if i == none {
			return ""
		}
		return gs[i].sort
	}

	songAZ := songs(func(a, b song) int { return 0 })
	put("SONGAZ.ORD", u16(nil, songAZ...))
	put("SONGAZ.LET", letters(songAZ, func(i int) string { return x.songs[i].sort }))
	put("SONGART.ORD", u16(nil, songs(func(a, b song) int {
		return cmp.Or(az(sortOf(x.artists, a.artist), sortOf(x.artists, b.artist)), az(sortOf(x.albums, a.album), sortOf(x.albums, b.album)), cmp.Compare(a.number, b.number))
	})...))
	put("SONGADD.ORD", u16(nil, songs(func(a, b song) int { return cmp.Compare(b.added, a.added) })...))
	put("SONGYEAR.ORD", u16(nil, songs(func(a, b song) int { return cmp.Compare(b.year, a.year) })...))

	albumAZ := groups(x.albums, func(a, b *group) int { return 0 })
	put("ALBAZ.ORD", u16(nil, albumAZ...))
	put("ALBAZ.LET", letters(albumAZ, func(i int) string { return x.albums[i].sort }))
	put("ALBART.ORD", u16(nil, groups(x.albums, func(a, b *group) int {
		return cmp.Or(az(sortOf(x.artists, a.artist), sortOf(x.artists, b.artist)), cmp.Compare(x.year(a.songs), x.year(b.songs)))
	})...))
	put("ALBADD.ORD", u16(nil, groups(x.albums, func(a, b *group) int { return cmp.Compare(x.added(b.songs), x.added(a.songs)) })...))
	put("ALBYEAR.ORD", u16(nil, groups(x.albums, func(a, b *group) int { return cmp.Compare(x.year(b.songs), x.year(a.songs)) })...))

	artistAZ := groups(x.artists, func(a, b *group) int { return 0 })
	put("ARTAZ.ORD", u16(nil, artistAZ...))
	put("ARTAZ.LET", letters(artistAZ, func(i int) string { return x.artists[i].sort }))
	put("ARTMOST.ORD", u16(nil, groups(x.artists, most)...))

	genreAZ := groups(x.genres, func(a, b *group) int { return 0 })
	put("GENAZ.ORD", u16(nil, genreAZ...))
	put("GENAZ.LET", letters(genreAZ, func(i int) string { return x.genres[i].sort }))
	put("GENMOST.ORD", u16(nil, groups(x.genres, most)...))

	put("STRINGS.BIN", w.pool)
	return files
}

func build(lib Library) *index {
	x := &index{albumOf: map[string]int{}, artistOf: map[string]int{}, genOf: map[string]int{}}
	byID := map[string]int{}
	entity := func(gs *[]*group, of map[string]int, key, name, sort string) int {
		if name == "" {
			return none
		}
		if i, ok := of[key]; ok {
			return i
		}
		*gs = append(*gs, &group{name: name, sort: sortKey(sort), artist: none})
		of[key] = len(*gs) - 1
		return len(*gs) - 1
	}
	for _, t := range lib.Tracks {
		title := t.Title
		sort := t.LatinTitle()
		if title == "" {
			title = strings.TrimSuffix(path.Base(lib.Paths[t.ID]), path.Ext(lib.Paths[t.ID]))
			sort = title
		}
		s := song{
			title: clip(title), sort: sortKey(sort), file: lib.Paths[t.ID],
			number: min(t.TrackNumber, 255), total: min(t.TrackTotal, 255),
			length: uint32(t.PlayingLength() / time.Millisecond),
			year:   yearOf(t.Released), added: int(t.AddedAt.Unix() / 86400),
		}
		s.artist = entity(&x.artists, x.artistOf, t.Artist, t.Artist, t.LatinArtist())
		s.album = entity(&x.albums, x.albumOf, t.Album+"\x00"+t.Artist, t.Album, t.LatinAlbum())
		s.gen = entity(&x.genres, x.genOf, t.Genre, t.Genre, t.LatinGenre())
		byID[t.ID] = len(x.songs)
		x.songs = append(x.songs, s)
	}
	for i, s := range x.songs {
		for _, g := range []struct {
			gs []*group
			at int
		}{{x.artists, s.artist}, {x.albums, s.album}, {x.genres, s.gen}} {
			if g.at != none {
				g.gs[g.at].songs = append(g.gs[g.at].songs, i)
			}
		}
		if s.album != none {
			x.albums[s.album].artist = s.artist
		}
	}
	for _, a := range x.albums {
		slices.SortStableFunc(a.songs, func(i, j int) int { return cmp.Compare(x.songs[i].number, x.songs[j].number) })
	}
	for _, g := range x.genres {
		slices.SortStableFunc(g.songs, func(i, j int) int { return az(x.songs[i].sort, x.songs[j].sort) })
	}
	for i, a := range x.albums {
		if a.artist != none {
			x.artists[a.artist].albums = append(x.artists[a.artist].albums, i)
		}
	}
	for _, g := range x.artists {
		slices.SortStableFunc(g.albums, func(i, j int) int { return az(x.albums[i].sort, x.albums[j].sort) })
		slices.SortStableFunc(g.songs, func(i, j int) int {
			a, b := x.songs[i], x.songs[j]
			albumSort := func(s song) string {
				if s.album == none {
					return ""
				}
				return x.albums[s.album].sort
			}
			return cmp.Or(az(albumSort(a), albumSort(b)), cmp.Compare(a.number, b.number))
		})
	}
	for _, p := range lib.Playlists {
		g := &group{name: p.Name, sort: sortKey(p.LatinName()), artist: none}
		for _, id := range p.Tracks {
			if i, ok := byID[id]; ok {
				g.songs = append(g.songs, i)
			}
		}
		x.playlists = append(x.playlists, g)
	}
	for _, gs := range [][]*group{x.albums, x.artists, x.genres, x.playlists} {
		for _, g := range gs {
			g.name = clip(g.name)
		}
	}
	return x
}

func (x *index) length(songs []int) uint32 {
	var total uint32
	for _, i := range songs {
		total += x.songs[i].length
	}
	return total
}

func (x *index) year(songs []int) int {
	year := 0
	for _, i := range songs {
		if y := x.songs[i].year; y != 0 && (year == 0 || y < year) {
			year = y
		}
	}
	return year
}

func (x *index) added(songs []int) int {
	added := 0
	for _, i := range songs {
		added = max(added, x.songs[i].added)
	}
	return added
}

func (x *index) mainGenre(songs []int) int {
	counts := map[int]int{}
	best := none
	for _, i := range songs {
		g := x.songs[i].gen
		if g == none {
			continue
		}
		counts[g]++
		if best == none || counts[g] > counts[best] || (counts[g] == counts[best] && az(x.genres[g].sort, x.genres[best].sort) < 0) {
			best = g
		}
	}
	return best
}

func (x *index) count(songs []int, of func(song) int) int {
	seen := map[int]bool{}
	for _, i := range songs {
		if v := of(x.songs[i]); v != none {
			seen[v] = true
		}
	}
	return len(seen)
}

func sortKey(latin string) string { return strings.ToUpper(strings.TrimSpace(latin)) }

func bucket(sort string) int {
	if sort != "" && sort[0] >= 'A' && sort[0] <= 'Z' {
		return int(sort[0] - 'A')
	}
	return 26
}

func az(a, b string) int { return cmp.Or(cmp.Compare(bucket(a), bucket(b)), strings.Compare(a, b)) }

func order(n int, less func(i, j int) int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	slices.SortStableFunc(out, less)
	return out
}

func letters(order []int, sortOf func(int) string) []byte {
	starts := make([]int, 28)
	for b := range 27 {
		starts[b] = len(order)
		for pos, i := range order {
			if bucket(sortOf(i)) >= b {
				starts[b] = pos
				break
			}
		}
	}
	starts[27] = len(order)
	return u16(nil, starts...)
}

func yearOf(released string) int {
	if len(released) < 4 {
		return 0
	}
	y, err := strconv.Atoi(released[:4])
	if err != nil {
		return 0
	}
	return y
}

func clip(s string) string {
	for len(s) > 255 {
		_, size := utf8.DecodeLastRuneInString(s)
		s = s[:len(s)-size]
	}
	return s
}

func u16(b []byte, vs ...int) []byte {
	for _, v := range vs {
		b = binary.LittleEndian.AppendUint16(b, uint16(v))
	}
	return b
}

type writer struct {
	pool    []byte
	strings map[string]uint32
}

func (w *writer) ref(b []byte, s string) []byte {
	at, ok := w.strings[s]
	if !ok {
		at = uint32(len(w.pool))
		w.strings[s] = at
		w.pool = append(w.pool, byte(len(s)))
		w.pool = append(w.pool, s...)
	}
	return binary.LittleEndian.AppendUint32(b, at)
}
