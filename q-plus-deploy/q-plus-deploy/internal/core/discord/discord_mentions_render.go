package discord

type MentionRenderer interface {
	ClickableSlashCommand(command string) string
}
