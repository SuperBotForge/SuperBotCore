package handlers

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"q+/internal/core"
	"q+/internal/discord/oauth"
	"q+/internal/generated/ent/discordrole"
	"strings"
	"time"
)

func Setup(ctx InteractionContext, options OptionMap) error {
	ctx.log().Trace().Msg("Setup command")

	stateData := &oauth.StateData{
		CreatedAt: time.Now(),
		AppId:     ctx.I.AppID,
		GuildId:   ctx.I.GuildID,
	}
	authUrl := ctx.Oauth.AuthUrl(stateData)
	err := ctx.interactionCommandRespondEphemeral("нажми [сюда](" + authUrl + "), я точно не украду твой аккаунт")

	if err != nil {
		return err
	}
	return nil
}

func AddRole(ctx InteractionContext, options OptionMap) error {
	ctx.log().Trace().Msg("Add role command")

	role, err := core.AddDiscordRole(useCaseContext(ctx, core.AddDiscordRoleParams{
		ServerCommandParams: ctx.serverCommandParams(),
		Type:                discordrole.Type(options.String("type")),
		RoleId:              options.RoleId("role"),
	}))
	if err != nil {
		return err
	}

	discordRole := discordgo.Role{ID: role.ID}
	typeName := ""
	switch role.Type {
	case discordrole.TypeStudent:
		typeName = "Студенты"
	case discordrole.TypeExaminer:
		typeName = "Принимающие"
	}
	return ctx.interactionCommandRespondNoMentions("Теперь бот знает, что " + discordRole.Mention() + " это " + typeName)
}

func RemoveRole(ctx InteractionContext, options OptionMap) error {
	ctx.log().Trace().Msg("Remove role command")

	err := core.RemoveDiscordRole(useCaseContext(ctx, core.RemoveDiscordRoleParams{
		ServerCommandParams: ctx.serverCommandParams(),
		RoleId:              options.RoleId("role"),
	}))
	if err != nil {
		return err
	}

	discordRole := discordgo.Role{ID: options.RoleId("role")}
	return ctx.interactionCommandRespondNoMentions("Теперь бот забыл про роль " + discordRole.Mention())
}

func ListRoles(ctx InteractionContext, options OptionMap) error {
	ctx.log().Trace().Msg("List roles command")

	roles, err := core.ListDiscordRoles(useCaseContext(ctx, core.ListDiscordRolesParams{
		ServerCommandParams: ctx.serverCommandParams(),
	}))
	if err != nil {
		return err
	}

	if len(roles) == 0 {
		return ctx.interactionCommandRespondNoMentions("Бот пока не знает ни одной роли")
	}

	studentRoleMentions := make([]string, 0)
	examinerRoleMentions := make([]string, 0)
	for _, role := range roles {
		discordRole := discordgo.Role{ID: role.ID}
		switch role.Type {
		case discordrole.TypeStudent:
			studentRoleMentions = append(studentRoleMentions, discordRole.Mention())
		case discordrole.TypeExaminer:
			examinerRoleMentions = append(examinerRoleMentions, discordRole.Mention())
		}
	}

	return ctx.interactionCommandRespondNoMentions(fmt.Sprintf(
		"Бот знает следующие роли: \nСтуденты: %s\nПринимающие: %s",
		strings.Join(studentRoleMentions, ", "),
		strings.Join(examinerRoleMentions, ", "),
	))
}
