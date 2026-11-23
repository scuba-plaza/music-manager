package tsgen

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"
)

func generate(t *testing.T, roots ...any) string {
	t.Helper()
	out, err := Generate(roots...)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func check(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

type Song struct {
	Title  string    `json:"title"`
	Plays  int64     `json:"plays"`
	Rating float64   `json:"rating"`
	Liked  bool      `json:"liked"`
	Added  time.Time `json:"added"`
	Notes  string
	Secret string `json:"-"`
	hidden string
}

func TestAStructBecomesAnInterface(t *testing.T) {
	_ = Song{}.hidden
	check(t, generate(t, Song{}), header+`
export interface Song {
  title: string;
  plays: number;
  rating: number;
  liked: boolean;
  added: string;
  Notes: string;
}
`)
}

type Draft struct {
	Name  string `json:"name,omitempty"`
	Count int    `json:",omitempty"`
	Kind  string `json:"kind"`
}

func TestOmitemptyIsOptional(t *testing.T) {
	check(t, generate(t, Draft{}), header+`
export interface Draft {
  name?: string;
  Count?: number;
  kind: string;
}
`)
}

type Album struct {
	Cover *Song   `json:"cover"`
	Best  *Song   `json:"best,omitempty"`
	Count *int    `json:"count"`
	Picks []*Song `json:"picks,omitempty"`
}

func TestPointersAreNullable(t *testing.T) {
	check(t, generate(t, &Album{}), header+`
export interface Album {
  cover: Song | null;
  best?: Song;
  count: number | null;
  picks?: (Song | null)[];
}

export interface Song {
  title: string;
  plays: number;
  rating: number;
  liked: boolean;
  added: string;
  Notes: string;
}
`)
}

type Shelf struct {
	Names  []string            `json:"names"`
	Tags   []string            `json:"tags,omitempty"`
	Pair   [2]int              `json:"pair"`
	Grid   [][]int             `json:"grid,omitempty"`
	Raw    []byte              `json:"raw"`
	Counts map[string]int      `json:"counts"`
	Moves  map[string]string   `json:"moves,omitempty"`
	Drafts map[string][]Draft  `json:"drafts,omitempty"`
	ByKind map[kindName]string `json:"byKind,omitempty"`
}

type kindName string

func TestSlicesAndMaps(t *testing.T) {
	check(t, generate(t, Shelf{}), header+`
export interface Draft {
  name?: string;
  Count?: number;
  kind: string;
}

export interface Shelf {
  names: string[];
  tags?: string[];
  pair: number[];
  grid?: number[][];
  raw: string;
  counts: Record<string, number>;
  moves?: Record<string, string>;
  drafts?: Record<string, Draft[]>;
  byKind?: Record<string, string>;
}
`)
}

type StringKeys struct {
	ByName map[string]string `json:"byName"`
}

type IntKeys struct {
	ByNumber map[int]string `json:"byNumber"`
}

func TestOnlyAMapWithStringKeysCanBecomeARecord(t *testing.T) {
	check(t, generate(t, StringKeys{}), header+`
export interface StringKeys {
  byName: Record<string, string>;
}
`)
	if _, err := Generate(IntKeys{}); err == nil || !strings.Contains(err.Error(), "IntKeys.ByNumber") {
		t.Fatalf("want an error naming IntKeys.ByNumber, got %v", err)
	}
}

type Base struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
}

type Extra struct {
	Note string `json:"note,omitempty"`
}

type Tagged struct {
	Label string `json:"label"`
}

type Change struct {
	Base
	*Extra
	Tagged  `json:"tagged"`
	Kind    int    `json:"kind"`
	History string `json:"history"`
}

func TestEmbeddedStructsAreFlattened(t *testing.T) {
	check(t, generate(t, Change{}), header+`
export interface Change {
  id: string;
  note?: string;
  tagged: Tagged;
  kind: number;
  history: string;
}

export interface Tagged {
  label: string;
}
`)
	raw, err := json.Marshal(Change{Extra: &Extra{Note: "n"}})
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"id":"","note":"n","tagged":{"label":""},"kind":0,"history":""}`; string(raw) != want {
		t.Errorf("encoding/json writes %s, want %s", raw, want)
	}
}

type Loose struct {
	Data    any             `json:"data,omitempty"`
	Old     interface{}     `json:"old"`
	Raw     json.RawMessage `json:"raw"`
	Fault   error           `json:"fault"`
	Company string          `json:"company"`
	Anyway  []any           `json:"anyway"`
}

func TestUnknownReplacesAny(t *testing.T) {
	out := generate(t, Loose{})
	if regexp.MustCompile(`\bany\b`).MatchString(out) {
		t.Errorf("the output says any:\n%s", out)
	}
	check(t, out, header+`
export interface Loose {
  data?: unknown;
  old: unknown;
  raw: unknown;
  fault: unknown;
  company: string;
  anyway: unknown[];
}
`)
}

type Zebra struct {
	A Mango `json:"a"`
}

type Mango struct {
	B Apple `json:"b"`
}

type Apple struct {
	C string `json:"c"`
}

type Nothing struct{}

func TestOutputIsDeterministic(t *testing.T) {
	first := generate(t, Zebra{}, Nothing{}, Apple{})
	for range 20 {
		if again := generate(t, Nothing{}, Apple{}, Zebra{}); again != first {
			t.Fatalf("two runs differ:\n%s\n---\n%s", first, again)
		}
	}
	check(t, first, header+`
export interface Apple {
  c: string;
}

export interface Mango {
  b: Apple;
}

export type Nothing = Record<string, never>;

export interface Zebra {
  a: Mango;
}
`)
}

type Callback struct {
	Name   string     `json:"name"`
	OnDone func()     `json:"onDone"`
	Ticks  chan int   `json:"ticks"`
	Inner  []Callback `json:"inner"`
}

type Ticker struct {
	Ticks chan int `json:"ticks"`
}

type Stamp struct{ n int }

func (s Stamp) MarshalJSON() ([]byte, error) { return json.Marshal(s.n) }

type Stamped struct {
	At Stamp `json:"at"`
}

type Odd struct {
	Weird string `json:"x-y"`
}

type Quoted struct {
	Count int `json:"count,string"`
}

func TestWhatTheGeneratorCannotWriteIsAnErrorNotAGuess(t *testing.T) {
	type Song struct {
		Name string `json:"name"`
	}
	type Pair struct {
		Local Song `json:"local"`
	}
	for _, c := range []struct {
		name  string
		roots []any
		names string
	}{
		{"a function field", []any{Callback{}}, "Callback.OnDone"},
		{"a channel field", []any{Ticker{}}, "Ticker.Ticks"},
		{"a type that marshals itself", []any{Stamped{}}, "Stamped.At"},
		{"a root that is not a struct", []any{"stamp"}, "string"},
		{"two types of one name", []any{Album{}, Pair{}}, "Song"},
		{"a name that is no identifier", []any{Odd{}}, "Odd.Weird"},
		{"the string option", []any{Quoted{}}, "Quoted.Count"},
	} {
		if _, err := Generate(c.roots...); err == nil || !strings.Contains(err.Error(), c.names) {
			t.Errorf("%s: want an error naming %s, got %v", c.name, c.names, err)
		}
	}
}

type Required struct {
	Code string `json:"code"`
}

type Holder struct {
	*Required
	Name string `json:"name"`
}

func TestFieldsOfAnEmbeddedPointerAreOptionalBecauseANilOneWritesNone(t *testing.T) {
	check(t, generate(t, Holder{}), header+`
export interface Holder {
  code?: string;
  name: string;
}
`)
	raw, err := json.Marshal(Holder{})
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"name":""}`; string(raw) != want {
		t.Errorf("encoding/json writes %s, want %s", raw, want)
	}
}

type Nested struct {
	Inner Song      `json:"inner,omitempty"`
	When  time.Time `json:"when,omitempty"`
	Zero  Song      `json:"zero,omitzero"`
}

func TestOmitemptyOfAStructIsNotOptionalButOmitzeroIs(t *testing.T) {
	out := generate(t, Nested{})
	for _, want := range []string{"  inner: Song;\n", "  when: string;\n", "  zero?: Song;\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("the output lacks %q:\n%s", want, out)
		}
	}
	raw, err := json.Marshal(Nested{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"inner"`) || !strings.Contains(string(raw), `"when"`) || strings.Contains(string(raw), `"zero"`) {
		t.Errorf("encoding/json writes %s", raw)
	}
}
