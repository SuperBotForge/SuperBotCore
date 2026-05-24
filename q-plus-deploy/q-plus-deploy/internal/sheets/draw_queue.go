package sheets

import (
	"context"
	"github.com/rs/zerolog"
	"github.com/samber/lo"
	"google.golang.org/api/sheets/v4"
	"q+/internal/core"
	"q+/internal/generated/ent"
	"q+/internal/utils"
	"strings"
)

var titleColor = &sheets.Color{ // red
	Red:   244 / 255.0,
	Green: 204 / 255.0,
	Blue:  204 / 255.0,
	Alpha: 0,
}

var doneColor = &sheets.Color{ // green
	Red:   217 / 255.0,
	Green: 234 / 255.0,
	Blue:  211 / 255.0,
	Alpha: 0,
}
var busyColor = &sheets.Color{ // yellow
	Red:   255 / 255.0,
	Green: 242 / 255.0,
	Blue:  204 / 255.0,
	Alpha: 0,
}

var transparentColor = &sheets.Color{ // transparent white
	Red:   1,
	Green: 1,
	Blue:  1,
	Alpha: 1,
}

var mainColumnsWidth = int64(180)
var criteriaColumnsWidth = int64(130)
var spaceColumnsWidth = int64(70)

func (p *Presenter) RedrawQueue(ctx context.Context, spreadsheetId string, sheetId int64, queue *ent.Queue) error {
	logger := zerolog.Ctx(ctx)

	requests := createRedrawQueueRequests(sheetId, queue)

	_, err := p.SpreadsheetsService.
		BatchUpdate(spreadsheetId, &sheets.BatchUpdateSpreadsheetRequest{Requests: requests}).Context(ctx).Do()
	if err != nil {
		return err
	}
	logger.Info().
		Int64("queue_id", queue.ID).
		Str("spreadsheet_id", spreadsheetId).
		Int64("sheet_id", sheetId).
		Msg("Queue table has been redrawn")
	return nil
}

func createRedrawQueueRequests(sheetId int64, queue *ent.Queue) []*sheets.Request {
	clearRequest := createClearRequest(sheetId)
	requests := []*sheets.Request{clearRequest}

	requests = append(requests, createSetColumnCountRequest(sheetId, int64(6+len(queue.Edges.Examiners)*3)))

	requests = append(requests, createResizeColumnRequest(sheetId, 0, int64(6+len(queue.Edges.Examiners)*3), spaceColumnsWidth))

	requests = append(requests, createDrawQueueRequests(sheetId, []string{queue.Name}, queue.Edges.Places, nil, true, 0)...)

	for i, examiner := range queue.Edges.Examiners {
		teacherCriteriaMap := make(map[int64]bool)
		for _, criterion := range examiner.Edges.Criteria {
			teacherCriteriaMap[criterion.ID] = true
		}

		places := make([]*ent.QueuePlace, 0)
		for _, place := range queue.Edges.Places {
			for _, criterion := range place.Edges.QueuePlaceCriteria {
				if !criterion.Passed {
					if teacherCriteriaMap[criterion.CriterionID] {
						places = append(places, place)
						break
					}
				}
			}
		}
		note := ""
		if len(examiner.Note) > 0 {
			note = " (" + examiner.Note + ")"
		}
		requests = append(requests, createDrawQueueRequests(sheetId, []string{core.GetName(examiner.Edges.Teacher) + note, utils.JoinCriteria(examiner.Edges.Criteria)}, places, examiner.Edges.CurrentQueuePlace, true, int64(3+i*3))...)
	}
	return requests
}

func createDrawQueueRequests(sheetId int64, title []string, places []*ent.QueuePlace, currentPlace *ent.QueuePlace, isDrawCriteria bool, columnIndex int64) []*sheets.Request {
	values := make([][]string, len(places)+1)
	values[0] = title
	for i, place := range places {
		team := strings.Join(lo.Map(place.Edges.Team, func(u *ent.User, _ int) string { return core.GetName(u) }), ", ")
		if currentPlace != nil && currentPlace.ID == place.ID {
			team = "👉 " + team
		}

		if isDrawCriteria {
			notPassedCriteria := lo.FilterMap(place.Edges.QueuePlaceCriteria, func(c *ent.QueuePlaceCriterion, _ int) (*ent.Criterion, bool) {
				return c.Edges.Criterion, !c.Passed
			})
			values[i+1] = []string{team, utils.JoinCriteria(notPassedCriteria)}
		} else {
			values[i+1] = []string{team}
		}
	}

	requests := make([]*sheets.Request, 0, len(places)+2) // each place color + title color + values

	requests = append(requests, createSetValuesToRangeRequest(sheetId, values, columnIndex, 0, 2, -1))
	requests = append(requests, createQueueColumnColorRequests(sheetId, places, columnIndex)...)
	requests = append(requests, createResizeColumnRequest(sheetId, columnIndex, columnIndex+1, mainColumnsWidth))
	if isDrawCriteria {
		requests = append(requests, createResizeColumnRequest(sheetId, columnIndex+1, columnIndex+2, criteriaColumnsWidth))
	}

	return requests
}

func createQueueColumnColorRequests(sheetId int64, places []*ent.QueuePlace, columnIndex int64) []*sheets.Request {

	requests := make([]*sheets.Request, len(places)+1) // each place color + title color

	requests[0] = createChangeColorRequest(sheetId, 0, columnIndex, titleColor)
	for i, place := range places {
		requests[i+1] = createChangeColorRequest(sheetId, int64(i+1), columnIndex, getPlaceColor(place))
	}

	return requests
}

func getPlaceColor(place *ent.QueuePlace) *sheets.Color {
	notPassedCriteria := lo.Filter(place.Edges.QueuePlaceCriteria, func(c *ent.QueuePlaceCriterion, _ int) bool { return !c.Passed })
	if len(notPassedCriteria) == 0 {
		return doneColor
	}
	busy := false
	for _, u := range place.Edges.Team {
		if u.IsBusy {
			busy = true
			break
		}
	}
	if busy || len(place.Edges.CurrentExaminer) > 0 {
		return busyColor
	}
	return transparentColor
}

func createChangeColorRequest(sheetId int64, row int64, col int64, color *sheets.Color) *sheets.Request {
	return &sheets.Request{
		RepeatCell: &sheets.RepeatCellRequest{
			Cell: &sheets.CellData{
				UserEnteredFormat: &sheets.CellFormat{
					BackgroundColor: color,
				},
			},
			Fields: "userEnteredFormat.backgroundColor",
			Range: &sheets.GridRange{
				StartColumnIndex: col,
				StartRowIndex:    row,
				EndColumnIndex:   col + 1,
				EndRowIndex:      row + 1,
				SheetId:          sheetId,
			},
		},
	}
}
