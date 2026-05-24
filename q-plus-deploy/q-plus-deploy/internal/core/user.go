package core

import (
	"q+/internal/generated/ent"
	"q+/internal/generated/ent/user"
	"q+/internal/generated/ent/useraccount"
	"strings"
)

type RegisterParams struct {
	ServerCommandParams
	DiscordId  string
	Surname    string
	Name       string
	Patronymic string
	Group      string
	Gmail      string
}

var Register = wrapTx(register)

// upsert user by discord id
func register(ctx UseCaseContext[RegisterParams]) (*ent.User, error) {
	u, err := ctx.getUserById(ctx.Params.DiscordId)

	if u == nil {
		u, err = ctx.ent().User.
			Create().
			SetSurname(ctx.Params.Surname).
			SetName(ctx.Params.Name).
			SetPatronymic(ctx.Params.Patronymic).
			SetGroup(ctx.Params.Group).
			SetDiscordID(ctx.Params.DiscordId).
			Save(ctx.Ctx)
		if err != nil {
			return nil, err
		}

		err = ctx.ent().UserAccount.
			CreateBulk(
				ctx.ent().UserAccount.Create().
					SetType(useraccount.TypeDiscord).
					SetAccountIdentifier(ctx.Params.DiscordId).
					SetUserID(u.ID),
				ctx.ent().UserAccount.Create().
					SetType(useraccount.TypeGmail).
					SetAccountIdentifier(ctx.Params.Gmail).
					SetUserID(u.ID),
			).Exec(ctx.Ctx)
		if err != nil {
			return nil, err
		}

		return u, nil
		//return u.Update().
		//	AddUserAccounts(accounts...).
		//	Save(ctx.Ctx)
	} else {
		_, err := ctx.ent().UserAccount.
			Delete().
			Where(
				useraccount.TypeEQ(useraccount.TypeGmail),
				useraccount.UserID(u.ID),
			).
			Exec(ctx.Ctx)
		if err != nil {
			return nil, err
		}
		err = ctx.ent().UserAccount.Create().
			SetType(useraccount.TypeGmail).
			SetAccountIdentifier(ctx.Params.Gmail).
			SetUserID(u.ID).
			Exec(ctx.Ctx)
		if err != nil {
			return nil, err
		}

		return u.Update().
			SetSurname(ctx.Params.Surname).
			SetName(ctx.Params.Name).
			SetPatronymic(ctx.Params.Patronymic).
			SetGroup(ctx.Params.Group).
			Save(ctx.Ctx)
	}
}

type IsUserRegisteredParams struct {
	UserDiscordId string
}

var IsUserRegistered = wrapTx(isUserRegistered)

func isUserRegistered(ctx UseCaseContext[IsUserRegisteredParams]) (bool, error) {
	u, err := ctx.ent().User.
		Query().
		Where(user.HasUserAccountsWith(
			useraccount.TypeEQ(useraccount.TypeDiscord),
			useraccount.AccountIdentifier(ctx.Params.UserDiscordId),
		)).
		Only(ctx.Ctx)
	if ent.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if u.Name == "" || u.Surname == "" || u.Patronymic == "" {
		return false, nil
	}

	return true, nil
}

func GetName(u *ent.User) string {
	return strings.TrimSpace(u.Surname + " " + u.Name)
}
