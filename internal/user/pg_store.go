package user

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"SuperBotGo/internal/locale"
	"SuperBotGo/internal/model"
)

type PgUserRepo struct {
	pool *pgxpool.Pool
}

func NewPgUserRepo(pool *pgxpool.Pool) *PgUserRepo {
	return &PgUserRepo{pool: pool}
}

func (r *PgUserRepo) FindByID(ctx context.Context, id model.GlobalUserID) (*model.GlobalUser, error) {
	var u model.GlobalUser
	var tsuID *string
	var profileJSON *string
	var locale *string

	err := r.pool.QueryRow(ctx, `
		SELECT id, tsu_accounts_id, primary_channel, profile_data, locale, role
		FROM global_users WHERE id = $1
	`, id).Scan(&u.ID, &tsuID, &u.PrimaryChannel, &profileJSON, &locale, &u.Role)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find user %d: %w", id, err)
	}

	u.TsuAccountsID = tsuID
	if locale != nil {
		u.Locale = *locale
	}
	if profileJSON != nil && *profileJSON != "" {
		if err := json.Unmarshal([]byte(*profileJSON), &u.ProfileData); err != nil {
			return nil, fmt.Errorf("unmarshal profile data for user %d: %w", id, err)
		}
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, channel_type, channel_user_id, global_user_id
		FROM channel_accounts WHERE global_user_id = $1
	`, id)
	if err != nil {
		return nil, fmt.Errorf("find accounts for user %d: %w", id, err)
	}
	defer rows.Close()

	for rows.Next() {
		var acc model.ChannelAccount
		if err := rows.Scan(&acc.ID, &acc.ChannelType, &acc.ChannelUserID, &acc.GlobalUserID); err != nil {
			return nil, fmt.Errorf("scan account: %w", err)
		}
		u.Accounts = append(u.Accounts, acc)
	}

	return &u, nil
}

func (r *PgUserRepo) Save(ctx context.Context, user *model.GlobalUser) (*model.GlobalUser, error) {
	var profileStr *string
	if user.ProfileData != nil {
		b, err := json.Marshal(user.ProfileData)
		if err != nil {
			return nil, fmt.Errorf("marshal profile data: %w", err)
		}
		s := string(b)
		profileStr = &s
	}

	role := user.Role
	if role == "" {
		role = "USER"
	}

	loc := user.Locale
	if loc == "" {
		loc = locale.Default()
	}

	if user.ID == 0 {
		err := r.pool.QueryRow(ctx, `
			INSERT INTO global_users (tsu_accounts_id, primary_channel, profile_data, locale, role)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id
		`, nil, user.PrimaryChannel, profileStr, loc, role).Scan(&user.ID)
		if err != nil {
			return nil, fmt.Errorf("insert user: %w", err)
		}
	} else {
		_, err := r.pool.Exec(ctx, `
			UPDATE global_users
			SET primary_channel = $2, profile_data = $3, locale = $4, role = $5
			WHERE id = $1
		`, user.ID, user.PrimaryChannel, profileStr, loc, role)
		if err != nil {
			return nil, fmt.Errorf("update user %d: %w", user.ID, err)
		}
	}

	return user, nil
}

func (r *PgUserRepo) FindByTsuAccountsID(ctx context.Context, tsuAccountsID string) (*model.GlobalUser, error) {
	var id model.GlobalUserID
	err := r.pool.QueryRow(ctx, `
		SELECT id FROM global_users WHERE tsu_accounts_id = $1
	`, tsuAccountsID).Scan(&id)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find user by tsu_accounts_id %s: %w", tsuAccountsID, err)
	}
	return r.FindByID(ctx, id)
}

func (r *PgUserRepo) Delete(ctx context.Context, id model.GlobalUserID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM global_users WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete user %d: %w", id, err)
	}
	return nil
}

func (r *PgUserRepo) SetTsuAccountsID(ctx context.Context, userID model.GlobalUserID, tsuAccountsID string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE global_users SET tsu_accounts_id = $2 WHERE id = $1`, userID, tsuAccountsID)
	if err != nil {
		return fmt.Errorf("set tsu_accounts_id for user %d: %w", userID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("user %d not found", userID)
	}
	return nil
}

func (r *PgUserRepo) UpdateLocale(ctx context.Context, userID model.GlobalUserID, locale string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE global_users SET locale = $2 WHERE id = $1`, userID, locale)
	if err != nil {
		return fmt.Errorf("update locale for user %d: %w", userID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("user %d not found", userID)
	}
	return nil
}

func (r *PgUserRepo) GetUserInfo(ctx context.Context, userID int64) (*model.UserInfo, error) {
	var info model.UserInfo
	err := r.pool.QueryRow(ctx, `
		SELECT gu.id,
		    COALESCE(
		        NULLIF(TRIM(CONCAT(pe.last_name, ' ', pe.first_name, ' ', COALESCE(pe.middle_name, ''))), ''),
		        COALESCE((
		            SELECT NULLIF(TRIM(username), '')
		            FROM channel_accounts
		            WHERE global_user_id = gu.id AND channel_type = gu.primary_channel
		            LIMIT 1
		        ), ''),
		        ''
		    ),
		    COALESCE(pe.external_id, ''),
		    COALESCE(gu.tsu_accounts_id, ''),
		    EXISTS(
		        SELECT 1 FROM teacher_positions tp
		        WHERE tp.person_id = pe.id AND tp.status = 'active'
		    ),
		    EXISTS(
		        SELECT 1 FROM student_positions sp
		        WHERE sp.person_id = pe.id AND sp.status = 'active'
		    ),
		    EXISTS(
		        SELECT 1 FROM administrative_appointments aa
		        WHERE aa.person_id = pe.id AND aa.appointment_type = 'dean'
		          AND aa.scope_type = 'faculty' AND aa.status = 'active'
		    )
		FROM global_users gu
		LEFT JOIN persons pe ON pe.global_user_id = gu.id
		WHERE gu.id = $1
	`, userID).Scan(&info.ID, &info.FullName, &info.ExternalID, &info.TsuAccountsID, &info.IsTeacher, &info.IsStudent, &info.IsDeanOffice)
	info.TsuLinked = info.TsuAccountsID != ""
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("user %d not found", userID)
	}
	if err != nil {
		return nil, fmt.Errorf("get user info %d: %w", userID, err)
	}
	return &info, nil
}

func (r *PgUserRepo) GetUsersInfo(ctx context.Context, userIDs []int64) ([]model.UserInfoFull, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
		    gu.id,
		    COALESCE(
		        NULLIF(TRIM(CONCAT(pe.last_name, ' ', pe.first_name, ' ', COALESCE(pe.middle_name, ''))), ''),
		        COALESCE((
		            SELECT NULLIF(TRIM(username), '')
		            FROM channel_accounts
		            WHERE global_user_id = gu.id AND channel_type = gu.primary_channel
		            LIMIT 1
		        ), ''),
		        ''
		    ),
		    COALESCE(pe.external_id, ''),
		    COALESCE(gu.tsu_accounts_id, ''),
		    EXISTS(
		        SELECT 1 FROM teacher_positions tp
		        WHERE tp.person_id = pe.id AND tp.status = 'active'
		    ),
		    COALESCE(sp.status, ''),
		    COALESCE(sp.nationality_type, ''),
		    COALESCE(sp.funding_type, ''),
		    COALESCE(sp.education_form, ''),
		    COALESCE(f.name, ''),
		    COALESCE(d.name, ''),
		    COALESCE(pr.name, ''),
		    COALESCE(st.name, ''),
		    COALESCE(sg.code, ''),
		    COALESCE(sg.name, '')
		FROM global_users gu
		LEFT JOIN persons pe ON pe.global_user_id = gu.id
		LEFT JOIN student_positions sp ON sp.person_id = pe.id AND sp.status = 'active'
		LEFT JOIN study_groups sg ON sg.id = sp.study_group_id
		LEFT JOIN streams st ON st.id = sg.stream_id
		LEFT JOIN programs pr ON pr.id = st.program_id
		LEFT JOIN departments d ON d.id = pr.department_id
		LEFT JOIN faculties f ON f.id = d.faculty_id
		WHERE gu.id = ANY($1)
		ORDER BY gu.id
	`, userIDs)
	if err != nil {
		return nil, fmt.Errorf("get users info: %w", err)
	}
	defer rows.Close()

	index := make(map[int64]int)
	var result []model.UserInfoFull

	for rows.Next() {
		var (
			id              int64
			fullName        string
			externalID      string
			tsuAccountsID   string
			isTeacher       bool
			posStatus       string
			nationalityType string
			fundingType     string
			educationForm   string
			facultyName     string
			departmentName  string
			programName     string
			streamName      string
			groupCode       string
			groupName       string
		)
		if err := rows.Scan(
			&id, &fullName, &externalID, &tsuAccountsID, &isTeacher,
			&posStatus, &nationalityType, &fundingType, &educationForm,
			&facultyName, &departmentName, &programName, &streamName,
			&groupCode, &groupName,
		); err != nil {
			return nil, fmt.Errorf("scan user info row: %w", err)
		}

		idx, exists := index[id]
		if !exists {
			idx = len(result)
			index[id] = idx
			result = append(result, model.UserInfoFull{
				UserInfo: model.UserInfo{
					ID:            id,
					FullName:      fullName,
					ExternalID:    externalID,
					TsuAccountsID: tsuAccountsID,
					TsuLinked:     tsuAccountsID != "",
					IsTeacher:     isTeacher,
				},
			})
		}

		if posStatus != "" {
			result[idx].Positions = append(result[idx].Positions, model.UserPosition{
				PositionType:    "student",
				Status:          posStatus,
				NationalityType: nationalityType,
				FundingType:     fundingType,
				EducationForm:   educationForm,
				FacultyName:     facultyName,
				DepartmentName:  departmentName,
				ProgramName:     programName,
				StreamName:      streamName,
				GroupCode:       groupCode,
				GroupName:       groupName,
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user info rows: %w", err)
	}

	return result, nil
}

var _ UserRepository = (*PgUserRepo)(nil)

type PgAccountRepo struct {
	pool *pgxpool.Pool
}

func NewPgAccountRepo(pool *pgxpool.Pool) *PgAccountRepo {
	return &PgAccountRepo{pool: pool}
}

func (r *PgAccountRepo) FindByChannelAndPlatformID(ctx context.Context, ct model.ChannelType, platformID model.PlatformUserID) (*model.ChannelAccount, error) {
	var acc model.ChannelAccount
	var username *string
	err := r.pool.QueryRow(ctx, `
		SELECT id, channel_type, channel_user_id, global_user_id, username
		FROM channel_accounts
		WHERE channel_type = $1 AND channel_user_id = $2
	`, ct, platformID).Scan(&acc.ID, &acc.ChannelType, &acc.ChannelUserID, &acc.GlobalUserID, &username)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find account %s/%s: %w", ct, platformID, err)
	}
	if username != nil {
		acc.Username = *username
	}
	return &acc, nil
}

func (r *PgAccountRepo) FindByGlobalUserID(ctx context.Context, globalUserID model.GlobalUserID) ([]model.ChannelAccount, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, channel_type, channel_user_id, global_user_id
		FROM channel_accounts WHERE global_user_id = $1
	`, globalUserID)
	if err != nil {
		return nil, fmt.Errorf("find accounts for user %d: %w", globalUserID, err)
	}
	defer rows.Close()

	var result []model.ChannelAccount
	for rows.Next() {
		var acc model.ChannelAccount
		if err := rows.Scan(&acc.ID, &acc.ChannelType, &acc.ChannelUserID, &acc.GlobalUserID); err != nil {
			return nil, fmt.Errorf("scan account: %w", err)
		}
		result = append(result, acc)
	}
	return result, nil
}

func (r *PgAccountRepo) Save(ctx context.Context, account *model.ChannelAccount) (*model.ChannelAccount, error) {
	if account.ID == 0 {
		err := r.pool.QueryRow(ctx, `
			INSERT INTO channel_accounts (channel_type, channel_user_id, global_user_id, username)
			VALUES ($1, $2, $3, NULLIF($4, ''))
			RETURNING id
		`, account.ChannelType, account.ChannelUserID, account.GlobalUserID, account.Username).Scan(&account.ID)
		if err != nil {
			return nil, fmt.Errorf("insert account: %w", err)
		}
	} else {
		_, err := r.pool.Exec(ctx, `
			UPDATE channel_accounts
			SET channel_type = $2, channel_user_id = $3, global_user_id = $4, username = COALESCE(NULLIF($5, ''), username)
			WHERE id = $1
		`, account.ID, account.ChannelType, account.ChannelUserID, account.GlobalUserID, account.Username)
		if err != nil {
			return nil, fmt.Errorf("update account %d: %w", account.ID, err)
		}
	}
	return account, nil
}

var _ AccountRepository = (*PgAccountRepo)(nil)
