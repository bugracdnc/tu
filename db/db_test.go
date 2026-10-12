package db

import (
	"errors"
	"os"
	"testing"
	"time"
	"tu/models"
)

var TestingStorage *Storage
var TestFilePath = "testing/test.db"
var TestFilePath_BrokenFile = "testing/test_broken.db"

func TestNewStorage(t *testing.T) {
	TestingStorage = NewStorage(TestFilePath)

	if TestingStorage == nil {
		t.Error("Failed to create struct object")
	}
}

func TestStorageLoad(t *testing.T) {
	if TestingStorage == nil {
		TestingStorage = NewStorage(TestFilePath)
	}

	if err := TestingStorage.Load(); err != nil {
		t.Errorf("%s", err.Error())
	}
}

func TestStorageLoad_WithoutFile(t *testing.T) {
	if TestingStorage == nil {
		TestingStorage = NewStorage(TestFilePath)
	}

	os.Remove(TestFilePath)

	if err := TestingStorage.Load(); err != nil {
		t.Errorf("%s", err.Error())
	}
}

func TestStorageLoad_BrokenFile(t *testing.T) {
	BrokenStorage := NewStorage(TestFilePath_BrokenFile)

	os.WriteFile(TestFilePath_BrokenFile, []byte{}, os.ModeDir)

	if err := BrokenStorage.Load(); err == nil {
		t.Errorf("wanted error, got a good struct instead: %v", BrokenStorage)
	}
}

func TestStorageLoad_NoPerm(t *testing.T) {
	NoPermStorage := NewStorage("/test.db")

	os.WriteFile(TestFilePath_BrokenFile, []byte{}, os.ModeDir)

	if err := NoPermStorage.Load(); err == nil {
		t.Errorf("wanted error, got a good struct instead: %v", NoPermStorage)
	}
}

func TestStorageAdd(t *testing.T) {
	if TestingStorage == nil {
		TestingStorage = NewStorage(TestFilePath)
	}

	if err := TestingStorage.Add(*models.NewTuDateFromDateWithName(time.Now(), "testing environment")); err != nil {
		t.Errorf("%s", err.Error())
	}
}

func TestStorageAdd_WithoutLoad(t *testing.T) {
	TestingStorage = NewStorage(TestFilePath)

	if err := TestingStorage.Add(*models.NewTuDateFromDateWithName(time.Now(), "testing environment")); err != nil {
		t.Errorf("%s", err.Error())
	}
}

func TestStorageAdd_BadDateString(t *testing.T) {
	if TestingStorage == nil {
		TestingStorage = NewStorage(TestFilePath)
	}

	tuDate, err := models.NewTuDateFromStringWithName("2026-4-3 5:4:12", "testing environment")

	if err == nil {
		if err := TestingStorage.Add(*tuDate); err == nil {
			t.Errorf("wanted error, got nil")
		}
	}
}

func TestStorageRead(t *testing.T) {
	if TestingStorage == nil {
		TestingStorage = NewStorage(TestFilePath)
	}

	readSlice := TestingStorage.Read()
	for i, read := range readSlice {
		if read != TestingStorage.dates[i] {
			t.Errorf("what is read and exists are not matching")
		}
	}
}

func TestStorageFindUpdateRemove(t *testing.T) {
	path := t.TempDir() + "/data.db"
	s := NewStorage(path)
	if err := s.Load(); err != nil {
		t.Fatal(err)
	}
	exam := *models.NewTuDateFromDateWithName(time.Now(), "Final Exam")
	midterm := *models.NewTuDateFromDateWithName(time.Now(), "Midterm exam")
	s.Add(exam)
	s.Add(midterm)

	if d, err := s.Find(exam.UUID[:4]); err != nil || d.UUID != exam.UUID {
		t.Errorf("find by id prefix: got %v %v", d, err)
	}
	if d, err := s.Find("final exam"); err != nil || d.UUID != exam.UUID {
		t.Errorf("find by name: got %v %v", d, err)
	}
	if d, err := s.Find("mid"); err != nil || d.UUID != midterm.UUID {
		t.Errorf("find by part of name: got %v %v", d, err)
	}
	if _, err := s.Find("exam"); err == nil {
		t.Error("wanted ambiguity error")
	}
	if _, err := s.Find("nothing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("wanted ErrNotFound, got %v", err)
	}

	exam.Name = "Renamed"
	if err := s.Update(exam); err != nil {
		t.Fatal(err)
	}
	if n, err := s.Remove(midterm.UUID); err != nil || n != 1 {
		t.Errorf("remove: got %d %v", n, err)
	}

	reloaded := NewStorage(path)
	if err := reloaded.Load(); err != nil {
		t.Fatal(err)
	}
	if got := reloaded.Read(); len(got) != 1 || got[0].Name != "Renamed" {
		t.Errorf("after reload got %v", got)
	}
}

func TestStorageLoad_CorruptFile(t *testing.T) {
	path := t.TempDir() + "/data.db"
	os.WriteFile(path, []byte("{not json"), 0644)

	if err := NewStorage(path).Load(); err == nil {
		t.Error("wanted error for corrupt data file")
	}
}
