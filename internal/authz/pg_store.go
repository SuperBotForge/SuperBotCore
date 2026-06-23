package authz

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"SuperBotGo/internal/model"
)

type PgStore struct {
	pool *pgxpool.Pool
}

func NewPgStore(pool *pgxpool.Pool) *PgStore {
	return &PgStore{pool: pool}
}

func (s *PgStore) GetRoles(ctx context.Context, userID model.GlobalUserID, roleType model.RoleLayer) ([]model.UserRole, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, role_type, role_name
		FROM user_roles
		WHERE user_id = $1 AND role_type = $2
	`, userID, roleType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []model.UserRole
	for rows.Next() {
		var r model.UserRole
		if err := rows.Scan(&r.ID, &r.UserID, &r.RoleType, &r.RoleName); err != nil {
			return nil, err
		}
		roles = append(roles, r)
	}
	return roles, rows.Err()
}

func (s *PgStore) GetAllRoleNames(ctx context.Context, userID model.GlobalUserID) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT role_name FROM user_roles WHERE user_id = $1
		UNION
		SELECT role FROM global_users WHERE id = $1 AND role IS NOT NULL AND role <> ''
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

func (s *PgStore) GetCommandPolicy(ctx context.Context, pluginID, commandName string) (bool, string, bool, error) {
	var enabled bool
	var policyExpr *string
	err := s.pool.QueryRow(ctx, `
		SELECT enabled, policy_expression FROM plugin_command_settings
		WHERE plugin_id = $1 AND command_name = $2
	`, pluginID, commandName).Scan(&enabled, &policyExpr)

	if err == pgx.ErrNoRows {
		return true, "", false, nil
	}
	if err != nil {
		return false, "", false, err
	}

	expr := ""
	if policyExpr != nil {
		expr = *policyExpr
	}
	return enabled, expr, true, nil
}

func (s *PgStore) GetExternalID(ctx context.Context, userID model.GlobalUserID) (string, error) {
	var extID *string
	err := s.pool.QueryRow(ctx, `
		SELECT external_id FROM persons WHERE global_user_id = $1
	`, userID).Scan(&extID)

	if err == pgx.ErrNoRows || extID == nil {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return *extID, nil
}

func (s *PgStore) GetUserChannelAndLocale(ctx context.Context, userID model.GlobalUserID) (string, string, error) {
	var ch, loc *string
	err := s.pool.QueryRow(ctx, `
		SELECT primary_channel, locale FROM global_users WHERE id = $1
	`, userID).Scan(&ch, &loc)

	if err == pgx.ErrNoRows {
		return "", "", nil
	}
	if err != nil {
		return "", "", err
	}

	primaryChannel := ""
	locale := ""
	if ch != nil {
		primaryChannel = *ch
	}
	if loc != nil {
		locale = *loc
	}
	return primaryChannel, locale, nil
}

func (s *PgStore) GetPluginPolicy(ctx context.Context, pluginID string) (string, error) {
	var expr string
	err := s.pool.QueryRow(ctx, `
		SELECT policy_expression FROM plugin_settings WHERE plugin_id = $1
	`, pluginID).Scan(&expr)
	if err != nil {
		return "", nil
	}
	return expr, nil
}

func (s *PgStore) GetDistinctRoleNames(ctx context.Context) []string {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT role_name FROM user_roles
		UNION
		SELECT DISTINCT role FROM global_users WHERE role IS NOT NULL AND role <> ''
		ORDER BY 1
	`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if rows.Scan(&name) == nil {
			names = append(names, name)
		}
	}
	return names
}

var _ Store = (*PgStore)(nil)
