package api

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"SuperBotGo/internal/weborigin"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CommandSetting struct {
	ID               int64     `json:"id"`
	PluginID         string    `json:"plugin_id"`
	CommandName      string    `json:"command_name"`
	Enabled          bool      `json:"enabled"`
	AllowUserKeys    bool      `json:"allow_user_keys"`
	AllowServiceKeys bool      `json:"allow_service_keys"`
	PolicyExpression string    `json:"policy_expression"`
	AllowedOrigins   []string  `json:"allowed_origins"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type PluginFrontendOrigins struct {
	PluginID       string    `json:"plugin_id"`
	AllowedOrigins []string  `json:"allowed_origins"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type CommandPermStore interface {
	ListCommandSettings(ctx context.Context, pluginID string) ([]CommandSetting, error)
	GetCommandSetting(ctx context.Context, pluginID, commandName string) (CommandSetting, bool, error)
	GetPluginFrontendOrigins(ctx context.Context, pluginID string) (PluginFrontendOrigins, bool, error)
	SetCommandEnabled(ctx context.Context, pluginID, commandName string, enabled bool) error
	SetTriggerAccess(ctx context.Context, pluginID, commandName string, allowUserKeys, allowServiceKeys bool) error
	SetPolicyExpression(ctx context.Context, pluginID, commandName, expression string) error
	GetPolicyExpression(ctx context.Context, pluginID, commandName string) (string, error)
	SetAllowedOrigins(ctx context.Context, pluginID, commandName string, origins []string) error
	SetPluginFrontendOrigins(ctx context.Context, pluginID string, origins []string) error
	DeleteCommandSettings(ctx context.Context, pluginID string, commandNames []string) error
	DeleteAllPluginCommandSettings(ctx context.Context, pluginID string) error
}

type PgCommandPermStore struct {
	pool *pgxpool.Pool
}

func NewPgCommandPermStore(pool *pgxpool.Pool) *PgCommandPermStore {
	return &PgCommandPermStore{pool: pool}
}

func (s *PgCommandPermStore) ListCommandSettings(ctx context.Context, pluginID string) ([]CommandSetting, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, plugin_id, command_name, enabled, allow_user_keys, allow_service_keys,
		       COALESCE(policy_expression, ''), allowed_origins::text, created_at, updated_at
		FROM plugin_command_settings
		WHERE plugin_id = $1
		ORDER BY command_name
	`, pluginID)
	if err != nil {
		return nil, fmt.Errorf("list command settings for %q: %w", pluginID, err)
	}
	defer rows.Close()

	var settings []CommandSetting
	for rows.Next() {
		var s CommandSetting
		var allowedOrigins string
		if err := rows.Scan(
			&s.ID,
			&s.PluginID,
			&s.CommandName,
			&s.Enabled,
			&s.AllowUserKeys,
			&s.AllowServiceKeys,
			&s.PolicyExpression,
			&allowedOrigins,
			&s.CreatedAt,
			&s.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan command setting: %w", err)
		}
		if err := parseAllowedOriginsJSON(allowedOrigins, &s.AllowedOrigins); err != nil {
			return nil, fmt.Errorf("scan command setting allowed origins: %w", err)
		}
		settings = append(settings, s)
	}
	return settings, rows.Err()
}

func (s *PgCommandPermStore) GetCommandSetting(ctx context.Context, pluginID, commandName string) (CommandSetting, bool, error) {
	var setting CommandSetting
	var allowedOrigins string
	err := s.pool.QueryRow(ctx, `
		SELECT id, plugin_id, command_name, enabled, allow_user_keys, allow_service_keys,
		       COALESCE(policy_expression, ''), allowed_origins::text, created_at, updated_at
		FROM plugin_command_settings
		WHERE plugin_id = $1 AND command_name = $2
	`, pluginID, commandName).Scan(
		&setting.ID,
		&setting.PluginID,
		&setting.CommandName,
		&setting.Enabled,
		&setting.AllowUserKeys,
		&setting.AllowServiceKeys,
		&setting.PolicyExpression,
		&allowedOrigins,
		&setting.CreatedAt,
		&setting.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return CommandSetting{}, false, nil
		}
		return CommandSetting{}, false, fmt.Errorf("get command setting %q/%q: %w", pluginID, commandName, err)
	}
	if err := parseAllowedOriginsJSON(allowedOrigins, &setting.AllowedOrigins); err != nil {
		return CommandSetting{}, false, fmt.Errorf("get command setting %q/%q allowed origins: %w", pluginID, commandName, err)
	}
	return setting, true, nil
}

func (s *PgCommandPermStore) GetPluginFrontendOrigins(ctx context.Context, pluginID string) (PluginFrontendOrigins, bool, error) {
	var setting PluginFrontendOrigins
	var allowedOrigins string
	err := s.pool.QueryRow(ctx, `
		SELECT plugin_id, allowed_origins::text, created_at, updated_at
		FROM plugin_frontend_origins
		WHERE plugin_id = $1
	`, pluginID).Scan(
		&setting.PluginID,
		&allowedOrigins,
		&setting.CreatedAt,
		&setting.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return PluginFrontendOrigins{}, false, nil
		}
		return PluginFrontendOrigins{}, false, fmt.Errorf("get plugin frontend origins %q: %w", pluginID, err)
	}
	if err := parseAllowedOriginsJSON(allowedOrigins, &setting.AllowedOrigins); err != nil {
		return PluginFrontendOrigins{}, false, fmt.Errorf("get plugin frontend origins %q allowed origins: %w", pluginID, err)
	}
	return setting, true, nil
}

func (s *PgCommandPermStore) SetCommandEnabled(ctx context.Context, pluginID, commandName string, enabled bool) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO plugin_command_settings (plugin_id, command_name, enabled, updated_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (plugin_id, command_name) DO UPDATE SET
			enabled    = EXCLUDED.enabled,
			updated_at = now()
	`, pluginID, commandName, enabled)
	if err != nil {
		return fmt.Errorf("set command enabled %q/%q: %w", pluginID, commandName, err)
	}
	return nil
}

func (s *PgCommandPermStore) SetTriggerAccess(ctx context.Context, pluginID, commandName string, allowUserKeys, allowServiceKeys bool) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO plugin_command_settings (plugin_id, command_name, enabled, allow_user_keys, allow_service_keys, updated_at)
		VALUES ($1, $2, true, $3, $4, now())
		ON CONFLICT (plugin_id, command_name) DO UPDATE SET
			allow_user_keys = EXCLUDED.allow_user_keys,
			allow_service_keys = EXCLUDED.allow_service_keys,
			updated_at = now()
	`, pluginID, commandName, allowUserKeys, allowServiceKeys)
	if err != nil {
		return fmt.Errorf("set trigger access %q/%q: %w", pluginID, commandName, err)
	}
	return nil
}

func (s *PgCommandPermStore) SetPolicyExpression(ctx context.Context, pluginID, commandName, expression string) error {
	var policyExpr *string
	if expression != "" {
		policyExpr = &expression
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO plugin_command_settings (plugin_id, command_name, enabled, policy_expression, updated_at)
		VALUES ($1, $2, true, $3, now())
		ON CONFLICT (plugin_id, command_name) DO UPDATE SET
			policy_expression = EXCLUDED.policy_expression,
			updated_at = now()
	`, pluginID, commandName, policyExpr)
	if err != nil {
		return fmt.Errorf("set policy expression %q/%q: %w", pluginID, commandName, err)
	}
	return nil
}

func (s *PgCommandPermStore) GetPolicyExpression(ctx context.Context, pluginID, commandName string) (string, error) {
	var expr *string
	err := s.pool.QueryRow(ctx, `
		SELECT policy_expression FROM plugin_command_settings
		WHERE plugin_id = $1 AND command_name = $2
	`, pluginID, commandName).Scan(&expr)
	if err != nil {
		return "", nil
	}
	if expr == nil {
		return "", nil
	}
	return *expr, nil
}

func (s *PgCommandPermStore) SetAllowedOrigins(ctx context.Context, pluginID, commandName string, origins []string) error {
	origins, err := weborigin.CanonicalizeList(origins)
	if err != nil {
		return err
	}
	rawOrigins, err := json.Marshal(origins)
	if err != nil {
		return fmt.Errorf("marshal allowed origins: %w", err)
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO plugin_command_settings (plugin_id, command_name, enabled, allowed_origins, updated_at)
		VALUES ($1, $2, true, $3::jsonb, now())
		ON CONFLICT (plugin_id, command_name) DO UPDATE SET
			allowed_origins = EXCLUDED.allowed_origins,
			updated_at = now()
	`, pluginID, commandName, string(rawOrigins))
	if err != nil {
		return fmt.Errorf("set allowed origins %q/%q: %w", pluginID, commandName, err)
	}
	return nil
}

func (s *PgCommandPermStore) SetPluginFrontendOrigins(ctx context.Context, pluginID string, origins []string) error {
	origins, err := weborigin.CanonicalizeList(origins)
	if err != nil {
		return err
	}
	rawOrigins, err := json.Marshal(origins)
	if err != nil {
		return fmt.Errorf("marshal plugin frontend origins: %w", err)
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO plugin_frontend_origins (plugin_id, allowed_origins, updated_at)
		VALUES ($1, $2::jsonb, now())
		ON CONFLICT (plugin_id) DO UPDATE SET
			allowed_origins = EXCLUDED.allowed_origins,
			updated_at = now()
	`, pluginID, string(rawOrigins))
	if err != nil {
		return fmt.Errorf("set plugin frontend origins %q: %w", pluginID, err)
	}
	return nil
}

func (s *PgCommandPermStore) IsAllowedFrontendOrigin(ctx context.Context, origin string) (bool, error) {
	origin, err := weborigin.Canonicalize(origin)
	if err != nil {
		return false, err
	}

	var allowed bool
	err = s.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM plugin_frontend_origins
			WHERE allowed_origins ? $1
		) OR EXISTS (
			SELECT 1
			FROM plugin_command_settings
			WHERE allowed_origins ? $1
		)
	`, origin).Scan(&allowed)
	if err != nil {
		return false, fmt.Errorf("check allowed frontend origin %q: %w", origin, err)
	}
	return allowed, nil
}

func parseAllowedOriginsJSON(value string, target *[]string) error {
	if value == "" {
		*target = []string{}
		return nil
	}
	if err := json.Unmarshal([]byte(value), target); err != nil {
		return err
	}
	if *target == nil {
		*target = []string{}
	}
	return nil
}

func (s *PgCommandPermStore) DeleteCommandSettings(ctx context.Context, pluginID string, commandNames []string) error {
	if len(commandNames) == 0 {
		return nil
	}
	_, err := s.pool.Exec(ctx, `
		DELETE FROM plugin_command_settings
		WHERE plugin_id = $1 AND command_name = ANY($2)
	`, pluginID, commandNames)
	if err != nil {
		return fmt.Errorf("delete command settings for %q: %w", pluginID, err)
	}
	return nil
}

func (s *PgCommandPermStore) DeleteAllPluginCommandSettings(ctx context.Context, pluginID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin delete all command settings for %q: %w", pluginID, err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		DELETE FROM plugin_command_settings
		WHERE plugin_id = $1
	`, pluginID); err != nil {
		return fmt.Errorf("delete all command settings for %q: %w", pluginID, err)
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM plugin_frontend_origins
		WHERE plugin_id = $1
	`, pluginID); err != nil {
		return fmt.Errorf("delete plugin frontend origins for %q: %w", pluginID, err)
	}
	return tx.Commit(ctx)
}

var _ CommandPermStore = (*PgCommandPermStore)(nil)
