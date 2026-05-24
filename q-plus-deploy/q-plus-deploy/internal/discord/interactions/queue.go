package interactions

import (
	"github.com/bwmarrin/discordgo"
	"q+/internal/core"
	"q+/internal/discord/interactions/handlers"
)

var queueCommands = []builder{
	commandF(core.ChannelQueue, handlers.Next, &discordgo.ApplicationCommand{
		Name:        "next",
		Description: "Вызвать следующего из текущей очереди",
	}),
	commandF(core.ChannelQueue, handlers.Pick, &discordgo.ApplicationCommand{
		Name:        "pick",
		Description: "Вызвать конкретного студента/команду из очереди",
	},
		autocompleteOption(handlers.AutocompleteQueuePlacesForExaminer, &discordgo.ApplicationCommandOption{
			Name:        "place",
			Description: "Запись в очередь",
			Type:        discordgo.ApplicationCommandOptionInteger,
			Required:    true,
		}),
	),
	commandF(core.ChannelQueue, handlers.Reping, &discordgo.ApplicationCommand{
		Name:        "reping",
		Description: "Вызвать студента/команду еще разок",
	}),
	commandF(core.ChannelQueue, handlers.Pause, &discordgo.ApplicationCommand{
		Name:        "pause",
		Description: "Отпустить текущую команду и сообщить о своем перерыве",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Name:        "duration",
				Description: "Длительность (примерно) (в мин)",
				Type:        discordgo.ApplicationCommandOptionInteger,
				Required:    true,
			},
		},
	}),
	commandF(core.ChannelQueue|core.ChannelStudent, handlers.StartSignUp, &discordgo.ApplicationCommand{
		Name:        "start-sign-up",
		Description: "Начать запись в очередь",
	},
		autocompleteOption(handlers.AutocompleteQueueList, &discordgo.ApplicationCommandOption{
			Name:        "queue",
			Description: "Очередь",
			Type:        discordgo.ApplicationCommandOptionInteger,
			Required:    true,
		}),
	),
	commandF(core.ChannelQueue, handlers.StartQueue, &discordgo.ApplicationCommand{
		Name:        "start-queue",
		Description: "Начать очередь",
	},
		autocompleteOption(handlers.AutocompleteQueueList, &discordgo.ApplicationCommandOption{
			Name:        "queue",
			Description: "Очередь",
			Type:        discordgo.ApplicationCommandOptionInteger,
			Required:    true,
		}),
	),
	commandF(core.ChannelQueue, handlers.EndQueue, &discordgo.ApplicationCommand{
		Name:        "end-queue",
		Description: "Закончить очередь",
	}),
	commandF(core.ChannelStudent, handlers.SignUp, &discordgo.ApplicationCommand{
		Name:        "sign-up",
		Description: "Записаться в очередь",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Name:        "note",
				Description: "Заметка (например, что вы хотите сдать)",
				Type:        discordgo.ApplicationCommandOptionString,
			},
			{
				Name:        "teammate_2",
				Description: "Участник команды №2",
				Type:        discordgo.ApplicationCommandOptionUser,
			},
			{
				Name:        "teammate_3",
				Description: "Участник команды №3",
				Type:        discordgo.ApplicationCommandOptionUser,
			},
			{
				Name:        "teammate_4",
				Description: "Участник команды №4",
				Type:        discordgo.ApplicationCommandOptionUser,
			},
			{
				Name:        "teammate_5",
				Description: "Участник команды №5",
				Type:        discordgo.ApplicationCommandOptionUser,
			},
			{
				Name:        "teammate_6",
				Description: "Участник команды №6",
				Type:        discordgo.ApplicationCommandOptionUser,
			},
		},
	},
		autocompleteOption(handlers.AutocompleteSignUpStartedQueueList, &discordgo.ApplicationCommandOption{
			Name:        "queue",
			Description: "Очередь",
			Type:        discordgo.ApplicationCommandOptionInteger,
			Required:    true,
		}),
	),
	commandF(core.ChannelStudent, handlers.Leave, &discordgo.ApplicationCommand{
		Name:        "leave",
		Description: "Удалить запись из очереди",
	},
		autocompleteOption(handlers.AutocompleteSignUpStartedQueueList, &discordgo.ApplicationCommandOption{
			Name:        "queue",
			Description: "Очередь",
			Type:        discordgo.ApplicationCommandOptionInteger,
			Required:    true,
		}),
		autocompleteOption(handlers.AutocompleteQueuePlacesForCurrentUser, &discordgo.ApplicationCommandOption{
			Name:        "place",
			Description: "Запись в очередь (поиск по имени студента или по критерию)",
			Type:        discordgo.ApplicationCommandOptionInteger,
			Required:    true,
		}),
	),
	command(core.ChannelTeacher, &discordgo.ApplicationCommand{
		Name:        "queue",
		Description: "Очередь",
	},
		subcommandF(handlers.QueueCreate, &discordgo.ApplicationCommandOption{
			Name:        "create",
			Description: "Создать очередь",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "name",
					Description: "Название",
					Type:        discordgo.ApplicationCommandOptionString,
				},
				{
					Name:        "start",
					Description: "Время начала (yyyy-mm-dd hh:mm)",
					Type:        discordgo.ApplicationCommandOptionString,
				},
				{
					Name:        "end",
					Description: "Время окончания (yyyy-mm-dd hh:mm)",
					Type:        discordgo.ApplicationCommandOptionString,
				},
				{
					Name:        "sign_up_lead_time",
					Description: "За сколько времени до начала очереди открывать запись (в формате 1d2h30m)",
					Type:        discordgo.ApplicationCommandOptionString,
				},
			},
		},
			autocompleteOption(handlers.AutocompleteQueueTemplateList, &discordgo.ApplicationCommandOption{
				Name:        "template",
				Description: "Шаблон очереди",
				Type:        discordgo.ApplicationCommandOptionInteger,
				Required:    true,
			}),
		),
		subcommandF(handlers.QueueList, &discordgo.ApplicationCommandOption{
			Name:        "list",
			Description: "Список всех очередей у шаблона",
		},
			autocompleteOption(handlers.AutocompleteQueueTemplateList, &discordgo.ApplicationCommandOption{
				Name:        "template",
				Description: "Шаблон очереди",
				Type:        discordgo.ApplicationCommandOptionInteger,
				Required:    true,
			}),
		),
		subcommandF(handlers.QueueEdit, &discordgo.ApplicationCommandOption{
			Name:        "edit",
			Description: "Отредактировать очередь",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "name",
					Description: "Название",
					Type:        discordgo.ApplicationCommandOptionString,
				},
				{
					Name:        "start",
					Description: "Время начала (yyyy-mm-dd hh:mm), для удаления введите минус",
					Type:        discordgo.ApplicationCommandOptionString,
				},
				{
					Name:        "end",
					Description: "Время окончания (yyyy-mm-dd hh:mm), для удаления введите минус",
					Type:        discordgo.ApplicationCommandOptionString,
				},
				{
					Name:        "sign_up_lead_time",
					Description: "За сколько времени до начала очереди открывать запись (в формате 1d2h30m)",
					Type:        discordgo.ApplicationCommandOptionString,
				},
			},
		},
			autocompleteOption(handlers.AutocompleteQueueList, &discordgo.ApplicationCommandOption{
				Name:        "queue",
				Description: "Очередь",
				Type:        discordgo.ApplicationCommandOptionInteger,
				Required:    true,
			}),
		),
		subcommandGroup(&discordgo.ApplicationCommandOption{
			Name:        "criterion",
			Description: "Критерии",
		},
			subcommandF(handlers.QueueSelectCriteria, &discordgo.ApplicationCommandOption{
				Name:        "select",
				Description: "Выбрать критерии для очереди из списка критериев предмета",
			},
				autocompleteOption(handlers.AutocompleteQueueList, &discordgo.ApplicationCommandOption{
					Name:        "queue",
					Description: "Очередь",
					Type:        discordgo.ApplicationCommandOptionInteger,
					Required:    true,
				}),
			),
		),
		subcommandGroup(&discordgo.ApplicationCommandOption{
			Name:        "teacher",
			Description: "Принимающий",
		},
			subcommandF(handlers.QueueTeacherSet, &discordgo.ApplicationCommandOption{
				Name:        "set",
				Description: "Прикрепить к очереди принимающего",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        "note",
						Description: "Заметка, которую увидят студенты (например ваш кабинет при сдаче очно). Для удаления введите минус",
						Type:        discordgo.ApplicationCommandOptionString,
					},
					{
						Name:        "teacher",
						Description: "Кого вы хотите прикрепить (по умолчанию - вы)",
						Type:        discordgo.ApplicationCommandOptionUser,
					},
				},
			},
				autocompleteOption(handlers.AutocompleteNotEndedQueueList, &discordgo.ApplicationCommandOption{
					Name:        "queue",
					Description: "Очередь",
					Type:        discordgo.ApplicationCommandOptionInteger,
					Required:    true, // TODO default current
				}),
			),
			subcommandF(handlers.QueueTeacherDelete, &discordgo.ApplicationCommandOption{
				Name:        "delete",
				Description: "Открепить принимающего от очереди",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        "teacher",
						Description: "Кого вы хотите открепить (по умолчанию - вы)",
						Type:        discordgo.ApplicationCommandOptionUser,
					},
				},
			},
				autocompleteOption(handlers.AutocompleteNotEndedQueueList, &discordgo.ApplicationCommandOption{
					Name:        "queue",
					Description: "Очередь",
					Type:        discordgo.ApplicationCommandOptionInteger,
					Required:    true, // TODO default current
				}),
			),
			//subcommandF(handlers.QueueTeacherGet, &discordgo.ApplicationCommandOption{
			//	Name:        "get",
			//	Description: "Информация о принимающем",
			//	Options:     []*discordgo.ApplicationCommandOption{},
			//}),
		),
	),
}
