package bot

import (
	"context"
	"errors"
	"flag"
	"github.com/rs/zerolog"
	"github.com/samber/do/v2"
	"github.com/samber/lo"
	"os"
	"q+/internal/core"
	"q+/internal/cron"
	"q+/internal/discord"
	"q+/internal/discord/oauth"
	"q+/internal/discord/sender"
	"q+/internal/ent/client"
	"q+/internal/ent/migration"
	"q+/internal/logging"
	"q+/internal/sheets"
	"q+/internal/web"
	"q+/internal/web/jwt"
	"sync"
)

func Run(ctx context.Context) (err error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	inj := do.New()

	debug := flag.Bool("debug", false, "sets log level to debug")
	flag.Parse()

	do.ProvideValue(inj, logging.LogConfig{
		Debug: *debug,
		//GelfAddr: "localhost:12201",
		GelfAddr: "",
	})
	do.Provide(inj, logging.NewLogger)

	do.ProvideValue(inj, client.DBConfig{
		Host:     os.Getenv("DB_HOST"),
		Port:     os.Getenv("DB_PORT"),
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
		Database: os.Getenv("DB_NAME"),
		Debug:    *debug,
	})
	do.Provide(inj, client.NewEntClientWrapper)
	do.Provide(inj, client.NewEntClient)
	do.Provide(inj, migration.NewMigrator)

	do.ProvideValue(inj, discord.Config{
		Token:           os.Getenv("DISCORD_TOKEN"),
		GuildId:         os.Getenv("DISCORD_GUILD_ID"),
		FrontendBaseUrl: os.Getenv("FRONTEND_BASE_URL"),
	})
	do.Provide(inj, discord.NewDiscordClient)
	do.Provide(inj, discord.NewBot)

	do.ProvideValue(inj, oauth.Config{
		ClientID:     os.Getenv("DISCORD_APP_CLIENT_ID"),
		ClientSecret: os.Getenv("DISCORD_APP_CLIENT_SECRET"),
		BackendUrl:   os.Getenv("BACKEND_URL"),
	})
	do.Provide(inj, oauth.NewOauth)
	do.Provide(inj, web.NewOauthHandler)

	do.ProvideTransient(inj, core.NewCore)

	do.ProvideValue(inj, web.Config{
		Host:  os.Getenv("WEB_HOST"),
		Port:  os.Getenv("WEB_PORT"),
		Debug: *debug,
	})
	do.Provide(inj, web.NewOgentServer)

	do.Provide(inj, sheets.NewSheetsPresenter)

	do.Provide(inj, cron.NewScheduler)

	do.Provide(inj, sender.NewSender)

	do.Provide(inj, discord.NewMentionRenderer)

	do.ProvideValue(inj, jwt.Config{
		Secret: []byte(os.Getenv("JWT_SECRET")),
	})
	do.Provide(inj, jwt.NewCoder)

	defer func() {
		errs := inj.ShutdownWithContext(ctx)
		if errs != nil && len(*errs) > 0 {
			err = errs
		}
	}()

	logger, err := do.Invoke[*zerolog.Logger](inj)
	if err != nil {
		return err
	}
	ctx = logger.WithContext(ctx)

	//migrator, err := do.Invoke[*migration.Migrator](inj)
	//if err != nil {
	//	return err
	//}
	//err = migrator.RunAutomaticMigration(ctx)
	//if err != nil {
	//	return err
	//}

	discordBot, err := do.Invoke[*discord.Bot](inj)
	if err != nil {
		return err
	}

	//server, err := do.Invoke[*web.Server](inj)
	//if err != nil {
	//	return err
	//}

	ogentServer, err := do.Invoke[*web.OgentServer](inj)
	if err != nil {
		return err
	}

	scheduler, err := do.Invoke[*cron.Scheduler](inj)
	if err != nil {
		return err
	}

	// run discord bot and web server concurrently
	var (
		wg      = sync.WaitGroup{}
		errChan = make(chan error, 3)
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		errChan <- discordBot.StartBot(ctx)

		cancel()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		errChan <- ogentServer.StartServer(ctx)

		cancel()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		errChan <- scheduler.Start(ctx)

		cancel()
	}()

	wg.Wait()
	close(errChan)

	return errors.Join(lo.ChannelToSlice(errChan)...)
}
