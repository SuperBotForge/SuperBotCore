package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"SuperBotGo/internal/admin"
	adminapi "SuperBotGo/internal/admin/api"
	tsuauth "SuperBotGo/internal/auth/tsu"
	"SuperBotGo/internal/auth/userhttp"
	"SuperBotGo/internal/authz"
	"SuperBotGo/internal/authz/outbox"
	"SuperBotGo/internal/authz/providers"
	"SuperBotGo/internal/authz/tuples"
	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/channel/dedup"
	"SuperBotGo/internal/channel/discord"
	"SuperBotGo/internal/channel/mattermost"
	"SuperBotGo/internal/channel/telegram"
	"SuperBotGo/internal/channel/vk"
	"SuperBotGo/internal/chat"
	"SuperBotGo/internal/config"
	"SuperBotGo/internal/database"
	"SuperBotGo/internal/filehttp"
	"SuperBotGo/internal/filestore"
	"SuperBotGo/internal/i18n"
	"SuperBotGo/internal/locale"
	"SuperBotGo/internal/metrics"
	"SuperBotGo/internal/model"
	"SuperBotGo/internal/notification"
	"SuperBotGo/internal/plugin"
	"SuperBotGo/internal/plugin/core"
	"SuperBotGo/internal/pubsub"
	"SuperBotGo/internal/state"
	"SuperBotGo/internal/trigger"
	"SuperBotGo/internal/university"
	"SuperBotGo/internal/user"
	"SuperBotGo/internal/wasm/adapter"
	"SuperBotGo/internal/wasm/eventbus"
	"SuperBotGo/internal/wasm/hostapi"
	"SuperBotGo/internal/wasm/registry"
	wasmrt "SuperBotGo/internal/wasm/runtime"

	v1 "github.com/authzed/authzed-go/proto/authzed/api/v1"
	authzed "github.com/authzed/authzed-go/v1"
	"github.com/authzed/grpcutil"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type runtimeServices struct {
	adapterRegistry  *channel.AdapterRegistry
	pluginManager    *plugin.Manager
	senderAPI        *plugin.SenderAPI
	metrics          *metrics.Metrics
	eventMetrics     *eventbus.Metrics
	rt               *wasmrt.Runtime
	hostAPI          *hostapi.HostAPI
	wasmLoader       *adapter.Loader
	triggerRegistry  *trigger.Registry
	triggerRouter    *trigger.Router
	cronScheduler    *trigger.CronScheduler
	memoryEventBus   *eventbus.Bus
	postgresEventBus *eventbus.PostgresBus
}

type postgresServices struct {
	pool               *pgxpool.Pool
	connString         string
	syncSvc            *university.SyncService
	userRepo           *user.PgUserRepo
	accountRepo        *user.PgAccountRepo
	pluginStore        *adminapi.PgPluginStore
	versionStore       *adminapi.PgVersionStore
	cmdPermStore       *adminapi.PgCommandPermStore
	serviceKeyStore    *adminapi.PgServiceKeyStore
	userTokenStore     *userhttp.PgTokenStore
	adminChatStore     *adminapi.PgAdminChatStore
	authzStore         *authz.PgStore
	universityProvider *providers.UniversityProvider
	adminBus           *pubsub.Bus
	chatRegistry       chat.Registry
	notifPrefsRepo     *notification.PgPrefsRepo
	notifScheduleStore *notification.PgScheduledStore
}

type tsuAuthServices struct {
	stateStore *tsuauth.StateStore
	linker     core.TsuAuthLinker
}

func newLogger() *slog.Logger {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
	return logger
}

func loadApplicationConfig(logger *slog.Logger) (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	locale.SetDefault(cfg.DefaultLocale)
	if err := i18n.Init(cfg.DefaultLocale); err != nil {
		logger.Warn("i18n initialization failed, continuing with fallback keys", slog.Any("error", err))
	}

	return cfg, nil
}

func newFileStore(ctx context.Context, cfg *config.Config, logger *slog.Logger) (filestore.FileStore, error) {
	store, err := filestore.NewS3Store(ctx, filestore.S3StoreConfig{
		Bucket:         cfg.FileStore.S3.Bucket,
		Region:         cfg.FileStore.S3.Region,
		Endpoint:       cfg.FileStore.S3.Endpoint,
		PublicEndpoint: cfg.FileStore.S3.PublicEndpoint,
		AccessKey:      cfg.FileStore.S3.AccessKey,
		SecretKey:      cfg.FileStore.S3.SecretKey,
		Prefix:         cfg.FileStore.S3.Prefix,
	})
	if err != nil {
		return nil, fmt.Errorf("create S3 file store: %w", err)
	}
	logger.Info("using S3 file store", slog.String("bucket", cfg.FileStore.S3.Bucket))
	return store, nil
}

func newRuntimeServices(ctx context.Context, cfg *config.Config, logger *slog.Logger, fileStore filestore.FileStore) (*runtimeServices, error) {
	services := &runtimeServices{
		adapterRegistry: channel.NewAdapterRegistry(),
		pluginManager:   plugin.NewManager(),
		metrics:         metrics.New(),
		eventMetrics:    eventbus.NewMetrics(),
	}
	services.adapterRegistry.SetMetrics(services.metrics)

	rt, err := wasmrt.NewRuntime(ctx, wasmrt.Config{
		CacheDir:                cfg.Admin.ModulesDir + "/.cache",
		DefaultMemoryLimitPages: defaultWasmMemoryLimitPages,
	})
	if err != nil {
		return nil, fmt.Errorf("create wasm runtime: %w", err)
	}
	rt.SetMetrics(services.metrics)
	services.rt = rt

	hostAPI := hostapi.NewHostAPI(hostapi.Dependencies{FileStore: fileStore})
	hostAPI.SetMetrics(services.metrics)
	hostAPI.SetMaxFileStoreSize(cfg.FileStore.MaxFileSize)
	hostAPI.SetHTTPPolicyEnforcement(cfg.WASM.HTTPPolicyEnabledValue())
	services.memoryEventBus = eventbus.New(nil, services.eventMetrics)
	services.memoryEventBus.SetAppMetrics(services.metrics)
	hostAPI.SetEventBus(services.memoryEventBus)
	logger.Info("plugin event bus initialised with in-memory at-least-once delivery")
	if cfg.WASM.HTTPPolicyEnabledValue() {
		logger.Info("wasm http policy enforcement enabled")
	} else {
		logger.Info("wasm http policy enforcement disabled")
	}

	if err := hostAPI.RegisterHostModule(ctx, rt); err != nil {
		return nil, fmt.Errorf("register wasm host module: %w", err)
	}
	services.hostAPI = hostAPI

	pluginRegistry := registry.NewPluginRegistry()
	services.wasmLoader = adapter.NewLoader(rt, hostAPI, adapter.MessageSendFunc(func(ctx context.Context, channelType model.ChannelType, chatID string, msg model.Message) error {
		if services.senderAPI == nil {
			return fmt.Errorf("sender API not initialized")
		}
		return services.senderAPI.ReplyToChat(ctx, channelType, chatID, msg)
	}))
	services.wasmLoader.SetMetrics(services.metrics)
	services.wasmLoader.SetRegistry(pluginRegistry)
	services.wasmLoader.SetStrictMigrate(cfg.WASM.StrictMigrateValue())
	if cfg.WASM.RPCEnabledValue() {
		hostAPI.SetPluginRegistry(services.wasmLoader)
		logger.Info("wasm inter-plugin RPC enabled")
	} else {
		logger.Info("wasm inter-plugin RPC disabled")
	}

	services.triggerRegistry = trigger.NewRegistry()
	services.wasmLoader.SetTriggerRegistry(services.triggerRegistry)

	services.triggerRouter = trigger.NewRouter(services.triggerRegistry, services.pluginManager)
	services.triggerRouter.SetMetrics(services.metrics)
	services.cronScheduler = trigger.NewCronScheduler(services.triggerRouter)
	services.cronScheduler.SetMetrics(services.metrics)
	services.triggerRegistry.SetCronScheduler(services.cronScheduler)

	return services, nil
}

func configureWasmEventBus(cfg *config.Config, logger *slog.Logger, stores *postgresServices, runtime *runtimeServices) {
	if cfg.WASM.EventsBackend != "postgres" {
		logger.Info("wasm event backend configured", slog.String("backend", "memory"))
		return
	}

	runtime.postgresEventBus = eventbus.NewPostgresBus(stores.pool, generateInstanceID(), nil, runtime.eventMetrics)
	runtime.postgresEventBus.SetAppMetrics(runtime.metrics)
	runtime.hostAPI.SetEventBus(runtime.postgresEventBus)
	logger.Info("wasm event backend configured", slog.String("backend", "postgres"))
}

func startWasmEventBus(ctx context.Context, logger *slog.Logger, cfg *config.Config, runtime *runtimeServices) {
	subscriber := trigger.NewEventSubscriber(runtime.triggerRouter, runtime.triggerRegistry)

	if cfg.WASM.EventsBackend == "postgres" {
		if runtime.postgresEventBus == nil {
			logger.Warn("postgres event bus was not configured, falling back to no-op start")
			return
		}
		go func() {
			if err := runtime.postgresEventBus.RunConsumer(ctx, subscriber.Handle); err != nil && ctx.Err() == nil {
				logger.Error("wasm postgres event consumer stopped", slog.Any("error", err))
			}
		}()
		logger.Info("wasm event consumer started", slog.String("backend", "postgres"))
		return
	}

	if runtime.memoryEventBus != nil {
		runtime.memoryEventBus.Subscribe(subscriber.Handle)
		logger.Info("wasm event subscriber registered", slog.String("backend", "memory"))
	}
}

func newBlobStore(ctx context.Context, cfg *config.Config, logger *slog.Logger) (adminapi.BlobStore, error) {
	store, err := adminapi.NewS3BlobStore(ctx, adminapi.S3BlobStoreConfig{
		Bucket:    cfg.Admin.S3.Bucket,
		Region:    cfg.Admin.S3.Region,
		Endpoint:  cfg.Admin.S3.Endpoint,
		AccessKey: cfg.Admin.S3.AccessKey,
		SecretKey: cfg.Admin.S3.SecretKey,
		Prefix:    cfg.Admin.S3.Prefix,
	})
	if err != nil {
		return nil, fmt.Errorf("create S3 blob store: %w", err)
	}
	logger.Info("using S3 blob store", slog.String("bucket", cfg.Admin.S3.Bucket))
	return store, nil
}

func newRedisClient(ctx context.Context, cfg *config.Config, logger *slog.Logger, cronScheduler *trigger.CronScheduler) (*redis.Client, error) {
	if cfg.Redis.Addr == "" {
		return nil, fmt.Errorf("redis addr must be set")
	}
	client, err := database.NewRedisClient(ctx, cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	if err != nil {
		return nil, err
	}
	cronScheduler.SetRedis(client)
	logger.Info("connected to Redis", slog.String("addr", cfg.Redis.Addr))
	return client, nil
}

func newPostgresServices(ctx context.Context, cfg *config.Config, logger *slog.Logger) (*postgresServices, error) {
	if cfg.Database.Host == "" || cfg.Database.DBName == "" {
		return nil, fmt.Errorf("database host and name must be set")
	}

	connString := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host, cfg.Database.Port, cfg.Database.User,
		cfg.Database.Password, cfg.Database.DBName, cfg.Database.SSLMode,
	)

	pool, err := database.NewPool(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	if err := database.RunMigrations(connString); err != nil {
		return nil, fmt.Errorf("run database migrations: %w", err)
	}

	notifScheduleStore := notification.NewPgScheduledStore(pool)
	if err := notifScheduleStore.EnsureSchema(ctx); err != nil {
		return nil, err
	}

	services := &postgresServices{
		pool:               pool,
		connString:         connString,
		syncSvc:            university.NewSyncService(pool),
		userRepo:           user.NewPgUserRepo(pool),
		accountRepo:        user.NewPgAccountRepo(pool),
		pluginStore:        adminapi.NewPgPluginStore(pool),
		versionStore:       adminapi.NewPgVersionStore(pool),
		cmdPermStore:       adminapi.NewPgCommandPermStore(pool),
		serviceKeyStore:    adminapi.NewPgServiceKeyStore(pool),
		userTokenStore:     userhttp.NewPgTokenStore(pool),
		adminChatStore:     adminapi.NewPgAdminChatStore(pool),
		authzStore:         authz.NewPgStore(pool),
		universityProvider: providers.NewUniversityProvider(pool),
		adminBus:           pubsub.NewBus(pool, connString, generateInstanceID()),
		chatRegistry:       chat.NewPgRegistry(pool),
		notifPrefsRepo:     notification.NewPgPrefsRepo(pool),
		notifScheduleStore: notifScheduleStore,
	}

	logger.Info("using PostgreSQL stores")
	return services, nil
}

func configureSpiceDB(ctx context.Context, cfg *config.Config, services *postgresServices, metricSet *metrics.Metrics, logger *slog.Logger) (*authzed.Client, error) {
	if cfg.SpiceDB.Endpoint == "" {
		logger.Warn("SpiceDB endpoint not configured, authorization may not work correctly")
		return nil, nil
	}

	client, err := authzed.NewClient(
		cfg.SpiceDB.Endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpcutil.WithInsecureBearerToken(cfg.SpiceDB.Token),
	)
	if err != nil {
		return nil, fmt.Errorf("create SpiceDB client: %w", err)
	}
	logger.Info("SpiceDB client initialized", slog.String("endpoint", cfg.SpiceDB.Endpoint))

	schemaBytes, err := os.ReadFile("deployments/schema.zed")
	if err != nil {
		return nil, fmt.Errorf("read SpiceDB schema file: %w", err)
	}
	if _, err := client.WriteSchema(ctx, &v1.WriteSchemaRequest{Schema: string(schemaBytes)}); err != nil {
		return nil, fmt.Errorf("write SpiceDB schema: %w", err)
	}
	logger.Info("SpiceDB schema loaded")

	tupleWriter := tuples.NewWriter(client)
	outboxWorker := outbox.NewWorker(services.pool, tupleWriter, logger)
	outboxWorker.SetMetrics(metricSet)
	go func() {
		if err := outboxWorker.Run(ctx); err != nil && ctx.Err() == nil {
			logger.Error("authz outbox worker stopped", slog.Any("error", err))
		}
	}()

	return client, nil
}

func configureTSUAccounts(cfg *config.Config, userRepo *user.PgUserRepo, accountRepo *user.PgAccountRepo, pool *pgxpool.Pool, cmdPermStore *adminapi.PgCommandPermStore, adminMux *http.ServeMux, sessions *userhttp.SessionManager, adminAuth *adminapi.AuthHandler, logger *slog.Logger) tsuAuthServices {
	var services tsuAuthServices
	if cfg.TsuAccounts.ApplicationID == "" || cfg.TsuAccounts.SecretKey == "" {
		logger.Info("TSU.Accounts not configured, skipping")
		return services
	}

	tsuClient := tsuauth.NewClient(
		&http.Client{Timeout: 10 * time.Second},
		cfg.TsuAccounts.ApplicationID,
		cfg.TsuAccounts.SecretKey,
		cfg.TsuAccounts.BaseURL,
	)

	loginURL := fmt.Sprintf("http://localhost:%d/oauth/authorize", cfg.Admin.Port)
	if cfg.TsuAccounts.CallbackURL != "" {
		loginURL = strings.TrimSuffix(cfg.TsuAccounts.CallbackURL, "/login") + "/authorize"
	}
	services.stateStore = tsuauth.NewStateStore(loginURL)

	personLinker := user.NewPersonAutoLinker(pool)
	tsuLinker := tsuauth.NewLinker(userRepo, accountRepo, personLinker, logger)
	tsuHandler := tsuauth.NewHandler(tsuClient, services.stateStore, tsuLinker, userRepo, personLinker, sessions, adminAuth, cfg.TsuAccounts.CallbackURL, logger)
	if cmdPermStore != nil {
		tsuHandler.SetExternalReturnToValidator(cmdPermStore.IsAllowedFrontendOrigin)
	}
	tsuHandler.RegisterRoutes(adminMux)

	services.linker = services.stateStore
	logger.Info("TSU.Accounts authentication enabled")
	return services
}

func registerAdminRoutes(
	cfg *config.Config,
	logger *slog.Logger,
	runtime *runtimeServices,
	stores *postgresServices,
	fileStore filestore.FileStore,
	blobStore adminapi.BlobStore,
	authorizer *authz.Authorizer,
	stateMgr *state.Manager,
	spiceClient *authzed.Client,
	userSessions *userhttp.SessionManager,
) (*http.ServeMux, *adminapi.AuthHandler) {
	adminHandler := adminapi.NewAdminHandler(
		stores.pluginStore,
		blobStore,
		runtime.wasmLoader,
		runtime.pluginManager,
		runtime.rt,
		runtime.hostAPI,
		stateMgr,
		stores.cmdPermStore,
		stores.versionStore,
		stores.adminBus,
		adminapi.PluginLifecycleOptions{ReconfigureEnabled: cfg.WASM.ReconfigureEnabledValue()},
		authorizer,
	)

	adminMux := http.NewServeMux()
	adminCredStore := adminapi.NewPgAdminCredStore(stores.pool)
	adminMailer := adminapi.NewSMTPMailer(adminapi.SMTPMailerConfig{
		Host:     cfg.SMTP.Host,
		Port:     cfg.SMTP.Port,
		Username: cfg.SMTP.Username,
		Password: cfg.SMTP.Password,
		From:     cfg.SMTP.From,
	})
	authHandler := adminapi.NewAuthHandler(cfg.Admin.APIKey, adminCredStore, userSessions, cfg.TsuAccounts.ApplicationID != "" && cfg.TsuAccounts.SecretKey != "")
	authHandler.RegisterRoutes(adminMux)
	userAuthHandler := userhttp.NewHandler(userSessions, stores.userTokenStore)
	userAuthHandler.AddAuthenticator(func(r *http.Request) (model.GlobalUserID, bool) {
		userID, ok := authHandler.AuthenticateSession(r)
		return model.GlobalUserID(userID), ok
	})
	userAuthHandler.RegisterRoutes(adminMux)
	adminapi.NewAdminCredHandler(adminCredStore, adminMailer, authHandler).RegisterRoutes(adminMux)
	adminHandler.RegisterRoutes(adminMux)
	adminapi.NewCommandPermHandler(stores.cmdPermStore, authorizer).RegisterRoutes(adminMux)
	adminapi.NewServiceKeyHandler(stores.serviceKeyStore).RegisterRoutes(adminMux)
	adminapi.NewUserHandler(adminapi.NewPgUserStore(stores.pool), authorizer).RegisterRoutes(adminMux)
	adminapi.NewPluginPermHandler(stores.pluginStore, runtime.wasmLoader, runtime.hostAPI, stores.adminBus).RegisterRoutes(adminMux)
	adminapi.NewChatHandler(stores.adminChatStore, runtime.adapterRegistry).RegisterRoutes(adminMux)
	adminapi.NewChannelStatusHandler(runtime.adapterRegistry, adminapi.ChannelStatusConfig{
		TelegramConfigured:   cfg.Telegram.Token != "",
		DiscordConfigured:    cfg.Discord.Token != "",
		VKConfigured:         cfg.VK.Token != "",
		MattermostConfigured: cfg.Mattermost.URL != "" && cfg.Mattermost.Token != "",
	}).RegisterRoutes(adminMux)
	adminapi.NewRuleSchemaHandler(authz.NewRuleSchemaBuilder(stores.authzStore, stores.universityProvider)).RegisterRoutes(adminMux)
	if spiceClient != nil {
		adminapi.NewRelationshipHandler(spiceClient).RegisterRoutes(adminMux)
	}

	adminapi.NewUniversityRefHandler(stores.pool).RegisterRoutes(adminMux)
	positionStore := adminapi.NewPgPositionStore(stores.pool)
	adminapi.NewPositionHandler(positionStore).RegisterRoutes(adminMux)
	adminapi.NewImportHandler(stores.syncSvc, stores.pool).RegisterRoutes(adminMux)
	adminapi.NewUniversitySyncHandler(stores.syncSvc).RegisterRoutes(adminMux)

	fileHandler := filehttp.NewHandler(fileStore, cfg.FileStore.MaxFileSize)
	if userSessions != nil {
		fileHandler.SetUserAuthenticator(userSessions.Authenticate)
	}
	if stores.userTokenStore != nil {
		fileHandler.SetUserTokenAuthenticator(stores.userTokenStore.AuthenticateUserToken)
	}
	fileHandler.SetPluginExists(func(pluginID string) bool {
		_, ok := runtime.pluginManager.Get(pluginID)
		return ok
	})
	fileHandler.SetPluginAllowsFiles(func(pluginID string) bool {
		return runtime.hostAPI.HasPermission(pluginID, "file")
	})
	fileHandler.RegisterRoutes(adminMux)

	httpTrigger := trigger.NewHTTPTriggerHandler(runtime.triggerRouter, runtime.triggerRegistry)
	httpTrigger.SetMetrics(runtime.metrics)
	httpTrigger.SetSettingLoader(func(ctx context.Context, pluginID, triggerName string) (trigger.HTTPTriggerSetting, bool, error) {
		loaded := trigger.HTTPTriggerSetting{
			Enabled:          true,
			AllowUserKeys:    true,
			AllowServiceKeys: false,
		}

		setting, found, err := stores.cmdPermStore.GetCommandSetting(ctx, pluginID, triggerName)
		if err != nil {
			return trigger.HTTPTriggerSetting{}, false, err
		}
		if found {
			loaded = trigger.HTTPTriggerSetting{
				Enabled:          setting.Enabled,
				AllowUserKeys:    setting.AllowUserKeys,
				AllowServiceKeys: setting.AllowServiceKeys,
				PolicyExpression: setting.PolicyExpression,
				AllowedOrigins:   setting.AllowedOrigins,
			}
		}
		if len(loaded.AllowedOrigins) == 0 {
			pluginOrigins, originsFound, err := stores.cmdPermStore.GetPluginFrontendOrigins(ctx, pluginID)
			if err != nil {
				return trigger.HTTPTriggerSetting{}, false, err
			}
			if originsFound {
				loaded.AllowedOrigins = pluginOrigins.AllowedOrigins
			}
		}
		return loaded, found || len(loaded.AllowedOrigins) > 0, nil
	})
	if userSessions != nil {
		httpTrigger.SetUserAuthenticator(userSessions.Authenticate)
	}
	if stores.userTokenStore != nil {
		httpTrigger.SetUserTokenAuthenticator(stores.userTokenStore.AuthenticateUserToken)
	}
	if stores.serviceKeyStore != nil {
		httpTrigger.SetServiceAuthenticator(func(ctx context.Context, rawToken, pluginID, triggerName string) (trigger.ServiceKeyPrincipal, bool, error) {
			principal, ok, err := stores.serviceKeyStore.AuthenticateServiceKey(ctx, rawToken, pluginID, triggerName)
			if err != nil {
				return trigger.ServiceKeyPrincipal{}, false, err
			}
			if !ok {
				return trigger.ServiceKeyPrincipal{}, false, nil
			}
			return trigger.ServiceKeyPrincipal{ID: principal.ID}, true, nil
		})
	}
	httpTrigger.SetPolicyEvaluator(authorizer.EvalPolicy)
	adminMux.Handle("/api/triggers/http/", httpTrigger)
	adminMux.Handle("GET /metrics", promhttp.Handler())
	admin.RegisterStaticRoutes(adminMux)

	return adminMux, authHandler
}

func newAdminServer(cfg *config.Config, authHandler *adminapi.AuthHandler, mux *http.ServeMux, metricSet *metrics.Metrics) *http.Server {
	authMiddleware := adminapi.NewAdminAuthMiddleware(authHandler)
	handler := authMiddleware.Wrap(mux)
	if metricSet != nil {
		handler = metricSet.InstrumentHTTP(handler)
	}
	return &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Admin.Port),
		Handler:      handler,
		ReadTimeout:  httpReadTimeout,
		WriteTimeout: httpWriteTimeout,
		IdleTimeout:  httpIdleTimeout,
	}
}

func startAdminServer(server *http.Server, logger *slog.Logger, port int) {
	go func() {
		logger.Info("starting Admin API HTTP server", slog.Int("port", port))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("admin API server error", slog.Any("error", err))
		}
	}()
}

func autoloadPlugins(ctx context.Context, stores *postgresServices, blobStore adminapi.BlobStore, runtime *runtimeServices, logger *slog.Logger) {
	if err := adminapi.AutoloadPlugins(ctx, stores.pluginStore, blobStore, runtime.wasmLoader, runtime.pluginManager); err != nil {
		logger.Warn("wasm autoload failed", slog.Any("error", err))
	}
	adapter.RegisterWasmPlugins(runtime.pluginManager, runtime.wasmLoader)
}

func startUniversityPuller(ctx context.Context, cfg *config.Config, syncSvc *university.SyncService, logger *slog.Logger) {
	if !cfg.UniversitySync.Enabled {
		return
	}

	pullInterval, err := time.ParseDuration(cfg.UniversitySync.Interval)
	if err != nil {
		pullInterval = defaultSyncInterval
	}
	dataSource := &university.StubDataSource{
		BaseURL: cfg.UniversitySync.BaseURL,
		Token:   cfg.UniversitySync.Token,
	}
	puller := university.NewPuller(dataSource, syncSvc, logger, pullInterval)
	go func() {
		if err := puller.Run(ctx); err != nil && ctx.Err() == nil {
			logger.Error("university puller stopped", slog.Any("error", err))
		}
	}()
}

func registerPluginCommands(stateMgr *state.Manager, plugins []plugin.Plugin) {
	for _, p := range plugins {
		for _, def := range p.Commands() {
			stateMgr.RegisterCommand(p.ID(), def)
		}
	}
}

func registerPluginCommandsFromMap(stateMgr *state.Manager, plugins map[string]plugin.Plugin) {
	for _, p := range plugins {
		for _, def := range p.Commands() {
			stateMgr.RegisterCommand(p.ID(), def)
		}
	}
}

func collectCommandNames(manager *plugin.Manager) []string {
	var commandNames []string
	for _, p := range manager.All() {
		for _, def := range p.Commands() {
			commandNames = append(commandNames, def.Name)
		}
	}
	return commandNames
}

func startPubSubSubscriber(ctx context.Context, logger *slog.Logger, cfg *config.Config, stores *postgresServices, blobStore adminapi.BlobStore, runtime *runtimeServices, stateMgr *state.Manager) {
	lifecycle := adminapi.NewPluginLifecycleService(
		stores.pluginStore,
		blobStore,
		runtime.wasmLoader,
		runtime.pluginManager,
		runtime.hostAPI,
		stateMgr,
		stores.cmdPermStore,
		stores.versionStore,
		stores.adminBus,
		adminapi.PluginLifecycleOptions{ReconfigureEnabled: cfg.WASM.ReconfigureEnabledValue()},
	)
	go func() {
		if err := stores.adminBus.Subscribe(ctx, lifecycle.HandleEvent); err != nil {
			logger.Error("pubsub subscriber stopped", slog.Any("error", err))
		}
	}()
	logger.Info("pub/sub subscriber started", slog.String("instance", stores.adminBus.InstanceID()))
}

func registerBotFeatures(bot any, mux *http.ServeMux, commandNames []string) error {
	if registrar, ok := bot.(channel.CommandRegistrar); ok {
		registrar.RegisterCommands(commandNames)
	}
	if registrar, ok := bot.(channel.RouteRegistrar); ok {
		return registrar.RegisterRoutes(mux)
	}
	return nil
}

func registerPreparedBot(
	starters []botStarter,
	registerAdapter func(channel.ChannelAdapter),
	mux *http.ServeMux,
	logger *slog.Logger,
	name string,
	bot any,
	adapter channel.ChannelAdapter,
	start func(context.Context) error,
	commandNames []string,
) []botStarter {
	if err := registerBotFeatures(bot, mux, commandNames); err != nil {
		logger.Error("failed to register "+name+" features", slog.Any("error", err))
		return starters
	}

	registerAdapter(adapter)
	return append(starters, func(ctx context.Context) {
		if err := start(ctx); err != nil {
			logger.Error(name+" bot stopped with error", slog.Any("error", err))
		}
	})
}

type botStarter func(context.Context)

func prepareConfiguredBots(
	cfg *config.Config,
	logger *slog.Logger,
	fileStore filestore.FileStore,
	redisClient *redis.Client,
	manager *channel.ChannelManager,
	metricSet *metrics.Metrics,
	commandNames []string,
	chatRegistry chat.Registry,
	mux *http.ServeMux,
) []botStarter {
	joinHandler := newChatJoinHandler(chatRegistry, logger)
	dedupMw := dedup.Middleware(redisClient, dedup.Config{}, logger, metricSet)
	var starters []botStarter

	if cfg.Telegram.Token != "" {
		logger.Info("starting Telegram bot", slog.String("mode", cfg.Telegram.Mode))
		tgHandler := channel.Chain(manager.OnUpdate, dedupMw, telegram.CallbackNormalizer())
		tgBot, err := telegram.NewBot(telegram.BotConfig{
			Token:         cfg.Telegram.Token,
			Mode:          cfg.Telegram.Mode,
			WebhookURL:    cfg.Telegram.WebhookURL,
			WebhookSecret: cfg.Telegram.WebhookSecret,
			WebhookListen: cfg.Telegram.WebhookListen,
		}, tgHandler, joinHandler, fileStore, cfg.FileStore.MaxFileSize, logger)
		if err != nil {
			logger.Error("failed to create Telegram bot", slog.Any("error", err))
		} else {
			starters = registerPreparedBot(
				starters,
				manager.RegisterAdapter,
				mux,
				logger,
				"Telegram",
				tgBot,
				tgBot.Adapter(),
				tgBot.Start,
				commandNames,
			)
		}
	} else {
		logger.Warn("Telegram token not configured, Telegram bot will not start")
	}

	if cfg.Discord.Token == "" {
		logger.Warn("Discord token not configured, Discord bot will not start")
	} else {
		logger.Info("starting Discord bot",
			slog.Int("shard_id", cfg.Discord.ShardID),
			slog.Int("shard_count", cfg.Discord.ShardCount))
		dcHandler := channel.Chain(manager.OnUpdate, dedupMw)
		dcBot, err := discord.NewBot(discord.BotConfig{
			Token:      cfg.Discord.Token,
			ShardID:    cfg.Discord.ShardID,
			ShardCount: cfg.Discord.ShardCount,
		}, dcHandler, joinHandler, fileStore, cfg.FileStore.MaxFileSize, logger)
		if err != nil {
			logger.Error("failed to create Discord bot", slog.Any("error", err))
		} else {
			starters = registerPreparedBot(
				starters,
				manager.RegisterAdapter,
				mux,
				logger,
				"Discord",
				dcBot,
				dcBot.Adapter(),
				dcBot.Start,
				commandNames,
			)
		}
	}

	if cfg.VK.Token == "" {
		logger.Warn("VK token not configured, VK bot will not start")
	} else {
		logger.Info("starting VK bot", slog.String("mode", cfg.VK.Mode))
		vkHandler := channel.Chain(manager.OnUpdate, dedupMw)
		vkBot, err := vk.NewBot(vk.BotConfig{
			Token:        cfg.VK.Token,
			Mode:         cfg.VK.Mode,
			CallbackURL:  cfg.VK.CallbackURL,
			CallbackPath: cfg.VK.CallbackPath,
		}, vkHandler, joinHandler, fileStore, cfg.FileStore.MaxFileSize, logger)
		if err != nil {
			logger.Error("failed to create VK bot", slog.Any("error", err))
		} else {
			starters = registerPreparedBot(
				starters,
				manager.RegisterAdapter,
				mux,
				logger,
				"VK",
				vkBot,
				vkBot.Adapter(),
				vkBot.Start,
				commandNames,
			)
		}
	}

	if cfg.Mattermost.URL == "" || cfg.Mattermost.Token == "" {
		logger.Warn("Mattermost config not complete, Mattermost bot will not start")
		return starters
	}

	logger.Info("starting Mattermost bot", slog.String("url", cfg.Mattermost.URL))
	mmHandler := channel.Chain(manager.OnUpdate, dedupMw)
	mmBot, err := mattermost.NewBot(mattermost.BotConfig{
		URL:           cfg.Mattermost.URL,
		Token:         cfg.Mattermost.Token,
		ActionsURL:    cfg.Mattermost.ActionsURL,
		ActionsPath:   cfg.Mattermost.ActionsPath,
		ActionsSecret: cfg.Mattermost.ActionsSecret,
	}, mmHandler, joinHandler, fileStore, cfg.FileStore.MaxFileSize, logger)
	if err != nil {
		logger.Error("failed to create Mattermost bot", slog.Any("error", err))
		return starters
	}

	return registerPreparedBot(
		starters,
		manager.RegisterAdapter,
		mux,
		logger,
		"Mattermost",
		mmBot,
		mmBot.Adapter(),
		mmBot.Start,
		commandNames,
	)
}

func startPreparedBots(ctx context.Context, starters []botStarter) {
	for _, starter := range starters {
		go starter(ctx)
	}
}

func startFileStoreCleanup(ctx context.Context, logger *slog.Logger, fileStore filestore.FileStore) {
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				n, err := fileStore.Cleanup(ctx)
				if err != nil {
					logger.Error("file store cleanup error", slog.Any("error", err))
				} else if n > 0 {
					logger.Info("file store cleanup", slog.Int("removed", n))
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

func shutdownRuntime(ctx context.Context, logger *slog.Logger, adminServer *http.Server, cronScheduler *trigger.CronScheduler, tsuStateStore *tsuauth.StateStore, wasmLoader *adapter.Loader, rt *wasmrt.Runtime) {
	if err := adminServer.Shutdown(ctx); err != nil {
		logger.Error("admin API server shutdown error", slog.Any("error", err))
	} else {
		logger.Info("admin API server stopped")
	}

	cronScheduler.Stop()
	if tsuStateStore != nil {
		tsuStateStore.Stop()
	}
	if err := wasmLoader.Close(ctx); err != nil {
		logger.Error("wasm loader close error", slog.Any("error", err))
	}
	if err := rt.Close(ctx); err != nil {
		logger.Error("wasm runtime close error", slog.Any("error", err))
	}
}
