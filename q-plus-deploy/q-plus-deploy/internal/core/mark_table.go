package core

import (
	"q+/internal/generated/ent"
	"q+/internal/generated/ent/marktable"
)

type CreateMarkTableTabParams struct {
	MarkTableId int64
	Name        string
}

var CreateMarkTableTab = wrapTx(createMarkTableTab)

func createMarkTableTab(ctx UseCaseContext[CreateMarkTableTabParams]) (*ent.MarkTableTab, error) {
	markTable, err := ctx.ent().MarkTable.
		Query().
		Where(marktable.ID(ctx.Params.MarkTableId)).
		Only(ctx.Ctx)
	if err != nil {
		return nil, err
	}

	sheetId, err := ctx.Core.sheetsService.CreateSheet(ctx.Ctx, markTable.SpreadsheetID, ctx.Params.Name)
	if err != nil {
		return nil, err
	}

	markTableTab, err := ctx.ent().MarkTableTab.
		Create().
		SetName(ctx.Params.Name).
		SetMarkTableID(ctx.Params.MarkTableId).
		SetSheetID(sheetId).
		Save(ctx.Ctx)

	return markTableTab, err
}
