package interactions

import (
	"github.com/bwmarrin/discordgo"
	"q+/internal/discord/interactions/handlers"
	"testing"
)

func Test_buildHandler_execute_handler(t *testing.T) {
	tests := []struct {
		name string
		data discordgo.ApplicationCommandInteractionData
		want string
	}{
		{
			name: "Test command",
			data: discordgo.ApplicationCommandInteractionData{
				Name:        "next",
				CommandType: discordgo.ChatApplicationCommand,
			},
			want: "next",
		},
		{
			name: "Test subcommand",
			data: discordgo.ApplicationCommandInteractionData{
				Name:        "queue",
				CommandType: discordgo.ChatApplicationCommand,
				Options: []*discordgo.ApplicationCommandInteractionDataOption{
					{
						Name: "create",
						Type: discordgo.ApplicationCommandOptionSubCommand,
					},
				},
			},
			want: "queue create",
		},
		{
			name: "Test subcommand group",
			data: discordgo.ApplicationCommandInteractionData{
				Name:        "queue",
				CommandType: discordgo.ChatApplicationCommand,
				Options: []*discordgo.ApplicationCommandInteractionDataOption{
					{
						Name: "teacher",
						Type: discordgo.ApplicationCommandOptionSubCommandGroup,
						Options: []*discordgo.ApplicationCommandInteractionDataOption{
							{
								Name: "get",
								Type: discordgo.ApplicationCommandOptionSubCommand,
							},
						},
					},
				},
			},
			want: "queue teacher get",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCount, mockHandlerName := buildAndExecuteTestHandler(tt.data)
			if mockCount != 1 {
				t.Errorf("testHandler() mockCount = %v, want 1", mockCount)
			}
			if mockHandlerName != tt.want {
				t.Errorf("testHandler() mockHandlerName = %v, want %v", mockHandlerName, tt.want)
			}
		})
	}
}

func buildAndExecuteTestHandler(data discordgo.ApplicationCommandInteractionData) (int, string) {
	var mockCount int
	var mockHandlerName string

	createTestHandler := func(name string) handlers.CommandHandler {
		return func(ctx handlers.InteractionContext, options handlers.OptionMap) error {
			mockCount++
			mockHandlerName = name
			return nil
		}
	}

	testCommands := createTestCommands(createTestHandler)
	testHandler := buildHandler(testCommands)

	ctx := handlers.InteractionContext{
		I: &discordgo.Interaction{
			Type: discordgo.InteractionApplicationCommand,
			Data: data,
		},
		Options: data.Options,
	}

	_ = testHandler(ctx)

	return mockCount, mockHandlerName
}

func createTestCommands(createTestHandler func(name string) handlers.CommandHandler) []builder {
	return []builder{
		commandF(createTestHandler("next"), &discordgo.ApplicationCommand{
			Name:        "next",
			Description: "Вызвать следующего из текущей очереди",
		}),
		command(&discordgo.ApplicationCommand{
			Name:        "queue",
			Description: "Очередь",
		},
			subcommandF(createTestHandler("queue create"), &discordgo.ApplicationCommandOption{
				Name:        "create",
				Description: "Создать очередь",
				Options:     []*discordgo.ApplicationCommandOption{},
			}),
			subcommandGroup(&discordgo.ApplicationCommandOption{
				Name:        "teacher",
				Description: "Принимающий",
			},
				subcommandF(createTestHandler("queue teacher get"), &discordgo.ApplicationCommandOption{
					Name:        "get",
					Description: "Информация о принимающем",
					Options:     []*discordgo.ApplicationCommandOption{},
				}),
			),
		),
	}
}

func Test_buildHandler_execute_handler_with_options(t *testing.T) {
	// TODO
}

func Test_buildHandler_accept_application_command(t *testing.T) {
	// TODO
}

func Test_buildHandler_accept_application_command_autocomplete(t *testing.T) {
	// TODO
}

func Test_buildHandler_accept_message_component(t *testing.T) {
	// TODO
}

func Test_buildHandler_accept_modal_submit(t *testing.T) {
	// TODO
}
