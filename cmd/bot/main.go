package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"SuperBotGo/internal/auth/userhttp"
	"SuperBotGo/internal/authz"
	"SuperBotGo/internal/blacklist"
	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/notification"
	"SuperBotGo/internal/plugin"
	"SuperBotGo/internal/plugin/core"
	"SuperBotGo/internal/state"
	"SuperBotGo/internal/state/storage"
	"SuperBotGo/internal/university"
	"SuperBotGo/internal/user"
)

const (
	defaultWasmMemoryLimitPages = 8192 // 512 MiB
	defaultSyncInterval         = 1 * time.Hour
	httpReadTimeout             = 30 * time.Second
	httpWriteTimeout            = 60 * time.Second
	httpIdleTimeout             = 120 * time.Second
	focusTrackerTimeout         = 10 * time.Minute
)

func main() {
	logger := newLogger()

	cfg, err := loadApplicationConfig(logger)
	if err != nil {
		logger.Error("failed to load config", slog.Any("error", err))
		os.Exit(1)
	}

	bootstrapCtx := context.Background()

	fileStore, err := newFileStore(bootstrapCtx, cfg, logger)
	if err != nil {
		logger.Error("failed to create file store", slog.Any("error", err))
		os.Exit(1)
	}

	runtime, err := newRuntimeServices(bootstrapCtx, cfg, logger, fileStore)
	if err != nil {
		logger.Error("failed to initialise runtime services", slog.Any("error", err))
		os.Exit(1)
	}

	blobStore, err := newBlobStore(bootstrapCtx, cfg, logger)
	if err != nil {
		logger.Error("failed to create blob store", slog.Any("error", err))
		os.Exit(1)
	}

	redisClient, err := newRedisClient(bootstrapCtx, cfg, logger, runtime.cronScheduler)
	if err != nil {
		logger.Error("failed to initialise Redis", slog.Any("error", err))
		os.Exit(1)
	}

	stores, err := newPostgresServices(bootstrapCtx, cfg, logger)
	if err != nil {
		logger.Error("failed to initialise PostgreSQL services", slog.Any("error", err))
		os.Exit(1)
	}
	stores.adminBus.SetMetrics(runtime.metrics)
	configureWasmEventBus(cfg, logger, stores, runtime)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	startWasmEventBus(ctx, logger, cfg, runtime)

	userService := user.NewService(stores.userRepo, stores.accountRepo)
	studentResolver := university.NewPgStudentResolver(stores.pool)
	notifyAPI := notification.NewNotifyAPI(runtime.adapterRegistry, userService, stores.notifPrefsRepo, studentResolver, stores.notifScheduleStore)
	notifyAPI.SetTeacherResolver(studentResolver)
	notifyAPI.WithMenuCommand("/start")
	runtime.hostAPI.SetNotifier(notification.NewWasmNotifier(notifyAPI))
	runtime.hostAPI.SetUserProvider(stores.userRepo)

	spiceClient, err := configureSpiceDB(bootstrapCtx, cfg, stores, runtime.metrics, logger)
	if err != nil {
		logger.Error("failed to initialise SpiceDB", slog.Any("error", err))
		os.Exit(1)
	}

	authorizer := authz.NewAuthorizer(stores.authzStore, spiceClient, logger, stores.universityProvider)
	authorizer.SetMetrics(runtime.metrics)
	autoloadPlugins(bootstrapCtx, stores, blobStore, runtime, logger)

	dialogStore := storage.NewRedisStorage(redisClient)
	dialogStore.SetMetrics(runtime.metrics)
	logger.Info("using Redis dialog storage")
	stateMgr := state.NewManager(dialogStore)
	stateMgr.SetMetrics(runtime.metrics)

	userCookieSameSite := userhttp.SameSiteMode(cfg.UserAuth.CookieSameSite)
	userSecureCookie := strings.HasPrefix(cfg.TsuAccounts.CallbackURL, "https://") || cfg.UserAuth.CookieSameSite == "none"
	userSessions := userhttp.NewSessionManagerWithSameSite(cfg.UserAuth.SessionSecret, userSecureCookie, userCookieSameSite)
	adminMux, authHandler := registerAdminRoutes(cfg, logger, runtime, stores, fileStore, blobStore, authorizer, stateMgr, spiceClient, userSessions)
	tsuAuth := configureTSUAccounts(cfg, stores.userRepo, stores.accountRepo, stores.pool, stores.cmdPermStore, runtime.adapterRegistry, adminMux, userSessions, authHandler, logger)

	runtime.senderAPI = plugin.NewSenderAPI(runtime.adapterRegistry, userService)

	allPlugins := []plugin.Plugin{
		core.New(runtime.senderAPI, tsuAuth.linker, stateMgr, userService, stores.notifPrefsRepo, runtime.pluginManager, authorizer, runtime.wasmLoader),
	}
	registerPluginCommands(stateMgr, allPlugins)
	registerPluginCommandsFromMap(stateMgr, runtime.pluginManager.All())

	stateAdapter := channel.NewStateManagerAdapter(stateMgr)
	runtime.pluginManager.Load(allPlugins)
	runtime.cronScheduler.Start()

	focusTracker := plugin.NewFocusTracker(focusTrackerTimeout)
	channelMgr := channel.NewChannelManager(
		userService,
		runtime.triggerRouter,
		stateAdapter,
		runtime.pluginManager,
		authorizer,
		runtime.adapterRegistry,
		focusTracker,
		logger,
	)
	channelMgr.SetMetrics(runtime.metrics)
	channelMgr.SetChatGroupResolver(stores.chatRegistry)
	channelMgr.SetBlacklistChecker(blacklist.NewStore(stores.pool))

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	botStarters := prepareConfiguredBots(cfg, logger, fileStore, redisClient, channelMgr, runtime.metrics, collectCommandNames(runtime.pluginManager), collectCommandEntries(runtime.pluginManager), stores.chatRegistry, adminMux)
	adminServer := newAdminServer(cfg, authHandler, adminMux, runtime.metrics)
	startUniversityPuller(bootstrapCtx, cfg, stores.syncSvc, logger)
	startAdminServer(adminServer, logger, cfg.Admin.Port)
	startPubSubSubscriber(ctx, logger, cfg, stores, blobStore, runtime, stateMgr)
	startPreparedBots(ctx, botStarters)
	startFileStoreCleanup(ctx, logger, fileStore)

	logger.Info("SuperBotGo started, waiting for shutdown signal")

	<-sigCh
	logger.Info("shutdown signal received, stopping...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	shutdownRuntime(shutdownCtx, logger, adminServer, runtime.cronScheduler, tsuAuth.stateStore, runtime.wasmLoader, runtime.rt)

	logger.Info("SuperBotGo stopped")
}

func generateInstanceID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		slog.Error("failed to generate random instance ID", slog.Any("error", err))
		os.Exit(1)
	}
	return hex.EncodeToString(b)
}
