package state

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"SuperBotGo/internal/model"
)

const dialogBackPrefix = "__dialog_back:"

func appendBackButton(message model.Message, option model.Option) model.Message {
	// Renderers use the last options block. Extend it instead of replacing
	// the step choices with a separate navigation keyboard.
	message.Blocks = append([]model.ContentBlock(nil), message.Blocks...)
	for i := len(message.Blocks) - 1; i >= 0; i-- {
		if block, ok := message.Blocks[i].(model.OptionsBlock); ok {
			// Keep navigation visible even when a channel truncates a long list.
			if len(block.Options) >= 10 {
				block.Options = append([]model.Option{option}, block.Options...)
			} else {
				block.Options = append(append([]model.Option(nil), block.Options...), option)
			}
			message.Blocks[i] = block
			return message
		}
	}
	message.Blocks = append(message.Blocks, model.OptionsBlock{Options: []model.Option{option}})
	return message
}

func newNavigationToken() string {
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(token[:])
}

func restoredNavigationToken(token string) string {
	if token == "" {
		return newNavigationToken()
	}
	return token
}

func copyHistory(history []model.DialogSnapshot) []model.DialogSnapshot {
	result := make([]model.DialogSnapshot, len(history))
	for i, snapshot := range history {
		result[i] = model.DialogSnapshot{
			Params: copyOptionMap(snapshot.Params), PageState: copyPageState(snapshot.PageState),
		}
	}
	return result
}

func (h *DslStateHandler) navigateBack(ctx context.Context, userID model.GlobalUserID, ds *DslState, value, locale string) (State, StepOutcome, error) {
	outcome := StepOutcome{CommandName: h.command.Name}
	// Old buttons must not rewind a different step or become a text answer.
	if !h.command.AllowBack || value != dialogBackPrefix+ds.NavigationToken {
		outcome.Message = h.BuildStepMessage(ctx, userID, ds, locale)
		return ds, outcome, nil
	}
	if len(ds.History) == 0 {
		outcome.IsCancelled = true
		return ds, outcome, nil
	}
	last := ds.History[len(ds.History)-1]
	ds.Params = copyOptionMap(last.Params)
	ds.PageState = copyPageState(last.PageState)
	ds.History = ds.History[:len(ds.History)-1]
	ds.NavigationToken = newNavigationToken()
	outcome.Message = h.BuildStepMessage(ctx, userID, ds, locale)
	return ds, outcome, nil
}
