package web

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog"
	"github.com/samber/do/v2"
	"github.com/samber/lo"
	"github.com/urfave/negroni"
	"net/http"
	"q+/internal/core"
	"q+/internal/discord"
	"q+/internal/discord/interactions"
	"q+/internal/discord/oauth"
	"q+/internal/generated/ent"
	"q+/internal/generated/ent/discordrole"
	"slices"
	"strconv"
	"time"
)

type OauthHandler struct {
	oauth   *oauth.Oauth
	core    *core.Core
	discord *discordgo.Session
}

func NewOauthHandler(i do.Injector) (*OauthHandler, error) {
	c := do.MustInvoke[*core.Core](i)
	o := do.MustInvoke[*oauth.Oauth](i)
	d, err := discordgo.New(discordgo.BasicToken(o.Config.ClientID, o.Config.ClientSecret))
	if err != nil {
		return nil, err
	}
	return &OauthHandler{
		oauth:   o,
		core:    c,
		discord: d,
	}, nil
}

func (h *OauthHandler) CreateOauthHandler() negroni.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		if r.URL.Path == h.oauth.RedirectPath() {
			err := h.HandleDiscordAuth(w, r)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(err.Error()))
				return
			}
		} else {
			// Fallback to the next handler
			next.ServeHTTP(w, r)
		}
	}
}

func (h *OauthHandler) HandleDiscordAuth(w http.ResponseWriter, r *http.Request) error {
	logger := zerolog.Ctx(r.Context())
	rc := http.NewResponseController(w)
	code := r.FormValue("code")
	state := r.FormValue("state")

	stateData, has := h.oauth.States[state]
	if !has {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("OAuth2 state not found"))
		return nil
	}
	delete(h.oauth.States, state)

	fiveMinutesAgo := time.Now().Add(-5 * time.Minute)
	if stateData.CreatedAt.Before(fiveMinutesAgo) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("OAuth2 state is over 5 minutes old"))
		return nil
	}

	accessToken, err := h.discord.AccessToken(code, h.oauth.RedirectUrl(), discordgo.WithContext(r.Context()))
	if err != nil {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Error while getting/processing access token request\n" + err.Error()))
		return nil
	}

	userSession, err := accessToken.NewSession()
	if err != nil {
		return err
	}

	user, err := userSession.User("@me", discordgo.WithContext(r.Context()))
	if err != nil {
		return err
	}

	logger.Info().
		Str("event", "discord_auth").
		Str("user_id", user.ID).
		Str("username", user.Username).
		Str("discriminator", user.Discriminator).
		Msg("User authenticated")

	setupPermissionsResponse, err := core.SetupPermissions(useCaseContext_(r.Context(), h, core.SetupPermissionsParams{
		GuildId: stateData.GuildId,
	}))
	if err != nil {
		return err
	}

	applicationCommandsPermissions := BuildApplicationCommandsPermissions(stateData.GuildId, stateData.AppId, setupPermissionsResponse)

	w.WriteHeader(200)
	w.Write([]byte("Ура, я украл твой аккаунт, " + user.Username))
	w.Write([]byte("\n\n"))
	err = rc.Flush()
	if err != nil {
		logger.Error().
			Str("event", "flush_error").
			Err(err).
			Msg("Error while flushing response")
	}

	for cmdId, permissionsList := range applicationCommandsPermissions {
		err = userSession.ApplicationCommandPermissionsEdit(
			stateData.AppId,
			stateData.GuildId,
			cmdId,
			&permissionsList,
			discordgo.WithContext(r.Context()),
		)
		if err != nil {
			return err
		}

		logger.Debug().
			Str("event", "discord_cmd_permissions_edit_progress").
			Str("cmd_id", cmdId).
			Int("permissions_count", len(permissionsList.Permissions)).
			Msg("Permissions set")

		w.Write([]byte(fmt.Sprintf("Установлено еще %d права на команду %s", len(permissionsList.Permissions), cmdId)))
		w.Write([]byte("\n...\n"))
		err = rc.Flush()
		if err != nil {
			logger.Error().
				Str("event", "flush_error").
				Err(err).
				Msg("Error while flushing response")
		}
	}

	w.Write([]byte(fmt.Sprintf("\n\nУстановлено %d прав на слеш-команды", len(applicationCommandsPermissions))))
	w.Write([]byte("\n\n"))
	w.Write([]byte("Теперь ты можешь закрыть эту вкладку"))

	return nil
}

func BuildApplicationCommandsPermissions(guildId string, appId string, permissionsData *core.SetupPermissionsResponse) map[string]discordgo.ApplicationCommandPermissionsList { // cmdID -> permissions
	allChannelInt, err := strconv.ParseUint(guildId, 10, 64)
	if err != nil {
		allChannelInt = 0
	}
	allChannelId := strconv.FormatUint(allChannelInt-1, 10)

	everyoneId := guildId

	result := make(map[string]discordgo.ApplicationCommandPermissionsList)
	for _, commandBuilder := range interactions.AllCommandBuilders {
		cmdID := discord.GetUploadedCommandId(commandBuilder.ApplicationCommandName())

		var list []*discordgo.ApplicationCommandPermissions

		studentRoles := mapRolesToAppCommandPermissions(permissionsData.Roles, discordrole.TypeStudent)

		examinerRoles := mapRolesToAppCommandPermissions(permissionsData.Roles, discordrole.TypeExaminer)

		switch commandBuilder.Permission() {
		case core.PermissionStudent:
			list = slices.Concat(list, studentRoles)
		case core.PermissionExaminer:
			list = slices.Concat(list, examinerRoles)
		case core.PermissionAnyone:
			list = slices.Concat(list, studentRoles, examinerRoles)
		case core.PermissionAdmin:
			// admin anyway has all permissions
		}

		switch commandBuilder.ChannelType() {
		case core.ChannelQueue:
			list = slices.Concat(list, mapChannelsToAppCommandPermissions(permissionsData.Channels, func(course *ent.ChannelsForCourse) string {
				return course.QueueChannelID
			}))
		case core.ChannelStudent:
			list = slices.Concat(list, mapChannelsToAppCommandPermissions(permissionsData.Channels, func(course *ent.ChannelsForCourse) string {
				return course.StudentChannelID
			}))
		case core.ChannelTeacher:
			list = slices.Concat(list, mapChannelsToAppCommandPermissions(permissionsData.Channels, func(course *ent.ChannelsForCourse) string {
				return course.TeacherChannelID
			}))
		case core.ChannelAny:
			list = append(list, &discordgo.ApplicationCommandPermissions{
				ID:         allChannelId,
				Type:       discordgo.ApplicationCommandPermissionTypeChannel,
				Permission: false,
			})
		case core.ChannelAll:
			list = slices.Concat(list,
				mapChannelsToAppCommandPermissions(permissionsData.Channels, func(course *ent.ChannelsForCourse) string {
					return course.QueueChannelID
				}),
				mapChannelsToAppCommandPermissions(permissionsData.Channels, func(course *ent.ChannelsForCourse) string {
					return course.StudentChannelID
				}),
				mapChannelsToAppCommandPermissions(permissionsData.Channels, func(course *ent.ChannelsForCourse) string {
					return course.TeacherChannelID
				}),
			)
		}

		result[cmdID] = discordgo.ApplicationCommandPermissionsList{Permissions: list}
		//discordgo.ApplicationCommandPermissionsList{
		//	// allowed channels
		//	// allowed roles
		//}
	}

	// all commands
	result[appId] = discordgo.ApplicationCommandPermissionsList{
		Permissions: []*discordgo.ApplicationCommandPermissions{
			{
				// @everyone
				ID:         everyoneId,
				Type:       discordgo.ApplicationCommandPermissionTypeRole,
				Permission: false,
			},
			{
				// All channels
				ID:         allChannelId,
				Type:       discordgo.ApplicationCommandPermissionTypeChannel,
				Permission: false,
			},
		},
	}

	return result
}

func mapChannelsToAppCommandPermissions(channels []*ent.ChannelsForCourse, transform func(course *ent.ChannelsForCourse) string) []*discordgo.ApplicationCommandPermissions {
	return lo.Map(channels, func(c *ent.ChannelsForCourse, _ int) *discordgo.ApplicationCommandPermissions {
		return &discordgo.ApplicationCommandPermissions{
			ID:         transform(c),
			Type:       discordgo.ApplicationCommandPermissionTypeChannel,
			Permission: true,
		}
	})
}

func mapRolesToAppCommandPermissions(roles []*ent.DiscordRole, rType discordrole.Type) []*discordgo.ApplicationCommandPermissions {
	filtered := lo.Filter(roles, func(role *ent.DiscordRole, _ int) bool {
		return role.Type == rType
	})
	return lo.Map(filtered, func(r *ent.DiscordRole, _ int) *discordgo.ApplicationCommandPermissions {
		return &discordgo.ApplicationCommandPermissions{
			ID:         r.ID,
			Type:       discordgo.ApplicationCommandPermissionTypeRole,
			Permission: true,
		}
	})
}
