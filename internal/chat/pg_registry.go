package chat

import (
	"context"
	"fmt"

	"SuperBotGo/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PgRegistry struct {
	pool *pgxpool.Pool
}

func NewPgRegistry(pool *pgxpool.Pool) *PgRegistry {
	return &PgRegistry{pool: pool}
}

func (r *PgRegistry) FindOrCreateChat(ctx context.Context, channelType model.ChannelType, chatID string, kind model.ChatKind, title string) (*model.ChatReference, error) {
	existing, err := r.FindChat(ctx, channelType, chatID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if title != "" && existing.Title != title {
			if _, err := r.pool.Exec(ctx,
				`UPDATE chat_references SET title = $1 WHERE id = $2`,
				title, existing.ID); err != nil {
				return nil, fmt.Errorf("update chat title: %w", err)
			}
			existing.Title = title
		}
		return existing, nil
	}
	return r.RegisterChat(ctx, model.ChatReference{
		ChannelType:    channelType,
		PlatformChatID: chatID,
		ChatKind:       kind,
		Title:          title,
	})
}

func (r *PgRegistry) FindChat(ctx context.Context, channelType model.ChannelType, platformChatID string) (*model.ChatReference, error) {
	var c model.ChatReference
	err := r.pool.QueryRow(ctx,
		`SELECT id, channel_type, platform_chat_id, chat_kind, COALESCE(title, ''), COALESCE(locale, '')
		 FROM chat_references
		 WHERE channel_type = $1 AND platform_chat_id = $2`,
		string(channelType), platformChatID,
	).Scan(&c.ID, &c.ChannelType, &c.PlatformChatID, &c.ChatKind, &c.Title, &c.Locale)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find chat: %w", err)
	}
	return &c, nil
}

func (r *PgRegistry) RegisterChat(ctx context.Context, ref model.ChatReference) (*model.ChatReference, error) {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO chat_references (channel_type, platform_chat_id, chat_kind, title)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id`,
		string(ref.ChannelType), ref.PlatformChatID, string(ref.ChatKind), ref.Title,
	).Scan(&ref.ID)
	if err != nil {
		return nil, fmt.Errorf("register chat: %w", err)
	}
	return &ref, nil
}

func (r *PgRegistry) UnregisterChat(ctx context.Context, chatRefID int64) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("unregister chat: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM chat_bindings WHERE chat_reference_id = $1`, chatRefID); err != nil {
		return fmt.Errorf("unregister chat: delete bindings: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM chat_reference_metadata WHERE chat_reference_id = $1`, chatRefID); err != nil {
		return fmt.Errorf("unregister chat: delete metadata: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM chat_references WHERE id = $1`, chatRefID); err != nil {
		return fmt.Errorf("unregister chat: delete ref: %w", err)
	}

	return tx.Commit(ctx)
}

func (r *PgRegistry) UnregisterChatByPlatformID(ctx context.Context, channelType model.ChannelType, platformChatID string) error {
	ref, err := r.FindChat(ctx, channelType, platformChatID)
	if err != nil {
		return err
	}
	if ref == nil {
		return nil
	}
	return r.UnregisterChat(ctx, ref.ID)
}

func (r *PgRegistry) UpdateChatLocale(ctx context.Context, chatRefID int64, locale string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE chat_references SET locale = $1 WHERE id = $2`,
		locale, chatRefID,
	)
	if err != nil {
		return fmt.Errorf("update chat locale: %w", err)
	}
	return nil
}

func (r *PgRegistry) FindChatGroupID(ctx context.Context, channelType model.ChannelType, platformChatID string) (int64, error) {
	var projectID int64
	err := r.pool.QueryRow(ctx,
		`SELECT cb.project_id
		 FROM chat_bindings cb
		 JOIN chat_references cr ON cr.id = cb.chat_reference_id
		 WHERE cr.channel_type = $1 AND cr.platform_chat_id = $2
		 LIMIT 1`,
		string(channelType), platformChatID,
	).Scan(&projectID)
	if err == pgx.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("find chat group id: %w", err)
	}
	return projectID, nil
}

var _ Registry = (*PgRegistry)(nil)
