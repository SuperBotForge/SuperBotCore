package sender

import (
	"context"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/samber/do/v2"
	"github.com/samber/lo"
	coreDiscord "q+/internal/core/discord"
	"q+/internal/generated/ent"
	"q+/internal/utils"
)

type Sender struct {
	session         *discordgo.Session
	mentionRenderer coreDiscord.MentionRenderer
}

func NewSender(i do.Injector) (*Sender, error) {
	session := do.MustInvoke[*discordgo.Session](i)
	mentionRenderer := do.MustInvokeAs[coreDiscord.MentionRenderer](i)

	return &Sender{
		session:         session,
		mentionRenderer: mentionRenderer,
	}, nil
}

func (s *Sender) SendMessage(ctx context.Context, channelID string, message string) error {
	_, err := s.session.ChannelMessageSend(channelID, message, discordgo.WithContext(ctx))
	return err
}

func (s *Sender) SendStudentGuide(ctx context.Context, queue *ent.Queue, course *ent.CourseInstance, channels *ent.ChannelsForCourse) error {
	message := fmt.Sprintf(
		"## Открылась запись в очередь '%s'\n"+
			"Чтобы записаться в очередь, в текущем канале (<#%s>) введите слеш-команду %s. "+
			"Если требуется, введите примечание (параметр *note*), "+
			"и в случае командной сдачи укажите членов команды (*teammate-n*) (команду записывает в очередь только один человек).\n"+
			"После отправки слеш-команды появится выпадающий список, в котором нужно выбрать то что вы сдаете.\n"+
			"Если вы записались случайно, или записались не так как надо, можно удалить запись из очереди слеш-командой %s\n"+
			"Очередь можно посмотреть в таблице %s\n"+
			"Ожидайте, пока принимающий вызовет вас в канале <#%s>\n",
		queue.Name,
		channels.StudentChannelID,
		s.mentionRenderer.ClickableSlashCommand("sign-up"),
		s.mentionRenderer.ClickableSlashCommand("leave"),
		utils.CreateGoogleSheetsSheetLink(course.QueuesSpreadsheetID, lo.FromPtr(queue.SheetID)),
		channels.QueueChannelID,
	)
	return s.SendMessage(ctx, channels.StudentChannelID, message)
}

func (s *Sender) SendTeacherGuide(ctx context.Context, queue *ent.Queue, course *ent.CourseInstance, markTableTab *ent.MarkTableTab, channels *ent.ChannelsForCourse) error {
	message := fmt.Sprintf(
		"## Началась очередь '%s'\n"+
			"Чтобы записаться в очередь, в текущем канале (<#%s>) введите слеш-команду %s\n"+
			"Если вы принимаете очно, можно написать где вы находитесь в параметре **note**\n"+
			"После отправки слеш-команды появится выпадающий список, в котором нужно выбрать то, что вы принимаете.\n"+
			"*Очередь* можно посмотреть в этой таблице %s\n"+
			"*Оценки* можно посмотреть в этой таблице %s\n"+
			"\n**Дальнейшие слеш-команды нужно вводить в канале <#%s>**\n"+
			"Чтобы вызвать следующего студента/команду, используйте %s\n"+
			"Чтобы вызвать конкретного студента/команду, используйте %s\n"+
			"Перед тем, как вызвать следующего, нужно выставить оценку (или просто отметить что студент сдал критерий) с помощью %s\n"+
			"**Внимание**, при отметке, что студент сдал критерий, используйте **однозначно** интерпретируемый вердикт сдал/не сдал, например просто `+` и `-`\n"+
			"Если вы хотите сообщить о своем перерыве, воспользуйтесь слеш-командой %s\n"+
			"Чтобы повторно вызвать студента/команду, если они долго не приходят, воспользуйтесь %s",
		queue.Name,
		channels.TeacherChannelID,
		s.mentionRenderer.ClickableSlashCommand("queue teacher set"),
		utils.CreateGoogleSheetsSheetLink(course.QueuesSpreadsheetID, lo.FromPtr(queue.SheetID)),
		utils.CreateGoogleSheetsSheetLink(markTableTab.Edges.MarkTable.SpreadsheetID, markTableTab.SheetID),
		channels.QueueChannelID,
		s.mentionRenderer.ClickableSlashCommand("next"),
		s.mentionRenderer.ClickableSlashCommand("pick"),
		s.mentionRenderer.ClickableSlashCommand("set-mark"),
		s.mentionRenderer.ClickableSlashCommand("pause"),
		s.mentionRenderer.ClickableSlashCommand("reping"),
	)
	return s.SendMessage(ctx, channels.TeacherChannelID, message)
}

func (s *Sender) SendNewStudentInQueueNotify(ctx context.Context, examsUsers []*ent.User, team []*ent.User, channels *ent.ChannelsForCourse) error {
	message := fmt.Sprintf(
		"%s, у вас в очереди появился новый свободный студент/команда %s",
		utils.JoinUserPings(examsUsers),
		utils.JoinUserPings(team),
	)
	return s.SendMessage(ctx, channels.TeacherChannelID, message)
}
