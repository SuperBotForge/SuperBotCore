package authz

import (
	"context"
	"testing"
	"time"

	"SuperBotGo/internal/model"
)

type authorizerTestStore struct {
	getExternalIDCalls           int
	getAllRoleNamesCalls         int
	getUserChannelAndLocaleCalls int

	externalID string
	roles      []string
	channel    string
	locale     string
}

func (s *authorizerTestStore) GetRoles(context.Context, model.GlobalUserID, model.RoleLayer) ([]model.UserRole, error) {
	return nil, nil
}

func (s *authorizerTestStore) GetAllRoleNames(context.Context, model.GlobalUserID) ([]string, error) {
	s.getAllRoleNamesCalls++
	return s.roles, nil
}

func (s *authorizerTestStore) GetCommandPolicy(context.Context, string, string) (bool, string, bool, error) {
	return false, "", false, nil
}

func (s *authorizerTestStore) GetExternalID(context.Context, model.GlobalUserID) (string, error) {
	s.getExternalIDCalls++
	return s.externalID, nil
}

func (s *authorizerTestStore) GetUserChannelAndLocale(context.Context, model.GlobalUserID) (string, string, error) {
	s.getUserChannelAndLocaleCalls++
	return s.channel, s.locale, nil
}

func (s *authorizerTestStore) GetPluginPolicy(context.Context, string) (string, error) {
	return "", nil
}

func (s *authorizerTestStore) GetDistinctRoleNames(context.Context) []string {
	return nil
}

type authorizerTestProvider struct {
	calls int
}

func (p *authorizerTestProvider) LoadAttributes(_ context.Context, sc *SubjectContext) error {
	p.calls++
	sc.Attrs["provider"] = "ok"
	return nil
}

func TestAuthorizerBuildSubjectContext_LoadsAndCaches(t *testing.T) {
	t.Parallel()

	store := &authorizerTestStore{
		externalID: "ext-42",
		roles:      []string{"admin"},
		channel:    "telegram",
		locale:     "ru",
	}
	provider := &authorizerTestProvider{}
	auth := NewAuthorizerWithTTL(store, nil, nil, time.Minute, 0, provider)

	sc, err := auth.buildSubjectContext(context.Background(), 42)
	if err != nil {
		t.Fatalf("buildSubjectContext() error = %v", err)
	}
	if sc.ExternalID != "ext-42" {
		t.Fatalf("ExternalID = %q, want %q", sc.ExternalID, "ext-42")
	}
	if len(sc.Roles) != 1 || sc.Roles[0] != "admin" {
		t.Fatalf("Roles = %#v, want [admin]", sc.Roles)
	}
	if sc.PrimaryChannel != "telegram" || sc.Locale != "ru" {
		t.Fatalf("channel/locale = %q/%q, want telegram/ru", sc.PrimaryChannel, sc.Locale)
	}
	if got := sc.Attrs["provider"]; got != "ok" {
		t.Fatalf("provider attr = %#v, want %q", got, "ok")
	}

	scCached, err := auth.buildSubjectContext(context.Background(), 42)
	if err != nil {
		t.Fatalf("buildSubjectContext(cached) error = %v", err)
	}
	if scCached.ExternalID != sc.ExternalID {
		t.Fatalf("cached ExternalID = %q, want %q", scCached.ExternalID, sc.ExternalID)
	}

	if store.getExternalIDCalls != 1 || store.getAllRoleNamesCalls != 1 || store.getUserChannelAndLocaleCalls != 1 {
		t.Fatalf("unexpected store call counts: ext=%d roles=%d channel=%d",
			store.getExternalIDCalls, store.getAllRoleNamesCalls, store.getUserChannelAndLocaleCalls)
	}
	if provider.calls != 1 {
		t.Fatalf("provider calls = %d, want 1", provider.calls)
	}
}

func TestAuthorizerBuildSubjectContext_SkipsEnrichmentWithoutExternalID(t *testing.T) {
	t.Parallel()

	store := &authorizerTestStore{
		externalID: "",
		roles:      []string{"user"},
		channel:    "discord",
		locale:     "en",
	}
	provider := &authorizerTestProvider{}
	auth := NewAuthorizerWithTTL(store, nil, nil, time.Minute, 0, provider)

	sc, err := auth.buildSubjectContext(context.Background(), 7)
	if err != nil {
		t.Fatalf("buildSubjectContext() error = %v", err)
	}
	if sc.ExternalID != "" {
		t.Fatalf("ExternalID = %q, want empty", sc.ExternalID)
	}
	if provider.calls != 0 {
		t.Fatalf("provider calls = %d, want 0 when external ID is missing", provider.calls)
	}
}
