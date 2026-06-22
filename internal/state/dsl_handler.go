package state

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"SuperBotGo/internal/i18n"
	"SuperBotGo/internal/locale"
	"SuperBotGo/internal/model"
)

// resolveLocalizedPrompt picks the best text for the given locale from a
// locale→text map, falling back to fallback if no match is found.
func resolveLocalizedPrompt(texts map[string]string, fallback, loc string) string {
	if text, ok := texts[loc]; ok {
		return text
	}
	if idx := strings.IndexByte(loc, '-'); idx > 0 {
		if text, ok := texts[loc[:idx]]; ok {
			return text
		}
	}
	if text, ok := texts[locale.Default()]; ok {
		return text
	}
	return fallback
}

const (
	PageNext = "__page_next"
	PagePrev = "__page_prev"
)

type DslState struct {
	Command   *CommandDefinition
	Params    model.OptionMap
	PageState map[string]int
}

func (s *DslState) IsComplete() bool {
	return s.Command.IsComplete(StepContext{Params: s.Params})
}

func (s *DslState) FinalParams() model.OptionMap {
	return copyOptionMap(s.Params)
}

func copyOptionMap(src model.OptionMap) model.OptionMap {
	dst := make(model.OptionMap, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func copyPageState(src map[string]int) map[string]int {
	dst := make(map[string]int, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

type DslStateHandler struct {
	command *CommandDefinition
}

func NewDslStateHandler(command *CommandDefinition) *DslStateHandler {
	return &DslStateHandler{command: command}
}

func (h *DslStateHandler) CreateNewState(_ string) (State, error) {
	return &DslState{
		Command:   h.command,
		Params:    make(model.OptionMap),
		PageState: make(map[string]int),
	}, nil
}

func (h *DslStateHandler) RestoreState(ds model.DialogState) (State, error) {
	return &DslState{
		Command:   h.command,
		Params:    copyOptionMap(ds.Params),
		PageState: copyPageState(ds.PageState),
	}, nil
}

func (h *DslStateHandler) PersistState(s State) model.DialogState {
	ds, ok := s.(*DslState)
	if !ok {
		slog.Error("PersistState: expected *DslState", "got", fmt.Sprintf("%T", s))
		return model.DialogState{CommandName: h.command.Name}
	}
	return model.DialogState{
		CommandName: h.command.Name,
		Params:      copyOptionMap(ds.Params),
		PageState:   copyPageState(ds.PageState),
	}
}

func (h *DslStateHandler) ProcessInput(ctx context.Context, userID model.GlobalUserID, s State, input model.UserInput, locale string) (State, StepOutcome, error) {
	ds, ok := s.(*DslState)
	if !ok {
		return nil, StepOutcome{}, fmt.Errorf("expected *DslState but got %T", s)
	}
	stepCtx := StepContext{
		Context: ctx,
		UserID:  userID,
		Locale:  locale,
		Params:  ds.Params,
	}
	step := h.command.CurrentStep(stepCtx)
	if step == nil {

		return ds, StepOutcome{
			CommandName: h.command.Name,
			IsComplete:  true,
			Params:      ds.FinalParams(),
		}, nil
	}

	if step.Pagination != nil {
		if _, isCallback := input.(model.CallbackInput); isCallback {
			textVal := input.TextValue()
			switch textVal {
			case PageNext:
				cur := ds.PageState[step.ParamName]
				ds.PageState[step.ParamName] = cur + 1
				msg := h.BuildStepMessage(ctx, userID, ds, locale)
				return ds, StepOutcome{
					Message:     msg,
					CommandName: h.command.Name,
					IsComplete:  false,
				}, nil
			case PagePrev:
				cur := ds.PageState[step.ParamName]
				if cur > 0 {
					ds.PageState[step.ParamName] = cur - 1
				}
				msg := h.BuildStepMessage(ctx, userID, ds, locale)
				return ds, StepOutcome{
					Message:     msg,
					CommandName: h.command.Name,
					IsComplete:  false,
				}, nil
			}
		}
	}

	isValid := true
	switch {
	case step.ValidateWithContext != nil:
		isValid = step.ValidateWithContext(stepCtx, input)
	case step.Validate != nil:
		isValid = step.Validate(input)
	}

	if isValid {
		ds.Params[step.ParamName] = input.TextValue()
	}

	msg := h.BuildStepMessage(ctx, userID, ds, locale)
	complete := h.command.IsComplete(StepContext{
		Context: ctx,
		UserID:  userID,
		Locale:  locale,
		Params:  ds.Params,
	})

	outcome := StepOutcome{
		Message:     msg,
		CommandName: h.command.Name,
		IsComplete:  complete,
	}
	if complete {
		outcome.Params = ds.FinalParams()
	}

	return ds, outcome, nil
}

func (h *DslStateHandler) BuildStepMessage(ctx context.Context, userID model.GlobalUserID, s State, locale string) model.Message {
	ds, ok := s.(*DslState)
	if !ok {
		slog.Error("BuildStepMessage: expected *DslState", "got", fmt.Sprintf("%T", s))
		return model.Message{}
	}
	stepCtx := StepContext{
		Context: ctx,
		UserID:  userID,
		Locale:  locale,
		Params:  ds.Params,
	}
	step := h.command.CurrentStep(stepCtx)
	if step == nil {
		return model.Message{}
	}

	message := step.MessageBuilder(stepCtx)

	if step.Pagination != nil {
		return h.applyPagination(message, step, ds, stepCtx)
	}

	return message
}

func (h *DslStateHandler) applyPagination(message model.Message, step *StepNode, ds *DslState, baseCtx StepContext) model.Message {
	config := step.Pagination
	currentPage := ds.PageState[step.ParamName]
	pageCtx := StepContext{
		Context: baseCtx.Context,
		UserID:  baseCtx.UserID,
		Locale:  baseCtx.Locale,
		Params:  ds.Params,
		Page:    currentPage,
	}
	result := config.PageProvider(pageCtx, currentPage)
	if result.Error != "" {
		return appendMessageText(message, paginationErrorText(baseCtx.Locale, result.Error))
	}

	var navOptions []model.Option
	if currentPage > 0 {
		navOptions = append(navOptions, model.Option{Label: i18n.Get("pagination.previous", baseCtx.Locale), Value: PagePrev})
	}
	if result.HasMore {
		navOptions = append(navOptions, model.Option{Label: i18n.Get("pagination.next", baseCtx.Locale), Value: PageNext})
	}

	allOptions := make([]model.Option, 0, len(result.Options)+len(navOptions))
	allOptions = append(allOptions, result.Options...)
	allOptions = append(allOptions, navOptions...)

	prompt := config.Prompt
	if config.PromptProvider != nil {
		prompt = config.PromptProvider(baseCtx)
	} else if len(config.Prompts) > 0 {
		prompt = resolveLocalizedPrompt(config.Prompts, config.Prompt, baseCtx.Locale)
	}

	paginatedBlock := model.OptionsBlock{
		Prompt:  prompt,
		Options: allOptions,
	}

	blocks := make([]model.ContentBlock, 0, len(message.Blocks)+1)
	blocks = append(blocks, message.Blocks...)
	blocks = append(blocks, paginatedBlock)

	return model.Message{Blocks: blocks}
}

func appendMessageText(message model.Message, text string) model.Message {
	if text == "" {
		return message
	}
	blocks := make([]model.ContentBlock, 0, len(message.Blocks)+1)
	blocks = append(blocks, message.Blocks...)
	blocks = append(blocks, model.TextBlock{Text: text})
	return model.Message{Blocks: blocks}
}

func paginationErrorText(locale, errText string) string {
	if errText != "" {
		return errText
	}
	if strings.HasPrefix(locale, "ru") {
		return "Не удалось загрузить варианты. Попробуйте ещё раз."
	}
	return "Failed to load options. Please try again."
}

var _ StateHandler = (*DslStateHandler)(nil)

var ErrCommandNotFound = errors.New("command not found")

var ErrNoActiveDialog = errors.New("no active dialog")
