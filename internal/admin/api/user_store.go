package api

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PgUserStore struct {
	pool *pgxpool.Pool
}

func NewPgUserStore(pool *pgxpool.Pool) *PgUserStore {
	return &PgUserStore{pool: pool}
}

func (s *PgUserStore) ListUsers(ctx context.Context, opts UserListOptions) ([]UserListItem, int, error) {
	baseQuery := "FROM global_users gu LEFT JOIN persons pe ON pe.global_user_id = gu.id WHERE 1=1"
	args := make([]interface{}, 0, 4)
	argNum := 1

	if opts.Search != "" {
		baseQuery += fmt.Sprintf(" AND (gu.id::text LIKE $%d OR pe.last_name ILIKE $%d OR pe.first_name ILIKE $%d OR EXISTS (SELECT 1 FROM channel_accounts ca2 WHERE ca2.global_user_id = gu.id AND (ca2.channel_user_id ILIKE $%d OR ca2.username ILIKE $%d)))", argNum, argNum, argNum, argNum, argNum)
		args = append(args, "%"+opts.Search+"%")
		argNum++
	}
	if opts.Role != "" {
		baseQuery += fmt.Sprintf(" AND gu.role = $%d", argNum)
		args = append(args, opts.Role)
		argNum++
	}
	if opts.Channel != "" {
		baseQuery += fmt.Sprintf(" AND EXISTS (SELECT 1 FROM channel_accounts ca3 WHERE ca3.global_user_id = gu.id AND ca3.channel_type = $%d)", argNum)
		args = append(args, opts.Channel)
		argNum++
	}

	var total int
	countQuery := "SELECT COUNT(DISTINCT gu.id) " + baseQuery
	if err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	dataQuery := `SELECT gu.id, COALESCE(gu.locale, ''), gu.role,
		COALESCE(TRIM(CONCAT(pe.last_name, ' ', pe.first_name, ' ', COALESCE(pe.middle_name, ''))), ''),
		COALESCE((SELECT json_agg(json_build_object('channel_type', ca.channel_type, 'username', COALESCE(ca.username, '')))
		          FROM channel_accounts ca WHERE ca.global_user_id = gu.id)::text, '[]'),
		gu.created_at ` + baseQuery + " ORDER BY gu.id DESC"
	if opts.Limit > 0 {
		dataQuery += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argNum, argNum+1)
		args = append(args, opts.Limit, opts.Offset)
	}

	rows, err := s.pool.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query users: %w", err)
	}
	defer rows.Close()

	users := make([]UserListItem, 0)
	for rows.Next() {
		var u UserListItem
		var accountsStr string
		if err := rows.Scan(&u.ID, &u.Locale, &u.Role, &u.PersonName, &accountsStr, &u.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan user: %w", err)
		}
		u.Accounts = make([]AccountBrief, 0)
		_ = json.Unmarshal([]byte(accountsStr), &u.Accounts)
		users = append(users, u)
	}
	return users, total, nil
}

func (s *PgUserStore) GetUser(ctx context.Context, id int64) (*UserDetail, error) {
	var u UserDetail
	var profileJSON []byte
	err := s.pool.QueryRow(ctx,
		`SELECT gu.id, gu.primary_channel, gu.tsu_accounts_id, COALESCE(gu.locale, ''), gu.role,
		        COALESCE(TRIM(CONCAT(pe.last_name, ' ', pe.first_name, ' ', COALESCE(pe.middle_name, ''))), ''),
		        gu.profile_data, gu.created_at
		 FROM global_users gu
		 LEFT JOIN persons pe ON pe.global_user_id = gu.id
		 WHERE gu.id = $1`, id,
	).Scan(&u.ID, &u.PrimaryChannel, &u.TsuAccountsID, &u.Locale, &u.Role, &u.PersonName, &profileJSON, &u.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if len(profileJSON) > 0 {
		json.Unmarshal(profileJSON, &u.ProfileData)
	}

	rows, err := s.pool.Query(ctx, `SELECT id, channel_type, channel_user_id, COALESCE(username, ''), created_at FROM channel_accounts WHERE global_user_id = $1 ORDER BY created_at`, id)
	if err != nil {
		return nil, fmt.Errorf("get accounts: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var acc AccountInfo
		if err := rows.Scan(&acc.ID, &acc.ChannelType, &acc.ChannelUserID, &acc.Username, &acc.LinkedAt); err != nil {
			return nil, fmt.Errorf("scan account: %w", err)
		}
		u.Accounts = append(u.Accounts, acc)
	}
	return &u, nil
}

func (s *PgUserStore) UpdateUser(ctx context.Context, id int64, req UpdateUserRequest) error {
	result, err := s.pool.Exec(ctx, `UPDATE global_users SET locale = COALESCE(NULLIF($2, ''), locale), role = COALESCE(NULLIF($3, ''), role) WHERE id = $1`,
		id, req.Locale, req.Role)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func (s *PgUserStore) DeleteUser(ctx context.Context, id int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, _ = tx.Exec(ctx, `DELETE FROM channel_accounts WHERE global_user_id = $1`, id)
	_, _ = tx.Exec(ctx, `DELETE FROM user_roles WHERE user_id = $1`, id)

	result, err := tx.Exec(ctx, `DELETE FROM global_users WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("user not found")
	}
	return tx.Commit(ctx)
}

func (s *PgUserStore) GetUserRoles(ctx context.Context, userID int64) ([]UserRoleEntry, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, role_name, role_type, COALESCE(scope, ''), created_at FROM user_roles WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, nil
	}
	defer rows.Close()

	var roles []UserRoleEntry
	for rows.Next() {
		var r UserRoleEntry
		if err := rows.Scan(&r.ID, &r.RoleName, &r.RoleType, &r.Scope, &r.GrantedAt); err != nil {
			return nil, err
		}
		roles = append(roles, r)
	}
	return roles, nil
}

func (s *PgUserStore) RemoveUserRole(ctx context.Context, userID int64, roleName, roleType string) error {
	_, _ = s.pool.Exec(ctx, `DELETE FROM user_roles WHERE user_id = $1 AND role_name = $2 AND role_type = $3`, userID, roleName, roleType)
	return nil
}

func (s *PgUserStore) UnlinkAccount(ctx context.Context, accountID int64) error {
	result, err := s.pool.Exec(ctx, `DELETE FROM channel_accounts WHERE id = $1`, accountID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("account not found")
	}
	return nil
}

func (s *PgUserStore) SearchPersons(ctx context.Context, search string, onlyUnlinked bool, limit int) ([]PersonBrief, error) {
	query := `
		SELECT id, COALESCE(external_id, ''), last_name, first_name, COALESCE(middle_name, ''),
		       COALESCE(email, ''), global_user_id IS NOT NULL
		FROM persons
		WHERE 1=1`
	args := make([]any, 0, 3)
	argN := 1

	if search != "" {
		query += fmt.Sprintf(`
		AND (last_name ILIKE $%d OR first_name ILIKE $%d OR middle_name ILIKE $%d OR external_id ILIKE $%d)`,
			argN, argN, argN, argN)
		args = append(args, "%"+search+"%")
		argN++
	}
	if onlyUnlinked {
		query += " AND global_user_id IS NULL"
	}
	query += fmt.Sprintf(" ORDER BY last_name, first_name LIMIT $%d", argN)
	args = append(args, limit)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("search persons: %w", err)
	}
	defer rows.Close()

	var persons []PersonBrief
	for rows.Next() {
		var p PersonBrief
		if err := rows.Scan(&p.ID, &p.ExternalID, &p.LastName, &p.FirstName, &p.MiddleName, &p.Email, &p.Linked); err != nil {
			return nil, fmt.Errorf("scan person: %w", err)
		}
		persons = append(persons, p)
	}
	return persons, nil
}

func (s *PgUserStore) LinkPerson(ctx context.Context, globalUserID int64, personID int64) error {
	result, err := s.pool.Exec(ctx, `
		UPDATE persons SET global_user_id = $1, updated_at = now()
		WHERE id = $2 AND global_user_id IS NULL
	`, globalUserID, personID)
	if err != nil {
		return fmt.Errorf("link person: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("person not found or already linked to another user")
	}
	return nil
}

func (s *PgUserStore) UnlinkPerson(ctx context.Context, globalUserID int64) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE persons SET global_user_id = NULL, updated_at = now()
		WHERE global_user_id = $1
	`, globalUserID)
	return err
}

var _ UserStore = (*PgUserStore)(nil)
