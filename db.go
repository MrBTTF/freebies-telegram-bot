package main

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"google.golang.org/api/sheets/v4"
)

const (
	SPREADSHEET_ID = "11ZWtDmKXFODksqeD61gaqYa9J6x2_WtBqs3iRR56jVs"
)


type Subscriber struct {
	ChatID   int64
	LastPost time.Time
}

type Storage struct {
	client *sheets.Service
}

func NewStorage(sheetClient *sheets.Service) *Storage {
	return &Storage{
		client: sheetClient,
	}
}

func (s *Storage) PersistSubscriber(index int, chatId int64, sinceTime time.Time) error {
	var vr sheets.ValueRange
	vr.MajorDimension = "COLUMNS"
	chatIdVal := []interface{}{chatId}

	sinceTimeStr := sinceTime.Format(time.RFC3339)
	sinceTimeVal := []interface{}{sinceTimeStr}

	vr.Values = append(vr.Values, chatIdVal, sinceTimeVal)

	var err error
	if index != -1 {
		readRange := fmt.Sprintf("Subscribers!A%d", index+1)
		_, err = s.client.Spreadsheets.Values.
			Update(SPREADSHEET_ID, readRange, &vr).
			ValueInputOption("RAW").Do()
	} else {
		readRange := "Subscribers"
		_, err = s.client.Spreadsheets.Values.
			Append(SPREADSHEET_ID, readRange, &vr).
			ValueInputOption("RAW").Do()
	}
	return err
}

func (s *Storage) DeleteSubscriber(index int) error {
	_, err := s.client.Spreadsheets.BatchUpdate(
		SPREADSHEET_ID,
		&sheets.BatchUpdateSpreadsheetRequest{
			Requests: []*sheets.Request{
				{
					DeleteDimension: &sheets.DeleteDimensionRequest{
						Range: &sheets.DimensionRange{
							Dimension:  "ROWS",
							StartIndex: int64(index),
							EndIndex:   int64(index + 1),
						},
					},
				},
			},
		},
	).Do()
	return err
}

func (s *Storage) ClearSubscribers() error {
	readRange := "Subscribers!A:B"
	_, err := s.client.Spreadsheets.Values.Clear(SPREADSHEET_ID, readRange, &sheets.ClearValuesRequest{}).Do()
	return err
}

func (s *Storage) ReadSubscribers() ([]Subscriber, error) {
	readRange := "Subscribers!A:B"
	resp, err := s.client.Spreadsheets.Values.Get(SPREADSHEET_ID, readRange).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve data from sheet: %v", err)
	}

	lines := []Subscriber{}
	for _, row := range resp.Values {
		if len(row) < 2 {
			continue
		}
		dateStr, ok := row[1].(string)
		if !ok {
			continue
		}
		date, err := time.Parse(time.RFC3339, dateStr)

		if dateStr == "" {
			date = time.Now()
		} else if err != nil {
			log.Println(err)
			return nil, err
		}

		chatID, err := strconv.Atoi(row[0].(string))
		if err != nil {
			return nil, err
		}

		lines = append(lines, Subscriber{
			int64(chatID),
			date,
		})
	}
	return lines, nil
}

func subscriberExist(chatID int64, subscribers []Subscriber) bool {
	for _, s := range subscribers {
		if s.ChatID == chatID {
			return true
		}
	}
	return false
}

func (s *Storage) WriteSubscribers(subscribers []Subscriber) error {
	for i, sub := range subscribers {
		err := s.PersistSubscriber(i, sub.ChatID, sub.LastPost)
		if err != nil {
			return err
		}
	}
	return nil
}
