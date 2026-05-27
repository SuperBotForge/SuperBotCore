package config

import (
	"strings"
	"testing"
)

func TestApplyDefaultsKeepsSensitiveSpiceDBValuesEmpty(t *testing.T) {
	var cfg Config

	cfg.applyDefaults()

	if cfg.SpiceDB.Endpoint != "" {
		t.Fatalf("SpiceDB.Endpoint = %q, want empty", cfg.SpiceDB.Endpoint)
	}
	if cfg.SpiceDB.Token != "" {
		t.Fatalf("SpiceDB.Token = %q, want empty", cfg.SpiceDB.Token)
	}
	if cfg.Telegram.Mode != "polling" {
		t.Fatalf("Telegram.Mode = %q, want polling", cfg.Telegram.Mode)
	}
	if cfg.VK.CallbackPath != "/vk/callback" {
		t.Fatalf("VK.CallbackPath = %q, want /vk/callback", cfg.VK.CallbackPath)
	}
	if cfg.UserAuth.CookieSameSite != "lax" {
		t.Fatalf("UserAuth.CookieSameSite = %q, want lax", cfg.UserAuth.CookieSameSite)
	}
}

func TestApplyDefaultsUsesLocalSpiceDBEndpointWhenTokenIsExplicit(t *testing.T) {
	cfg := Config{
		SpiceDB: SpiceDBConfig{
			Token: "local-secret",
		},
	}

	cfg.applyDefaults()

	if cfg.SpiceDB.Endpoint != "localhost:50051" {
		t.Fatalf("SpiceDB.Endpoint = %q, want localhost:50051", cfg.SpiceDB.Endpoint)
	}
	if cfg.SpiceDB.Token != "local-secret" {
		t.Fatalf("SpiceDB.Token = %q, want local-secret", cfg.SpiceDB.Token)
	}
}

func TestValidateVKCallbackRequiresURL(t *testing.T) {
	cfg := validConfig()
	cfg.VK.Mode = "callback"

	err := cfg.Validate()
	if err == nil || err.Error() != "vk.callback_url is required when vk.mode=callback" {
		t.Fatalf("Validate() error = %v, want vk.callback_url validation error", err)
	}
}

func TestValidateRejectsInvalidCallbackPaths(t *testing.T) {
	cfg := validConfig()
	cfg.VK.CallbackPath = "vk/callback"

	err := cfg.Validate()
	if err == nil || err.Error() != "vk.callback_path must start with \"/\", got \"vk/callback\"" {
		t.Fatalf("Validate() error = %v, want vk.callback_path validation error", err)
	}

	cfg = validConfig()
	cfg.Mattermost.ActionsPath = "mattermost/actions"

	err = cfg.Validate()
	if err == nil || err.Error() != "mattermost.actions_path must start with \"/\", got \"mattermost/actions\"" {
		t.Fatalf("Validate() error = %v, want mattermost.actions_path validation error", err)
	}
}

func TestValidateMattermostActionsRequireSecret(t *testing.T) {
	cfg := validConfig()
	cfg.Mattermost.ActionsURL = "https://bot.example.com/mattermost/actions"

	err := cfg.Validate()
	if err == nil || err.Error() != "mattermost.actions_url and mattermost.actions_secret must be set together" {
		t.Fatalf("Validate() error = %v, want mattermost.actions validation error", err)
	}
}

func TestValidateRequiresSecurityPairs(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{
			name: "spicedb endpoint without token",
			mutate: func(cfg *Config) {
				cfg.SpiceDB.Endpoint = "localhost:50051"
			},
			wantErr: "spicedb.endpoint and spicedb.token must be set together",
		},
		{
			name: "tsu application id without secret",
			mutate: func(cfg *Config) {
				cfg.TsuAccounts.ApplicationID = "app"
			},
			wantErr: "tsu_accounts.application_id and tsu_accounts.secret_key must be set together",
		},
		{
			name: "admin s3 access key without secret key",
			mutate: func(cfg *Config) {
				cfg.Admin.S3.AccessKey = "access"
			},
			wantErr: "admin.s3.access_key and admin.s3.secret_key must be set together",
		},
		{
			name: "filestore s3 secret key without access key",
			mutate: func(cfg *Config) {
				cfg.FileStore.S3.SecretKey = "secret"
			},
			wantErr: "filestore.s3.access_key and filestore.s3.secret_key must be set together",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.mutate(&cfg)

			err := cfg.Validate()
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("Validate() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateRejectsInvalidUserAuthCookieSameSite(t *testing.T) {
	cfg := validConfig()
	cfg.UserAuth.CookieSameSite = "wide-open"

	err := cfg.Validate()
	if err == nil || err.Error() != "user_auth.cookie_same_site must be \"lax\", \"strict\", or \"none\", got \"wide-open\"" {
		t.Fatalf("Validate() error = %v, want user_auth.cookie_same_site validation error", err)
	}
}

func TestValidateRejectsPlaceholderValues(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{
			name: "telegram token",
			mutate: func(cfg *Config) {
				cfg.Telegram.Token = "YOUR_TELEGRAM_BOT_TOKEN"
			},
			wantErr: "telegram.token contains placeholder value",
		},
		{
			name: "admin api key",
			mutate: func(cfg *Config) {
				cfg.Admin.APIKey = "your-secure-api-key-here"
			},
			wantErr: "admin.api_key contains placeholder value",
		},
		{
			name: "user session secret",
			mutate: func(cfg *Config) {
				cfg.UserAuth.SessionSecret = "CHANGE_ME_USER_SESSION_SECRET"
			},
			wantErr: "user_auth.session_secret contains placeholder value",
		},
		{
			name: "smtp password",
			mutate: func(cfg *Config) {
				cfg.SMTP.Password = "SMTP_PASSWORD"
			},
			wantErr: "smtp.password contains placeholder value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.mutate(&cfg)

			err := cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateAcceptsInteractiveChannelConfig(t *testing.T) {
	cfg := validConfig()
	cfg.VK.Mode = "callback"
	cfg.VK.CallbackURL = "https://bot.example.com/vk/callback"
	cfg.VK.CallbackPath = "/vk/callback"
	cfg.Mattermost.URL = "https://mattermost.example.com"
	cfg.Mattermost.Token = "mm-bot-token-123"
	cfg.Mattermost.ActionsURL = "https://bot.example.com/mattermost/actions"
	cfg.Mattermost.ActionsPath = "/mattermost/actions"
	cfg.Mattermost.ActionsSecret = "mm-actions-secret-123"

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func validConfig() Config {
	return Config{
		Telegram: TelegramConfig{
			Mode: "polling",
		},
		Discord: DiscordConfig{
			ShardCount: 1,
		},
		VK: VKConfig{
			Mode:         "longpoll",
			CallbackPath: "/vk/callback",
		},
		Mattermost: MattermostConfig{
			ActionsPath: "/mattermost/actions",
		},
	}
}
