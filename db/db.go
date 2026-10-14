package db

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"tu/models"
)

var ErrNotFound = errors.New("no tracked date matches")

type Storage struct {
	dates    []models.TuDate
	filepath string
	loaded   bool
}

func NewStorage(filepath string) *Storage {
	return &Storage{
		filepath: filepath,
		dates:    []models.TuDate{},
		loaded:   false,
	}
}

func (s *Storage) Load() error {
	_, err := os.Stat(s.filepath)
	if err != nil {
		dir, _ := filepath.Split(s.filepath)
		if errors.Is(err, os.ErrNotExist) {
			if dir != "" {
				if err := os.MkdirAll(dir, 0755); err != nil {
					return errors.New("Error: Cannot create data directory. " + err.Error())
				}
			}
			f, err := os.Create(s.filepath)
			if err != nil {
				return errors.New("Error: Cannot create data files. " + err.Error())
			}
			f.Close()
		}
	}

	byt, err := os.ReadFile(s.filepath)

	if err != nil {
		return err
	}

	s.dates = []models.TuDate{}
	if len(bytes.TrimSpace(byt)) > 0 {
		// Refuse to continue on a corrupt file, a later write would wipe it.
		if err := json.Unmarshal(byt, &s.dates); err != nil {
			return fmt.Errorf("data file %s is not valid JSON: %w", s.filepath, err)
		}
	}
	s.loaded = true
	return nil
}

func (s *Storage) ensureLoaded() error {
	if s.loaded {
		return nil
	}
	return s.Load()
}

func (s *Storage) Add(tuDate models.TuDate) error {
	if err := s.ensureLoaded(); err != nil {
		return err
	}
	s.dates = append(s.dates, tuDate)
	return s.write()

}

// Read returns a copy of all tracked dates.
func (s *Storage) Read() []models.TuDate {
	return slices.Clone(s.dates)
}

// Find resolves a user supplied reference to a single tracked date. The
// reference may be an ID prefix, an exact name or a unique part of a name
// (all case-insensitive).
func (s *Storage) Find(ref string) (models.TuDate, error) {
	ref = strings.ToLower(strings.TrimSpace(ref))
	if ref == "" {
		return models.TuDate{}, ErrNotFound
	}

	matchers := []func(models.TuDate) bool{
		func(d models.TuDate) bool { return strings.HasPrefix(strings.ToLower(d.UUID), ref) },
		func(d models.TuDate) bool { return strings.ToLower(d.Name) == ref },
		func(d models.TuDate) bool { return strings.Contains(strings.ToLower(d.Name), ref) },
	}
	for _, match := range matchers {
		var found []models.TuDate
		for _, d := range s.dates {
			if match(d) {
				found = append(found, d)
			}
		}
		switch len(found) {
		case 0:
			continue
		case 1:
			return found[0], nil
		default:
			names := make([]string, len(found))
			for i, d := range found {
				names[i] = fmt.Sprintf("%s (%s)", d.DisplayName(), d.ShortID())
			}
			return models.TuDate{}, fmt.Errorf("%q matches several dates: %s", ref, strings.Join(names, ", "))
		}
	}
	return models.TuDate{}, fmt.Errorf("%w %q", ErrNotFound, ref)
}

// Update replaces the stored date that has the same ID.
func (s *Storage) Update(tuDate models.TuDate) error {
	if err := s.ensureLoaded(); err != nil {
		return err
	}
	i := slices.IndexFunc(s.dates, func(d models.TuDate) bool { return d.UUID == tuDate.UUID })
	if i < 0 {
		return fmt.Errorf("%w id %s", ErrNotFound, tuDate.UUID)
	}
	s.dates[i] = tuDate
	return s.write()
}

// ReplaceAll swaps the whole set of dates, e.g. after syncing.
func (s *Storage) ReplaceAll(dates []models.TuDate) error {
	if err := s.ensureLoaded(); err != nil {
		return err
	}
	s.dates = slices.Clone(dates)
	return s.write()
}

// Remove deletes the dates with the given IDs and returns how many were removed.
func (s *Storage) Remove(ids ...string) (int, error) {
	if err := s.ensureLoaded(); err != nil {
		return 0, err
	}
	before := len(s.dates)
	s.dates = slices.DeleteFunc(s.dates, func(d models.TuDate) bool { return slices.Contains(ids, d.UUID) })
	removed := before - len(s.dates)
	if removed == 0 {
		return 0, nil
	}
	return removed, s.write()
}

// write saves atomically, so an interrupted write never truncates the data.
func (s *Storage) write() error {
	byt, err := json.MarshalIndent(s.dates, "", "    ")
	if err != nil {
		return err //edge case, no tests
	}

	tmp, err := os.CreateTemp(filepath.Dir(s.filepath), ".tu-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(byt); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0644); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.filepath)
}
