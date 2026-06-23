package tsu

import (
	"context"
	"log/slog"

	"SuperBotGo/internal/i18n"
	"SuperBotGo/internal/model"
	"SuperBotGo/internal/user"
)

// Notifier sends a message to a user on a specific channel.
type Notifier interface {
	SendToUser(ctx context.Context, channelType model.ChannelType, platformUserID model.PlatformUserID, msg model.Message) error
}

// PersonLinker links a person record to a global user by external ID.
type PersonLinker interface {
	LinkByExternalID(ctx context.Context, globalUserID model.GlobalUserID, externalID string) error
}

// Linker handles the business logic of linking TSU AccountId
// to a global user, merging accounts, and auto-linking persons.
type Linker struct {
	userRepo     user.UserRepository
	accountRepo  user.AccountRepository
	personLinker PersonLinker
	notifier     Notifier
	logger       *slog.Logger
}

func NewLinker(
	userRepo user.UserRepository,
	accountRepo user.AccountRepository,
	personLinker PersonLinker,
	logger *slog.Logger,
) *Linker {
	return &Linker{
		userRepo:     userRepo,
		accountRepo:  accountRepo,
		personLinker: personLinker,
		logger:       logger,
	}
}

// SetNotifier sets an optional notifier that sends a messenger message after successful linking.
func (l *Linker) SetNotifier(n Notifier) {
	l.notifier = n
}

// Link associates the TSU AccountId with the global user.
// If another user already owns this AccountId, it merges the current
// user's channel accounts into the existing user and deletes the orphan.
func (l *Linker) Link(ctx context.Context, currentUserID model.GlobalUserID, accountID string) error {
	existing, err := l.userRepo.FindByTsuAccountsID(ctx, accountID)
	if err != nil {
		return err
	}

	switch {
	case existing != nil && existing.ID == currentUserID:
		return l.linkSameUser(ctx, currentUserID, accountID)
	case existing != nil:
		return l.mergeInto(ctx, currentUserID, existing.ID, accountID)
	default:
		return l.linkNewAccount(ctx, currentUserID, accountID)
	}
}

func (l *Linker) linkSameUser(ctx context.Context, userID model.GlobalUserID, accountID string) error {
	l.autoLinkPerson(ctx, userID, accountID)
	if err := l.userRepo.SetTsuAccountsID(ctx, userID, accountID); err != nil {
		return err
	}
	l.notifyLinked(ctx, userID)
	return nil
}

func (l *Linker) linkNewAccount(ctx context.Context, userID model.GlobalUserID, accountID string) error {
	if err := l.userRepo.SetTsuAccountsID(ctx, userID, accountID); err != nil {
		return err
	}
	l.autoLinkPerson(ctx, userID, accountID)
	l.notifyLinked(ctx, userID)
	return nil
}

func (l *Linker) mergeInto(ctx context.Context, fromUserID, toUserID model.GlobalUserID, accountID string) error {
	accounts, err := l.accountRepo.FindByGlobalUserID(ctx, fromUserID)
	if err != nil {
		return err
	}
	for i := range accounts {
		accounts[i].GlobalUserID = toUserID
		if _, err := l.accountRepo.Save(ctx, &accounts[i]); err != nil {
			return err
		}
	}

	if err := l.userRepo.Delete(ctx, fromUserID); err != nil {
		l.logger.Warn("tsu linker: failed to delete orphaned user",
			slog.Int64("user_id", int64(fromUserID)),
			slog.Any("error", err))
	}

	l.autoLinkPerson(ctx, toUserID, accountID)
	l.notifyLinked(ctx, toUserID)

	l.logger.Info("tsu linker: merged accounts",
		slog.Int64("from_user", int64(fromUserID)),
		slog.Int64("to_user", int64(toUserID)),
		slog.Int("accounts_moved", len(accounts)))
	return nil
}

func (l *Linker) notifyLinked(ctx context.Context, userID model.GlobalUserID) {
	if l.notifier == nil {
		return
	}

	u, err := l.userRepo.FindByID(ctx, userID)
	locale := "ru"
	if err == nil && u != nil && u.Locale != "" {
		locale = u.Locale
	}

	accounts, err := l.accountRepo.FindByGlobalUserID(ctx, userID)
	if err != nil {
		l.logger.Warn("tsu linker: failed to get accounts for link notification",
			slog.Int64("user_id", int64(userID)),
			slog.Any("error", err))
		return
	}

	msg := model.NewTextMessage(i18n.Get("link.tsu_linked", locale))
	for _, acc := range accounts {
		if acc.ChannelType == model.ChannelWeb {
			continue
		}
		if err := l.notifier.SendToUser(ctx, acc.ChannelType, model.PlatformUserID(acc.ChannelUserID), msg); err != nil {
			l.logger.Warn("tsu linker: failed to send link notification",
				slog.Int64("user_id", int64(userID)),
				slog.String("channel", string(acc.ChannelType)),
				slog.Any("error", err))
		}
	}
}

func (l *Linker) autoLinkPerson(ctx context.Context, userID model.GlobalUserID, accountID string) {
	if err := l.personLinker.LinkByExternalID(ctx, userID, accountID); err != nil {
		l.logger.Warn("tsu linker: auto-link person failed",
			slog.Int64("user_id", int64(userID)),
			slog.String("external_id", accountID),
			slog.Any("error", err))
	}
}
