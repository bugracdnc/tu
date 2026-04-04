package db

import (
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
