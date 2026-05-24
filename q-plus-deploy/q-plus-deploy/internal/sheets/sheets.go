package sheets

import (
	"context"
	"github.com/rs/zerolog"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/sheets/v4"
)

func (p *Presenter) CreateSpreadsheet(ctx context.Context, name string) (spreadsheetId string, sheetId int64, err error) {
	logger := zerolog.Ctx(ctx)
	request := p.SpreadsheetsService.Create(&sheets.Spreadsheet{
		Properties: &sheets.SpreadsheetProperties{
			Title: name,
		},
	})
	request.Fields("spreadsheetId")
	spreadsheet, err := request.Context(ctx).Do()
	if err != nil {
		return "", -1, err
	}
	spreadsheetId = spreadsheet.SpreadsheetId
	sheetId = 0
	permission := &drive.Permission{
		Role: "writer",
		Type: "anyone",
	}
	_, err = p.DrivesService.Permissions.Create(spreadsheet.SpreadsheetId, permission).Context(ctx).Do()
	if err == nil {
		logger.Info().
			Str("event", "spreadsheet_created").
			Str("spreadsheet_id", spreadsheetId).
			Msg("Spreadsheet created")
	}
	return
}

func (p *Presenter) GetSpreadsheetTitle(ctx context.Context, spreadsheetId string) (string, error) {
	spreadsheet, err := p.SpreadsheetsService.Get(spreadsheetId).Context(ctx).Do()
	if err != nil {
		return "", err
	}
	return spreadsheet.Properties.Title, nil
}

func (p *Presenter) CreateSheet(ctx context.Context, spreadsheetId string, name string) (int64, error) {
	requests := []*sheets.Request{{AddSheet: &sheets.AddSheetRequest{
		Properties: &sheets.SheetProperties{
			Title: name,
		},
	}}}

	resp, err := p.SpreadsheetsService.
		BatchUpdate(spreadsheetId, &sheets.BatchUpdateSpreadsheetRequest{Requests: requests}).Context(ctx).Do()
	if err != nil {
		return -1, err
	}
	return resp.Replies[0].AddSheet.Properties.SheetId, nil
}
