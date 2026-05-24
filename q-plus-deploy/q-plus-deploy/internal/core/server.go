package core

import (
	"q+/internal/generated/ent"
	"q+/internal/generated/ent/channelsforcourse"
	"q+/internal/generated/ent/discordrole"
	"q+/internal/generated/ent/discordserver"
)

type AddDiscordRoleParams struct {
	ServerCommandParams
	Type   discordrole.Type
	RoleId string
}

var AddDiscordRole = wrapTx(addDiscordRole)

func addDiscordRole(ctx UseCaseContext[AddDiscordRoleParams]) (*ent.DiscordRole, error) {
	err := ctx.createDiscordServer(ctx.Params.DiscordServerID)
	if err != nil {
		return nil, err
	}

	return ctx.ent().DiscordRole.
		Create().
		SetID(ctx.Params.RoleId).
		SetType(ctx.Params.Type).
		SetDiscordServerID(ctx.Params.DiscordServerID).
		Save(ctx.Ctx)
}

type RemoveDiscordRoleParams struct {
	ServerCommandParams
	RoleId string
}

var RemoveDiscordRole = wrapTx0(removeDiscordRole)

func removeDiscordRole(ctx UseCaseContext[RemoveDiscordRoleParams]) error {
	return ctx.ent().DiscordRole.
		DeleteOneID(ctx.Params.RoleId).
		Exec(ctx.Ctx)
}

type SetupPermissionsParams struct {
	GuildId string
}

type SetupPermissionsResponse struct {
	Channels []*ent.ChannelsForCourse
	Roles    []*ent.DiscordRole
}

var SetupPermissions = wrapTx(setupPermissions)

func setupPermissions(ctx UseCaseContext[SetupPermissionsParams]) (*SetupPermissionsResponse, error) {
	channels, err := ctx.ent().ChannelsForCourse.
		Query().
		Where(
			channelsforcourse.HasDiscordServerWith(discordserver.ID(ctx.Params.GuildId)),
		).
		All(ctx.Ctx)
	if err != nil {
		return nil, err
	}

	roles, err := ctx.ent().DiscordRole.
		Query().
		Where(
			discordrole.HasDiscordServerWith(discordserver.ID(ctx.Params.GuildId)),
		).
		All(ctx.Ctx)

	return &SetupPermissionsResponse{
		Channels: channels,
		Roles:    roles,
	}, nil
}

type ListDiscordRolesParams struct {
	ServerCommandParams
}

var ListDiscordRoles = wrapTx(listDiscordRoles)

func listDiscordRoles(ctx UseCaseContext[ListDiscordRolesParams]) ([]*ent.DiscordRole, error) {
	err := ctx.createDiscordServer(ctx.Params.DiscordServerID)
	if err != nil {
		return nil, err
	}

	return ctx.ent().DiscordRole.
		Query().
		Where(
			discordrole.HasDiscordServerWith(discordserver.ID(ctx.Params.DiscordServerID)),
		).
		All(ctx.Ctx)
}
