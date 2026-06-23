package authz

import (
	"context"

	"SuperBotGo/internal/model"
)

type Store interface {
	GetRoles(ctx context.Context, userID model.GlobalUserID, roleType model.RoleLayer) ([]model.UserRole, error)
	GetAllRoleNames(ctx context.Context, userID model.GlobalUserID) ([]string, error)

	GetCommandPolicy(ctx context.Context, pluginID, commandName string) (enabled bool, policyExpr string, found bool, err error)
	GetPluginPolicy(ctx context.Context, pluginID string) (string, error)

	GetExternalID(ctx context.Context, userID model.GlobalUserID) (string, error)
	GetUserChannelAndLocale(ctx context.Context, userID model.GlobalUserID) (primaryChannel, locale string, err error)

	GetDistinctRoleNames(ctx context.Context) []string
}
