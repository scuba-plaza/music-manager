package browse

import (
	"fmt"
	"regexp"
	"testing"
)

func TestAShelfGivesBackWhatWasPut(t *testing.T) {
	var s shelf[string]
	if _, ok := s.get("a"); ok {
		t.Fatal("an empty shelf gave something back")
	}
	s.put("a", "one")
	got, ok := s.get("a")
	if !ok || got != "one" {
		t.Fatalf("get = %q, %v", got, ok)
	}
}

func TestAShelfForgetsTheOldestPastItsLimit(t *testing.T) {
	var s shelf[int]
	for i := 0; i <= maxDrafts; i++ {
		s.put(fmt.Sprint(i), i)
	}
	if _, ok := s.get("0"); ok {
		t.Error("the oldest was kept")
	}
	for i := 1; i <= maxDrafts; i++ {
		if v, ok := s.get(fmt.Sprint(i)); !ok || v != i {
			t.Errorf("item %d = %d, %v", i, v, ok)
		}
	}
}

func TestDroppingSomethingNotThereChangesNothing(t *testing.T) {
	var s shelf[int]
	s.drop("x")
	s.put("a", 1)
	s.drop("x")
	if v, ok := s.get("a"); !ok || v != 1 {
		t.Fatalf("get = %d, %v", v, ok)
	}
	s.drop("a")
	if _, ok := s.get("a"); ok {
		t.Fatal("dropped item still there")
	}
}

func TestNewIDsAreSixteenHexCharactersAndDiffer(t *testing.T) {
	a, err := newID()
	if err != nil {
		t.Fatal(err)
	}
	b, err := newID()
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[0-9a-f]{16}$`).MatchString(a) {
		t.Errorf("id %q", a)
	}
	if a == b {
		t.Errorf("ids equal: %q", a)
	}
}
