package state

import (
	"context"

	"SuperBotGo/internal/model"
)

type StepOutcome struct {
	Message     model.Message
	CommandName string
	IsComplete  bool
	IsCancelled bool
	Params      model.OptionMap
}

type State interface {
	IsComplete() bool
	FinalParams() model.OptionMap
}

type StateHandler interface {
	CreateNewState(commandName string) (State, error)

	RestoreState(ds model.DialogState) (State, error)

	PersistState(s State) model.DialogState

	ProcessInput(ctx context.Context, userID model.GlobalUserID, s State, input model.UserInput, locale string) (State, StepOutcome, error)

	BuildStepMessage(ctx context.Context, userID model.GlobalUserID, s State, locale string) model.Message
}
