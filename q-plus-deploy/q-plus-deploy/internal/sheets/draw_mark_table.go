package sheets

import (
	"context"
	"github.com/rs/zerolog"
	"google.golang.org/api/sheets/v4"
	"q+/internal/core"
	"q+/internal/generated/ent"
	"sort"
)

func (p *Presenter) RedrawMarkTableTab(
	ctx context.Context,
	spreadsheetId string,
	sheetId int64,
	tab *ent.MarkTableTab, // TODO rename sheet according to the tab name
	criteria []*ent.Criterion,
	students []*ent.User,
	marks []*ent.Mark,
) error {
	logger := zerolog.Ctx(ctx)

	requests := createRedrawMarkTableRequests(sheetId, tab, criteria, students, marks)
	_, err := p.SpreadsheetsService.BatchUpdate(spreadsheetId, &sheets.BatchUpdateSpreadsheetRequest{Requests: requests}).
		Context(ctx).Do()
	if err != nil {
		return err
	}

	logger.Info().
		Int64("mark_table_tab_id", tab.ID).
		Str("spreadsheet_id", spreadsheetId).
		Int64("sheet_id", sheetId).
		Msg("Mark table tab has been redrawn")
	return nil
}

func createRedrawMarkTableRequests(
	sheetId int64,
	tab *ent.MarkTableTab, // TODO rename sheet according to the tab name
	criteria []*ent.Criterion,
	students []*ent.User,
	marks []*ent.Mark,
) []*sheets.Request {
	values := make([][]string, 1+len(students))
	firstRow := make([]string, 1+len(criteria))

	sort.Slice(criteria, func(i, j int) bool {
		return criteria[i].Name < criteria[j].Name
	})
	criteriaIndex := make(map[int64]int, len(criteria)) // criterionId -> columnIndex
	for i, criterion := range criteria {
		firstRow[i+1] = criterion.Name
		criteriaIndex[criterion.ID] = i + 1
	}

	sort.Slice(students, func(i, j int) bool {
		return core.GetName(students[i]) < core.GetName(students[j])
	})
	studentIndex := make(map[int64]int, len(students)) // studentId -> rowIndex
	for i, student := range students {
		studentIndex[student.ID] = i + 1
	}

	values[0] = firstRow
	for i, student := range students {
		row := make([]string, 1+len(criteria))
		row[0] = core.GetName(student)
		values[i+1] = row
	}

	for _, mark := range marks {
		rowIndex, ok := studentIndex[mark.UserID]
		if !ok {
			continue
		}
		columnIndex, ok := criteriaIndex[mark.CriterionID]
		if !ok {
			continue
		}
		values[rowIndex][columnIndex] = mark.Value
	}

	requests := []*sheets.Request{
		createClearRequest(sheetId),
		createSetValuesToRangeRequest(sheetId, values, 0, 0, -1, -1),
	}
	return requests
}
