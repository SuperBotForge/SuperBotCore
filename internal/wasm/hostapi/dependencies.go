package hostapi

import (
	"context"
	"net/http"

	"SuperBotGo/internal/filestore"
	"SuperBotGo/internal/model"
)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type EventBus interface {
	Publish(ctx context.Context, topic string, payload []byte) error
}

type PluginRegistry interface {
	CallPlugin(ctx context.Context, target string, method string, params []byte) ([]byte, error)
}

type Notifier interface {
	NotifyUser(ctx context.Context, userID int64, text string, priority int) error
	NotifyUsers(ctx context.Context, userIDs []int64, msg model.Message, priority int) error
	NotifyTeacher(ctx context.Context, ref model.TeacherRef, msg model.Message, priority int) error
	NotifyChat(ctx context.Context, channelType string, chatID string, text string, priority int) error
	NotifyStudents(ctx context.Context, scope string, targetID int64, msg model.Message, priority int) error
}

type UserProvider interface {
	GetUserInfo(ctx context.Context, userID int64) (*model.UserInfo, error)
	GetUsersInfo(ctx context.Context, userIDs []int64) ([]model.UserInfoFull, error)
}

type Dependencies struct {
	HTTP           HTTPClient
	Events         EventBus
	PluginRegistry PluginRegistry
	Notifier       Notifier
	FileStore      filestore.FileStore
	UserProvider   UserProvider
}
