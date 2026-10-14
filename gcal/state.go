package gcal

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"
)

type Token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	Expiry       time.Time `json:"expiry"`
}

// Deletion is an event that was removed in tu and still has to be deleted
// from Google Calendar.
type Deletion struct {
	ID       string `json:"id"`
	Calendar string `json:"calendar"`
}

// State is everything tu keeps about the Google account: the OAuth client,
// the tokens and the bits of sync bookkeeping that don't belong to a date.
type State struct {
	ClientID     string `json:"client_id,omitempty"`
	ClientSecret string `json:"client_secret,omitempty"`
	Token        *Token `json:"token,omitempty"`
	Account      string `json:"account,omitempty"`

	PendingDeletes []Deletion `json:"pending_deletes,omitempty"`
	// Ignored holds event and series IDs removed from tu that should not be
	// pulled again.
	Ignored  []string  `json:"ignored,omitempty"`
	LastSync time.Time `json:"last_sync,omitzero"`

	path string
}

func LoadState(path string) (*State, error) {
	s := &State{path: path}
	byt, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if len(byt) > 0 {
		if err := json.Unmarshal(byt, s); err != nil {
			return nil, fmt.Errorf("google state file %s is not valid JSON: %w", path, err)
		}
	}
	return s, nil
}

// Save writes the state readable only by the user, as it holds the tokens.
func (s *State) Save() error {
	byt, err := json.MarshalIndent(s, "", "    ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".google-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(byt); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.path)
}

func (s *State) LoggedIn() bool {
	return s.Token != nil && s.Token.RefreshToken != ""
}

// Forget queues the deletion of a pushed event.
func (s *State) Forget(id, calendar string) {
	d := Deletion{ID: id, Calendar: calendar}
	if !slices.Contains(s.PendingDeletes, d) {
		s.PendingDeletes = append(s.PendingDeletes, d)
	}
}

// Ignore stops an event (or a whole series) from being pulled again.
func (s *State) Ignore(id string) {
	if id != "" && !slices.Contains(s.Ignored, id) {
		s.Ignored = append(s.Ignored, id)
	}
}
