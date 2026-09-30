package adapter

import (
	"SuperBotGo/internal/state"
	wasmrt "SuperBotGo/internal/wasm/runtime"
	"context"
	"log/slog"
)

// Resolve on every render, including after Back. Do not mutate saved metadata.
func resolvePromptBlock(block wasmrt.BlockDef, ctx state.StepContext, call func(context.Context, string, wasmrt.StepCallbackRequest) (*wasmrt.StepCallbackResponse, error)) wasmrt.BlockDef {
	if block.PromptsFn == "" {
		return block
	}
	resp, err := call(ctx.Context, block.PromptsFn, wasmrt.StepCallbackRequest{UserID: int64(ctx.UserID), Locale: ctx.Locale, Params: ctx.Params})
	if err != nil || resp == nil || resp.Error != "" || len(resp.Prompts) == 0 {
		slog.Error("wasm prompt callback failed", "callback", block.PromptsFn, "error", err)
		return block
	}
	// The usual Core locale resolver selects the translated value.
	if block.Type == "text" {
		block.Texts = resp.Prompts
	} else {
		block.Prompts = resp.Prompts
	}
	return block
}
