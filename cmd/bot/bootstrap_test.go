package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"testing"

	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/model"
)

type testBootstrapAdapter struct{}

func (a *testBootstrapAdapter) Type() model.ChannelType { return model.ChannelTelegram }
func (a *testBootstrapAdapter) SendToUser(context.Context, model.PlatformUserID, model.Message) error {
	return nil
}
func (a *testBootstrapAdapter) SendToChat(context.Context, string, model.Message) error {
	return nil
}

type testBootstrapBot struct {
	commands          []string
	routesRegistered  bool
	registerRoutesErr error
	startCalls        int
}

func (b *testBootstrapBot) RegisterCommands(commands []string) {
	b.commands = append([]string(nil), commands...)
}

func (b *testBootstrapBot) RegisterRoutes(mux *http.ServeMux) error {
	if b.registerRoutesErr != nil {
		return b.registerRoutesErr
	}
	b.routesRegistered = true
	mux.HandleFunc("GET /healthz", func(http.ResponseWriter, *http.Request) {})
	return nil
}

func (b *testBootstrapBot) Start(context.Context) error {
	b.startCalls++
	return nil
}

func TestRegisterPreparedBot_RegistersFeaturesAdapterAndStarter(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()
	bot := &testBootstrapBot{}
	adapter := &testBootstrapAdapter{}

	var registered channel.ChannelAdapter
	starters := registerPreparedBot(
		nil,
		func(a channel.ChannelAdapter) { registered = a },
		mux,
		logger,
		"Test",
		bot,
		adapter,
		bot.Start,
		[]string{"ping", "pong"},
		nil,
	)

	if registered != adapter {
		t.Fatal("expected adapter to be registered")
	}
	if len(starters) != 1 {
		t.Fatalf("len(starters) = %d, want 1", len(starters))
	}
	if len(bot.commands) != 2 || bot.commands[0] != "ping" || bot.commands[1] != "pong" {
		t.Fatalf("commands = %#v, want [ping pong]", bot.commands)
	}
	if !bot.routesRegistered {
		t.Fatal("expected routes to be registered")
	}

	starters[0](context.Background())
	if bot.startCalls != 1 {
		t.Fatalf("startCalls = %d, want 1", bot.startCalls)
	}
}

func TestRegisterPreparedBot_SkipsAdapterWhenFeatureRegistrationFails(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()
	bot := &testBootstrapBot{registerRoutesErr: errors.New("boom")}
	adapter := &testBootstrapAdapter{}

	var registered channel.ChannelAdapter
	starters := registerPreparedBot(
		nil,
		func(a channel.ChannelAdapter) { registered = a },
		mux,
		logger,
		"Test",
		bot,
		adapter,
		bot.Start,
		[]string{"ping"},
		nil,
	)

	if registered != nil {
		t.Fatal("did not expect adapter registration on feature error")
	}
	if len(starters) != 0 {
		t.Fatalf("len(starters) = %d, want 0", len(starters))
	}
	if bot.startCalls != 0 {
		t.Fatalf("startCalls = %d, want 0", bot.startCalls)
	}
}
