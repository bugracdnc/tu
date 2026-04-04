package db

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"tu/models"
)

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
			os.MkdirAll(dir, 0755)
			_, err = os.Create(s.filepath)
			if err != nil {
				log.Fatal("Error: Cannot create data files. " + err.Error())
			}
		}
	}

	byt, err := os.ReadFile(s.filepath)

	if err != nil {
		panic(err)
	}

	err = json.Unmarshal(byt, &s.dates)
	if err != nil {
		log.Println("Warning: Couldn't read json file, could be empty")
	}
	s.loaded = true
	return nil
}

func (s *Storage) Add(tudate models.TuDate) error {
	if !s.loaded {
		s.Load()
	}

	s.dates = append(s.dates, tudate)
	return s.write()

}

func (s *Storage) Read() []models.TuDate {
	return s.dates
}

func (s *Storage) write() error {

	if byt, err := json.MarshalIndent(s.dates, "", "    "); err != nil {
		return err
	} else {
		return os.WriteFile(s.filepath, byt, os.ModeAppend)
	}
}
