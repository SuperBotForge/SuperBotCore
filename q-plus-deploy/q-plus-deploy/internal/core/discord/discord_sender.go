package discord

import (
	"context"
	"q+/internal/generated/ent"
)

type Sender interface {
	SendMessage(ctx context.Context, channelID string, message string) error

	SendStudentGuide(ctx context.Context, queue *ent.Queue, course *ent.CourseInstance, channels *ent.ChannelsForCourse) error
	SendTeacherGuide(ctx context.Context, queue *ent.Queue, course *ent.CourseInstance, markTableTab *ent.MarkTableTab, channels *ent.ChannelsForCourse) error

	SendNewStudentInQueueNotify(ctx context.Context, examsUsers []*ent.User, team []*ent.User, channels *ent.ChannelsForCourse) error
}
