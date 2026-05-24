package handlers

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/samber/lo"
	"q+/internal/core"
	"q+/internal/generated/ent"
	"q+/internal/generated/ent/discordrole"
	"strings"
)

func AddChannelsForCourse(ctx InteractionContext, options OptionMap) error {
	channels, err := core.CreateChannelsForCourse(useCaseContext(ctx, core.CreateChannelsForCourseParams{
		ServerCommandParams: ctx.serverCommandParams(),
		Name:                options.String("name"),
		QueueChannelId:      options.ChannelId("queue_channel"),
		StudentChannelId:    options.ChannelId("student_channel"),
		TeacherChannelId:    options.ChannelId("teacher_channel"),
	}))
	if err != nil {
		return err
	}

	return printAllChannels(ctx, channels)
}

func CreateChannelsForCourse(ctx InteractionContext, options OptionMap) error {
	name := options.String("name")

	roles, err := core.ListDiscordRoles(useCaseContext(ctx, core.ListDiscordRolesParams{
		ServerCommandParams: ctx.serverCommandParams(),
	}))
	if err != nil {
		return err
	}

	studentRoles := make([]*ent.DiscordRole, 0)
	examinerRoles := make([]*ent.DiscordRole, 0)
	for _, role := range roles {
		switch role.Type {
		case discordrole.TypeStudent:
			studentRoles = append(studentRoles, role)
		case discordrole.TypeExaminer:
			examinerRoles = append(examinerRoles, role)
		}
	}

	category, err := ctx.S.GuildChannelCreate(ctx.I.GuildID, name, discordgo.ChannelTypeGuildCategory, discordgo.WithContext(ctx.Ctx))
	if err != nil {
		return err
	}

	queueChannel, err := ctx.S.GuildChannelCreateComplex(ctx.I.GuildID, discordgo.GuildChannelCreateData{
		Name:     "очередь",
		Type:     discordgo.ChannelTypeGuildText,
		ParentID: category.ID,
	})
	if err != nil {
		return err
	}

	studentChannel, err := ctx.S.GuildChannelCreateComplex(ctx.I.GuildID, discordgo.GuildChannelCreateData{
		Name:     "запись в очередь",
		Type:     discordgo.ChannelTypeGuildText,
		ParentID: category.ID,
	})
	if err != nil {
		return err
	}

	teacherPermissions := []*discordgo.PermissionOverwrite{
		{
			ID:   ctx.I.GuildID, // @everyone
			Type: discordgo.PermissionOverwriteTypeRole,
			Deny: discordgo.PermissionViewChannel,
		},
	}
	for _, role := range examinerRoles {
		teacherPermissions = append(teacherPermissions, &discordgo.PermissionOverwrite{
			ID:    role.ID,
			Type:  discordgo.PermissionOverwriteTypeRole,
			Allow: discordgo.PermissionViewChannel,
		})
	}
	teacherChannel, err := ctx.S.GuildChannelCreateComplex(ctx.I.GuildID, discordgo.GuildChannelCreateData{
		Name:                 "q-plus преподская",
		Type:                 discordgo.ChannelTypeGuildText,
		ParentID:             category.ID,
		PermissionOverwrites: teacherPermissions,
	})
	if err != nil {
		return err
	}

	channels, err := core.CreateChannelsForCourse(useCaseContext(ctx, core.CreateChannelsForCourseParams{
		ServerCommandParams: ctx.serverCommandParams(),
		Name:                name,
		QueueChannelId:      queueChannel.ID,
		StudentChannelId:    studentChannel.ID,
		TeacherChannelId:    teacherChannel.ID,
	}))
	if err != nil {
		return err
	}

	return printAllChannels(ctx, channels)
}

func printAllChannels(ctx InteractionContext, newChannels *ent.ChannelsForCourse) error {
	channelsForCourses, err := core.ListChannelsForCourse(useCaseContext(ctx, core.ListChannelsForCourseParams{
		ServerCommandParams: ctx.serverCommandParams(),
	}))
	if err != nil {
		return err
	}

	//output all newChannels for courses
	channelsForCoursesList := strings.Join(lo.Map(channelsForCourses, func(channelsForCourse *ent.ChannelsForCourse, _ int) string {
		return fmt.Sprintf("'%s', обновлен %s", channelsForCourse.Name, channelsForCourse.UpdatedAt.Format("2006-01-02 15:04"))
	}), "\n")
	return ctx.interactionCommandRespond(fmt.Sprintf("### Набор каналов '%s' создан\nНаборы на текущем сервере:\n%s", newChannels.Name, channelsForCoursesList))
}
