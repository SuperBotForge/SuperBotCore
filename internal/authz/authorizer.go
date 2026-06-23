package authz

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"SuperBotGo/internal/metrics"
	"golang.org/x/sync/errgroup"

	v1 "github.com/authzed/authzed-go/proto/authzed/api/v1"
	authzed "github.com/authzed/authzed-go/v1"

	"SuperBotGo/internal/model"
)

const (
	DefaultSubjectCacheTTL = 30 * time.Second
	DefaultPolicyCacheTTL  = 60 * time.Second
)

type commandPolicyKey struct {
	pluginID    string
	commandName string
}

type commandPolicyValue struct {
	enabled    bool
	policyExpr string
	found      bool
}

type Authorizer struct {
	store        Store
	client       *authzed.Client // Клиент SpiceDB
	providers    []AttributeProvider
	logger       *slog.Logger
	subjectCache *TTLCache[model.GlobalUserID, *SubjectContext]
	policyCache  *TTLCache[commandPolicyKey, commandPolicyValue]
	metrics      *metrics.Metrics
}

// NewAuthorizer создает новый авторизатор с клиентом SpiceDB
func NewAuthorizer(store Store, client *authzed.Client, logger *slog.Logger, providers ...AttributeProvider) *Authorizer {
	return NewAuthorizerWithTTL(store, client, logger, DefaultSubjectCacheTTL, DefaultPolicyCacheTTL, providers...)
}

func NewAuthorizerWithTTL(store Store, client *authzed.Client, logger *slog.Logger, subjectTTL, policyTTL time.Duration, providers ...AttributeProvider) *Authorizer {
	if logger == nil {
		logger = slog.Default()
	}

	var sc *TTLCache[model.GlobalUserID, *SubjectContext]
	if subjectTTL > 0 {
		sc = NewTTLCache[model.GlobalUserID, *SubjectContext](subjectTTL)
	}

	var pc *TTLCache[commandPolicyKey, commandPolicyValue]
	if policyTTL > 0 {
		pc = NewTTLCache[commandPolicyKey, commandPolicyValue](policyTTL)
	}

	return &Authorizer{
		store:        store,
		client:       client,
		providers:    providers,
		logger:       logger,
		subjectCache: sc,
		policyCache:  pc,
	}
}

func (a *Authorizer) SetMetrics(metricSet *metrics.Metrics) {
	a.metrics = metricSet
}

func (a *Authorizer) CheckCommand(
	ctx context.Context,
	userID model.GlobalUserID,
	pluginID string,
	commandName string,
	requirements *model.RoleRequirements,
) (bool, error) {
	start := time.Now()
	result := "allow"
	defer func() {
		if a.metrics == nil {
			return
		}
		a.metrics.AuthzCheckDuration.WithLabelValues(pluginID, commandName, result).Observe(time.Since(start).Seconds())
	}()

	// 1. Проверка требований к ролям через SpiceDB
	if requirements != nil {
		ok, err := a.checkRolesSpice(ctx, userID, requirements)
		if err != nil {
			result = "error"
			return false, err
		}
		if !ok {
			result = "deny"
			return false, nil
		}
	}

	// 2. Проверка политики команды (осталась в Store, так как это конфиг плагина)
	if pluginID == "" {
		return true, nil
	}

	enabled, policyExpr, found, err := a.getCommandPolicy(ctx, pluginID, commandName)
	if err != nil {
		result = "error"
		return false, err
	}
	if found && !enabled {
		result = "deny"
		return false, nil
	}

	pluginPolicyExpr, err := a.getPluginPolicy(ctx, pluginID)
	if err != nil {
		result = "error"
		return false, err
	}

	combined := combinePolicies(pluginPolicyExpr, policyExpr)
	if combined != "" {
		ok, evalErr := a.EvalPolicy(ctx, combined, userID)
		if evalErr != nil {
			result = "error"
			a.logger.Warn("policy expression error",
				slog.String("plugin", pluginID),
				slog.String("command", commandName),
				slog.Any("error", evalErr))
			return false, nil
		}
		if !ok {
			result = "deny"
		}
		return ok, nil
	}

	return true, nil
}

func combinePolicies(pluginPolicy, commandPolicy string) string {
	p := pluginPolicy != ""
	c := commandPolicy != ""
	switch {
	case p && c:
		return "(" + pluginPolicy + ") && (" + commandPolicy + ")"
	case p:
		return pluginPolicy
	case c:
		return commandPolicy
	default:
		return ""
	}
}

func (a *Authorizer) CheckAccess(ctx context.Context, userID model.GlobalUserID, _ *model.GlobalUser, req *model.RoleRequirements) (bool, error) {
	return a.checkRolesSpice(ctx, userID, req)
}

func (a *Authorizer) CanExecute(ctx context.Context, pluginID, commandName string, userID model.GlobalUserID) (bool, error) {
	return a.CheckCommand(ctx, userID, pluginID, commandName, nil)
}

func (a *Authorizer) EvalPolicy(ctx context.Context, expression string, userID model.GlobalUserID) (bool, error) {
	sc, err := a.buildSubjectContext(ctx, userID)
	if err != nil {
		return false, err
	}

	env := buildExprEnv(ctx, sc, a.client)
	return evaluate(expression, env)
}

// InvalidateUser removes the cached SubjectContext for a user.
func (a *Authorizer) InvalidateUser(userID model.GlobalUserID) {
	if a.subjectCache != nil {
		a.subjectCache.Delete(userID)
	}
}

// InvalidateCommandPolicy removes the cached policy for a command.
func (a *Authorizer) InvalidateCommandPolicy(pluginID, commandName string) {
	if a.policyCache != nil {
		a.policyCache.Delete(commandPolicyKey{pluginID, commandName})
	}
}

// ClearCaches clears all authorization caches.
func (a *Authorizer) ClearCaches() {
	if a.subjectCache != nil {
		a.subjectCache.Clear()
	}
	if a.policyCache != nil {
		a.policyCache.Clear()
	}
}

func (a *Authorizer) getCommandPolicy(ctx context.Context, pluginID, commandName string) (bool, string, bool, error) {
	key := commandPolicyKey{pluginID, commandName}

	if a.policyCache != nil {
		if cached, ok := a.policyCache.Get(key); ok {
			a.incAuthzCache("policy", "hit")
			return cached.enabled, cached.policyExpr, cached.found, nil
		}
		a.incAuthzCache("policy", "miss")
	}

	enabled, policyExpr, found, err := a.store.GetCommandPolicy(ctx, pluginID, commandName)
	if err != nil {
		return false, "", false, err
	}

	if a.policyCache != nil {
		a.policyCache.Set(key, commandPolicyValue{enabled, policyExpr, found})
	}
	return enabled, policyExpr, found, nil
}

func (a *Authorizer) getPluginPolicy(ctx context.Context, pluginID string) (string, error) {
	key := commandPolicyKey{pluginID, ""}

	if a.policyCache != nil {
		if cached, ok := a.policyCache.Get(key); ok {
			return cached.policyExpr, nil
		}
	}

	expr, err := a.store.GetPluginPolicy(ctx, pluginID)
	if err != nil {
		return "", err
	}

	if a.policyCache != nil {
		a.policyCache.Set(key, commandPolicyValue{enabled: true, policyExpr: expr, found: expr != ""})
	}
	return expr, nil
}

// InvalidatePluginPolicy removes the cached plugin-level policy.
func (a *Authorizer) InvalidatePluginPolicy(pluginID string) {
	if a.policyCache != nil {
		a.policyCache.Delete(commandPolicyKey{pluginID, ""})
	}
}

// checkRolesSpice - НОВАЯ РЕАЛИЗАЦИЯ через SpiceDB вместо SQL
func (a *Authorizer) checkRolesSpice(ctx context.Context, userID model.GlobalUserID, req *model.RoleRequirements) (bool, error) {
	if req == nil {
		return true, nil
	}

	if req.SystemRole == "" && len(req.GlobalRoles) == 0 && req.PluginID == "" {
		return true, nil
	}

	// Конвертируем ID пользователя в строку для SpiceDB
	userStrID := strconv.FormatInt(int64(userID), 10)
	subject := &v1.SubjectReference{
		Object: &v1.ObjectReference{ObjectType: "user", ObjectId: userStrID},
	}

	// 1. Проверка SystemRole
	if req.SystemRole != "" {
		ok, err := a.checkPermission(ctx, "system_role", req.SystemRole, "is_member", subject)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}

	// 2. Проверка GlobalRoles
	// Логика: у пользователя должны быть ВСЕ указанные роли (AND), как было в старом коде
	for _, role := range req.GlobalRoles {
		ok, err := a.checkPermission(ctx, "global_role", role, "is_member", subject)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}

	// 3. Проверка PluginRole
	if req.PluginID != "" && req.PluginRole != "" {
		// В SpiceDB мы можем проверять права на ресурс "plugin" или роль "plugin_role"
		// В схеме мы договорились использовать namespace "plugin" и permission "view/manage"
		// Но если нужно проверить именно роль плагина:
		ok, err := a.checkPermission(ctx, "plugin_role", req.PluginRole, "is_member", subject)
		if err != nil {
			// Если ресурс не найден или ошибка, считаем что нет доступа
			a.logger.Warn("plugin role check failed", slog.Any("error", err))
			return false, nil
		}
		if !ok {
			return false, nil
		}
	}

	return true, nil
}

// checkPermission - хелпер для запроса к SpiceDB
func (a *Authorizer) checkPermission(ctx context.Context, objectType, objectID, permission string, subject *v1.SubjectReference) (bool, error) {
	req := &v1.CheckPermissionRequest{
		Resource:   &v1.ObjectReference{ObjectType: objectType, ObjectId: objectID},
		Permission: permission,
		Subject:    subject,
	}

	resp, err := a.client.CheckPermission(ctx, req)
	if err != nil {
		return false, err
	}

	return resp.Permissionship == v1.CheckPermissionResponse_PERMISSIONSHIP_HAS_PERMISSION, nil
}

func (a *Authorizer) buildSubjectContext(ctx context.Context, userID model.GlobalUserID) (*SubjectContext, error) {
	if a.subjectCache != nil {
		if cached, ok := a.subjectCache.Get(userID); ok {
			a.incAuthzCache("subject", "hit")
			return cached, nil
		}
		a.incAuthzCache("subject", "miss")
	}

	sc := a.newSubjectContext(userID)
	a.loadSubjectBase(ctx, sc)
	a.loadSubjectEnrichment(ctx, sc)

	if a.subjectCache != nil {
		a.subjectCache.Set(userID, sc)
	}

	return sc, nil
}

func (a *Authorizer) newSubjectContext(userID model.GlobalUserID) *SubjectContext {
	return &SubjectContext{
		UserID: userID,
		Attrs:  make(map[string]any),
	}
}

func (a *Authorizer) loadSubjectBase(ctx context.Context, sc *SubjectContext) {
	g, groupCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		extID, err := a.store.GetExternalID(groupCtx, sc.UserID)
		if err != nil {
			a.logger.Warn("failed to get external_id", slog.Any("error", err))
			return nil
		}
		sc.ExternalID = extID
		return nil
	})

	g.Go(func() error {
		roles, err := a.store.GetAllRoleNames(groupCtx, sc.UserID)
		if err != nil {
			a.logger.Warn("failed to get roles", slog.Any("error", err))
			return nil
		}
		sc.Roles = roles
		return nil
	})

	g.Go(func() error {
		channel, locale, err := a.store.GetUserChannelAndLocale(groupCtx, sc.UserID)
		if err != nil {
			a.logger.Warn("failed to get channel/locale", slog.Any("error", err))
			return nil
		}
		sc.PrimaryChannel = channel
		sc.Locale = locale
		return nil
	})

	_ = g.Wait()
}

func (a *Authorizer) loadSubjectEnrichment(ctx context.Context, sc *SubjectContext) {
	if sc.ExternalID == "" {
		return
	}

	g, groupCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		groups, err := a.lookupMemberGroups(groupCtx, sc.UserID)
		if err != nil {
			a.logger.Warn("failed to get member groups from SpiceDB", slog.Any("error", err))
			return nil
		}
		sc.Groups = groups
		return nil
	})

	g.Go(func() error {
		for _, p := range a.providers {
			if err := p.LoadAttributes(groupCtx, sc); err != nil {
				a.logger.Warn("attribute provider failed", slog.Any("error", err))
			}
		}
		return nil
	})

	_ = g.Wait()
}

// lookupMemberGroups finds all study_groups where the user is a member via SpiceDB.
func (a *Authorizer) lookupMemberGroups(ctx context.Context, userID model.GlobalUserID) ([]string, error) {
	if a.client == nil {
		return nil, nil
	}

	userStrID := strconv.FormatInt(int64(userID), 10)
	stream, err := a.client.LookupResources(ctx, &v1.LookupResourcesRequest{
		ResourceObjectType: "study_group",
		Permission:         "view_own_data",
		Subject: &v1.SubjectReference{
			Object: &v1.ObjectReference{ObjectType: "user", ObjectId: userStrID},
		},
	})
	if err != nil {
		return nil, err
	}

	var groups []string
	for {
		resp, err := stream.Recv()
		if err != nil {
			break
		}
		groups = append(groups, resp.ResourceObjectId)
	}
	return groups, nil
}

func (a *Authorizer) incAuthzCache(cacheName, result string) {
	if a.metrics == nil {
		return
	}
	a.metrics.AuthzCacheTotal.WithLabelValues(cacheName, result).Inc()
}
