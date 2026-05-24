package sheets

import (
	"github.com/samber/lo"
	"google.golang.org/api/sheets/v4"
)

func createClearRequest(sheetId int64) *sheets.Request {
	return &sheets.Request{
		UpdateCells: &sheets.UpdateCellsRequest{
			Fields: "*",
			Range: &sheets.GridRange{
				SheetId:          sheetId,
				StartColumnIndex: 0,
				StartRowIndex:    0,
			},
		},
	}
}
func createSetValuesToRangeRequest(sheetId int64, values [][]string, startColumn int64, startRow int64, width int64, height int64) *sheets.Request {
	var endColumn = int64(0)
	var endRow = int64(0)
	if width >= 0 {
		endColumn = startColumn + width
	}
	if height >= 0 {
		endRow = startRow + height
	}
	return &sheets.Request{
		UpdateCells: &sheets.UpdateCellsRequest{
			Fields: "userEnteredValue",
			Range: &sheets.GridRange{
				SheetId:          sheetId,
				StartColumnIndex: startColumn,
				StartRowIndex:    startRow,
				EndColumnIndex:   endColumn,
				EndRowIndex:      endRow,
			},
			Rows: lo.Map(values, func(row []string, _ int) *sheets.RowData {
				return &sheets.RowData{
					Values: lo.Map(row, func(v string, _ int) *sheets.CellData {
						return &sheets.CellData{
							UserEnteredValue: &sheets.ExtendedValue{
								StringValue: &v,
							},
						}
					}),
				}
			}),
		},
	}
}

func createSetColumnCountRequest(sheetId int64, count int64) *sheets.Request {
	return &sheets.Request{
		UpdateSheetProperties: &sheets.UpdateSheetPropertiesRequest{
			Fields: "gridProperties.columnCount",
			Properties: &sheets.SheetProperties{
				SheetId: sheetId,
				GridProperties: &sheets.GridProperties{
					ColumnCount: count,
				},
			},
		},
	}
}

func createAutoresizeColumnRequest(sheetId int64, startIndex int64, endIndex int64) *sheets.Request {
	return &sheets.Request{
		AutoResizeDimensions: &sheets.AutoResizeDimensionsRequest{
			Dimensions: &sheets.DimensionRange{
				SheetId:    sheetId,
				Dimension:  "COLUMNS",
				StartIndex: startIndex,
				EndIndex:   endIndex,
			},
		},
	}
}

func createResizeColumnRequest(sheetId int64, startIndex int64, endIndex int64, width int64) *sheets.Request {
	return &sheets.Request{
		UpdateDimensionProperties: &sheets.UpdateDimensionPropertiesRequest{
			Properties: &sheets.DimensionProperties{
				PixelSize: width,
			},
			Range: &sheets.DimensionRange{
				SheetId:    sheetId,
				Dimension:  "COLUMNS",
				StartIndex: startIndex,
				EndIndex:   endIndex,
			},
			Fields: "pixelSize",
		},
	}
}
