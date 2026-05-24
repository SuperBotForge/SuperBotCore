package handlers

import (
	"fmt"
	"q+/internal/core"
	"q+/internal/utils"
)

func SetMark(ctx InteractionContext, options OptionMap) error {
	ctx.log().Trace().Msg("Set mark command")

	setMarkResponse, err := core.SetMark(useCaseContext(ctx, core.SetMarkParams{
		ServerCommandParams: ctx.serverCommandParams(),
		CriterionId:         options.Int("criterion"),
		Mark:                options.String("mark"),
		Teacher:             getUser(ctx.I),
	}))
	if err != nil {
		return err
	}

	resp := fmt.Sprintf("%s поставлена оценка `%s` за критерий %s",
		utils.JoinUserPings(setMarkResponse.Team),
		setMarkResponse.Mark,
		setMarkResponse.Criterion.Name,
	)
	return ctx.interactionCommandRespond(resp)
}
