// Package profile saves connection profiles to a JSON file; passwords go only to the OS credential store.
package profile

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"

	"github.com/zalando/go-keyring"

	"github.com/codedb-monitorsql/codedb-lens/internal/adapter"
)

const keyringService = "codeDB Lens"

type Profile struct {
	Name   string       `json:"name"`
	Conn   adapter.Conn `json:"conn"`
	Lang   string       `json:"lang,omitempty"`
	OutDir string       `json:"out_dir,omitempty"`
}

type File struct {
	Last     string    `json:"last,omitempty"`
	Profiles []Profile `json:"profiles"`
}

// Path is %APPDATA%\codeDB Lens\profiles.json on Windows, ~/.config/codedb-lens/profiles.json elsewhere.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	name := "codedb-lens"
	if os.PathSeparator == '\\' {
		name = "codeDB Lens"
	}
	return filepath.Join(dir, name, "profiles.json"), nil
}

func Load() (File, error) {
	var f File
	p, err := Path()
	if err != nil {
		return f, err
	}
	raw, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return f, nil
	}
	if err != nil {
		return f, err
	}
	return f, json.Unmarshal(raw, &f)
}

func (f File) Save() error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	raw, _ := json.MarshalIndent(f, "", "  ")
	return os.WriteFile(p, raw, 0o600)
}

func (f File) Get(name string) (Profile, bool) {
	for _, p := range f.Profiles {
		if p.Name == name {
			return p, true
		}
	}
	return Profile{}, false
}

// Put adds or replaces a profile and marks it as last used.
func (f *File) Put(p Profile) {
	for i := range f.Profiles {
		if f.Profiles[i].Name == p.Name {
			f.Profiles[i] = p
			f.Last = p.Name
			return
		}
	}
	f.Profiles = append(f.Profiles, p)
	sort.Slice(f.Profiles, func(i, j int) bool { return f.Profiles[i].Name < f.Profiles[j].Name })
	f.Last = p.Name
}

func (f *File) Delete(name string) {
	out := f.Profiles[:0]
	for _, p := range f.Profiles {
		if p.Name != name {
			out = append(out, p)
		}
	}
	f.Profiles = out
	if f.Last == name {
		f.Last = ""
	}
	_ = keyring.Delete(keyringService, name)
}

// SetPassword stores the password in the OS credential store. Returns an error where none is available
// (headless Linux); callers then ask every time or read CODEDB_LENS_PASSWORD.
func SetPassword(profile, password string) error {
	return keyring.Set(keyringService, profile, password)
}

func Password(profile string) (string, error) {
	return keyring.Get(keyringService, profile)
}
