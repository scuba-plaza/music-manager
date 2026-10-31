package device

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

type ProfileInfo struct {
	Name    string
	Label   string
	Builtin bool
}

type Registry struct{ profiles []Profile }

func Builtin() *Registry {
	return &Registry{profiles: slices.Clone(builtinProfiles)}
}

func LoadRegistry(path string) (*Registry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	user, err := ParseProfiles(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	registry := Builtin()
	for _, p := range user {
		if err := registry.add(p); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
	}
	return registry, nil
}

func (r *Registry) add(p Profile) error {
	if p.Name == Generic.Name() {
		return fmt.Errorf("profile %q: name is reserved for the library folder", p.Name)
	}
	if slices.ContainsFunc(r.profiles, func(known Profile) bool { return known.Name == p.Name }) {
		return fmt.Errorf("profile %q: name is already used by another profile", p.Name)
	}
	r.profiles = append(r.profiles, p)
	return nil
}

func (r *Registry) Get(name string) (Target, bool) {
	for _, p := range r.profiles {
		if p.Name == name {
			return p.Target(), true
		}
	}
	return nil, false
}

func (r *Registry) Profiles() []ProfileInfo {
	infos := make([]ProfileInfo, len(r.profiles))
	for i, p := range r.profiles {
		infos[i] = ProfileInfo{Name: p.Name, Label: p.Label, Builtin: i < len(builtinProfiles)}
	}
	return infos
}

func (r *Registry) Targets() []Target {
	targets := make([]Target, len(r.profiles))
	for i, p := range r.profiles {
		targets[i] = p.Target()
	}
	return targets
}

func ConfigPath(getenv func(string) string, home string) string {
	if path := getenv("MM_CONFIG"); path != "" {
		return path
	}
	if dir := getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "mm", "devices.toml")
	}
	return filepath.Join(home, ".config", "mm", "devices.toml")
}
