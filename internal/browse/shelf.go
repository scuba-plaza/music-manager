package browse

import (
	"crypto/rand"
	"encoding/hex"
)

type shelf[T any] struct {
	items map[string]T
	order []string
}

func (s *shelf[T]) put(id string, v T) {
	if s.items == nil {
		s.items = map[string]T{}
	}
	if _, ok := s.items[id]; !ok {
		s.order = append(s.order, id)
	}
	s.items[id] = v
	for len(s.order) > maxDrafts {
		delete(s.items, s.order[0])
		s.order = s.order[1:]
	}
}

func (s *shelf[T]) get(id string) (T, bool) {
	v, ok := s.items[id]
	return v, ok
}

func (s *shelf[T]) drop(id string) {
	if _, ok := s.items[id]; !ok {
		return
	}
	delete(s.items, id)
	for i, o := range s.order {
		if o == id {
			s.order = append(s.order[:i:i], s.order[i+1:]...)
			break
		}
	}
}

func newID() (string, error) {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}
