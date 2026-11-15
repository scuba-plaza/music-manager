package app

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"music-manager/internal/device"
	"music-manager/internal/manifest"
	"music-manager/internal/translit"
)

var ErrNoFormat = errors.New("that format is gone already")

func (a *App) Profiles() ([]manifest.Profile, error) {
	m, err := a.store.Load()
	if err != nil {
		return nil, err
	}
	return slices.Clone(m.Profiles), nil
}

func (a *App) Format(id string) (device.Target, error) {
	if id == device.GenericProfile.ID {
		return device.Generic, nil
	}
	profiles, err := a.Profiles()
	if err != nil {
		return nil, err
	}
	for _, p := range profiles {
		if p.ID == id {
			return device.TargetOf(p), nil
		}
	}
	return nil, fmt.Errorf("there is no format with the id %q: %w", id, ErrNoFormat)
}

func (a *App) CheckProfile(p manifest.Profile) (FieldErrors, error) {
	m, err := a.store.Load()
	if err != nil {
		return nil, err
	}
	var fields FieldErrors
	if err := check(tidy(p), m.Profiles, p.ID); err != nil {
		errors.As(err, &fields)
	}
	return fields, nil
}

func (a *App) CreateProfile(p manifest.Profile) (manifest.Profile, error) {
	p = tidy(p)
	err := a.store.Update(func(m *manifest.Manifest) error {
		if len(m.Profiles) >= manifest.MaxProfiles {
			return fmt.Errorf("there can be at most %d formats besides Generic: delete one first", manifest.MaxProfiles)
		}
		if err := check(p, m.Profiles, ""); err != nil {
			return err
		}
		p.ID = freshID(p.Name, m.Profiles)
		m.Profiles = append(m.Profiles, p)
		return nil
	})
	return p, err
}

func (a *App) UpdateProfile(id string, p manifest.Profile) (manifest.Profile, error) {
	p = tidy(p)
	err := a.store.Update(func(m *manifest.Manifest) error {
		i := slices.IndexFunc(m.Profiles, func(known manifest.Profile) bool { return known.ID == id })
		if i < 0 {
			return fmt.Errorf("there is no format with the id %q: %w", id, ErrNoFormat)
		}
		if err := check(p, m.Profiles, id); err != nil {
			return err
		}
		p.ID = id
		m.Profiles[i] = p
		return nil
	})
	return p, err
}

func (a *App) DeleteProfile(id string) error {
	return a.store.Update(func(m *manifest.Manifest) error {
		i := slices.IndexFunc(m.Profiles, func(known manifest.Profile) bool { return known.ID == id })
		if i < 0 {
			return ErrNoFormat
		}
		m.Profiles = slices.Delete(m.Profiles, i, i+1)
		return nil
	})
}

func tidy(p manifest.Profile) manifest.Profile {
	p.Name = strings.TrimSpace(p.Name)
	if p.CoverFormat == "keep" {
		p.CoverMaxEdge, p.CoverQuality = 0, 0
	}
	return p
}

func check(p manifest.Profile, others []manifest.Profile, own string) error {
	bad := FieldErrors{}
	for field, message := range p.Problems() {
		bad.Set(field, message)
	}
	if _, found := bad["name"]; !found {
		for _, other := range others {
			if other.ID != own && strings.EqualFold(other.Name, p.Name) {
				bad.Set("name", fmt.Sprintf("There is already a format called %q", other.Name))
			}
		}
	}
	return bad.OrNil()
}

func freshID(name string, taken []manifest.Profile) string {
	base := slug(name)
	id := base
	for n := 2; slices.ContainsFunc(taken, func(p manifest.Profile) bool { return p.ID == id }) || id == device.GenericProfile.ID; n++ {
		id = base + "-" + strconv.Itoa(n)
	}
	return id
}

func slug(name string) string {
	var b strings.Builder
	dash := true
	for _, r := range strings.ToLower(translit.ToASCII(name)) {
		switch {
		case r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(r)
			dash = false
		case !dash:
			b.WriteByte('-')
			dash = true
		}
	}
	if s := strings.Trim(b.String(), "-"); s != "" {
		return s
	}
	return "format"
}
