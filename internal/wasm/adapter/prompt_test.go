package adapter

import (
	"SuperBotGo/internal/model"
	"SuperBotGo/internal/state"
	wasmrt "SuperBotGo/internal/wasm/runtime"
	"context"
	"errors"
	"testing"
)

func TestPromptRender(t *testing.T) {
	for _, kind := range []string{"text", "options", "dynamic_options"} {
		block := wasmrt.BlockDef{Type: kind, PromptsFn: "prompt", Text: "fallback", Prompt: "fallback", Texts: map[string]string{"ru": "static"}, Prompts: map[string]string{"ru": "static"}}
		calls := 0
		callback := func(_ context.Context, name string, r wasmrt.StepCallbackRequest) (*wasmrt.StepCallbackResponse, error) {
			calls++
			if name != "prompt" || r.UserID != 7 {
				t.Fatal("callback context missing")
			}
			return &wasmrt.StepCallbackResponse{Prompts: map[string]string{"ru": "ru:" + r.Params["semester"], "en": "en:" + r.Params["semester"]}}, nil
		}
		for _, semester := range []string{"5", "6", "5"} {
			ctx := state.StepContext{UserID: 7, Locale: "ru", Params: model.OptionMap{"semester": semester}}
			got := resolvePromptBlock(block, ctx, callback)
			if kind == "text" {
				if resolveLocalized(got.Text, got.Texts, ctx.Locale) != "ru:"+semester {
					t.Fatal(got)
				}
			} else if resolveLocalized(got.Prompt, got.Prompts, ctx.Locale) != "ru:"+semester {
				t.Fatal(got)
			}
		}
		if calls != 3 || block.Texts["ru"] != "static" {
			t.Fatal("render cached or mutated metadata")
		}
		for _, resp := range []*wasmrt.StepCallbackResponse{nil, {}, {Error: "failed"}} {
			got := resolvePromptBlock(block, state.StepContext{}, func(context.Context, string, wasmrt.StepCallbackRequest) (*wasmrt.StepCallbackResponse, error) {
				if resp == nil {
					return nil, errors.New("failure")
				}
				return resp, nil
			})
			if got.Prompts["ru"] != "static" {
				t.Fatal("fallback lost")
			}
		}
		for _, loc := range []string{"ru", "en", "en-US"} {
			got := resolvePromptBlock(block, state.StepContext{UserID: 7, Locale: loc, Params: model.OptionMap{"semester": "5"}}, callback)
			values := got.Prompts
			if kind == "text" {
				values = got.Texts
			}
			want := "en:5"
			if loc == "ru" {
				want = "ru:5"
			}
			if ResolveLocalizedText(values, loc) != want {
				t.Fatal("incorrect locale selection")
			}
		}
		block.PromptsFn = ""
		resolvePromptBlock(block, state.StepContext{}, func(context.Context, string, wasmrt.StepCallbackRequest) (*wasmrt.StepCallbackResponse, error) {
			t.Fatal("static block called WASM")
			return nil, nil
		})
	}
}
