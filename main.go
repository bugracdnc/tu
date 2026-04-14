package main

import (
	"time"
	"tu/cli"
	"tu/db"
	"tu/models"
)

func addNewDates(db db.Storage) {
	tudate1 := models.NewTuDateFromDate(time.Now())
	db.Add(*tudate1)
	tudate2 := models.NewTuDateFromDate(time.Now())
	db.Add(*tudate2)
}

func main() {
	jsonFileDirPath := "~/.config/tu/"
	jsonFilePath := jsonFileDirPath + "data.db"
	TuDateStorage := db.NewStorage(jsonFilePath)
	err := TuDateStorage.Load()
	if err != nil {
		panic("Error: Cannot read storage: " + err.Error())
	}

	//addNewDates(*TuDateStorage)

	cli.Execute(TuDateStorage)
}
