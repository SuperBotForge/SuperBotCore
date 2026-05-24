package sheets

import (
	"context"
	"github.com/samber/do/v2"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

const (
	clientSecretPath = "google_credentials.json"
)

type Presenter struct {
	SpreadsheetsService *sheets.SpreadsheetsService
	DrivesService       *drive.Service
}

func NewSheetsPresenter(_ do.Injector) (*Presenter, error) {
	ctx := context.Background()
	sheetsService, err := sheets.NewService(ctx, option.WithCredentialsFile(clientSecretPath), option.WithScopes(sheets.SpreadsheetsScope, sheets.DriveScope))
	if err != nil {
		return nil, err
	}
	driveService, err := drive.NewService(ctx, option.WithCredentialsFile(clientSecretPath), option.WithScopes(drive.DriveScope))
	if err != nil {
		return nil, err
	}
	return &Presenter{
		SpreadsheetsService: sheetsService.Spreadsheets,
		DrivesService:       driveService,
	}, nil
}

//func Test() {
//	spreadsheetId := "1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms"
//	readRange := "Class Data!A2:E"
//	resp, err := SpreadsheetsService.Values.Get(spreadsheetId, readRange).Do()
//	if err != nil {
//		//logrus.Fatalf("Unable to retrieve data from sheet: %v", err)
//	}
//
//	if len(resp.Values) == 0 {
//		//logrus.Println("No data found.")
//	} else {
//		//logrus.Println("Name, Major:")
//		for _, row := range resp.Values {
//			// Print columns A and E, which correspond to indices 0 and 4.
//			//logrus.Printf("%s, %s\n", row[0], row[4])
//		}
//	}
//}
