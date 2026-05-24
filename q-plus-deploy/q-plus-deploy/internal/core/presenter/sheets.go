package presenter

import (
	"context"
	"q+/internal/generated/ent"
)

type SheetsService interface {
	// CreateSpreadsheet creates a new spreadsheet with the given name.
	CreateSpreadsheet(ctx context.Context, name string) (spreadsheetId string, sheetId int64, err error)

	// GetSpreadsheetTitle returns the title of the spreadsheet with the given ID.
	GetSpreadsheetTitle(ctx context.Context, spreadsheetId string) (title string, err error)

	// CreateSheet creates a new sheet with the given name in the spreadsheet with the given ID.
	CreateSheet(ctx context.Context, spreadsheetId string, name string) (sheetId int64, err error)

	// RedrawQueue redraws the queue in the spreadsheet. Requires the queue to be eager loaded with all the necessary data.
	RedrawQueue(ctx context.Context, spreadsheetId string, sheetId int64, queue *ent.Queue) error

	// RedrawMarkTableTab redraws the mark table tab in the spreadsheet. Requires the tab to be eager loaded with all the necessary data.
	RedrawMarkTableTab(
		ctx context.Context,
		spreadsheetId string,
		sheetId int64,
		tab *ent.MarkTableTab, // TODO rename sheet according to the tab name
		criteria []*ent.Criterion,
		students []*ent.User,
		marks []*ent.Mark,
	) error
}
