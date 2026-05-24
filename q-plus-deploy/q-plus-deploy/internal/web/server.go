package web

import (
	"context"
	"errors"
	"github.com/rs/cors"
	"github.com/rs/zerolog"
	"github.com/samber/do/v2"
	negronizerolog "github.com/samvdb/negroni-zerolog"
	"github.com/urfave/negroni"
	"net"
	"net/http"
	"q+/internal/generated/ent"
	"q+/internal/generated/ent/ogent"
	"q+/internal/web/jwt"
	"time"
)

type Config struct {
	Host  string
	Port  string
	Debug bool
}

type OgentServer struct {
	handler      ogent.Handler
	oauthHandler *OauthHandler
	client       *ent.Client
	config       Config
	coder        *jwt.Coder
}

func NewOgentServer(i do.Injector) (*OgentServer, error) {
	client := do.MustInvoke[*ent.Client](i)
	config := do.MustInvoke[Config](i)
	coder := do.MustInvoke[*jwt.Coder](i)
	oauthHandler := do.MustInvoke[*OauthHandler](i)
	return &OgentServer{
		handler:      NewMyOgentHandler(i),
		oauthHandler: oauthHandler,
		client:       client,
		config:       config,
		coder:        coder,
	}, nil
}

func (s *OgentServer) StartServer(ctx context.Context) error {
	logger := zerolog.Ctx(ctx)

	if !s.config.Debug {
		// TODO
	}
	srv, err := ogent.NewServer(s.handler,
		ogent.WithMiddleware(EntPrivacyMiddleware(s.coder)),
	)
	if err != nil {
		return err
	}

	n := negroni.New()
	n.Use(negroni.NewRecovery())
	n.Use(negronizerolog.NewMiddlewareFromLogger(*logger, "zerolog-web"))
	n.Use(cors.AllowAll())
	n.Use(s.oauthHandler.CreateOauthHandler())
	n.UseHandler(srv)
	server := &http.Server{
		Addr:    net.JoinHostPort(s.config.Host, s.config.Port),
		Handler: n,
		BaseContext: func(l net.Listener) context.Context {
			return ctx
		},
	}

	errChan := make(chan error, 1)
	go func() {
		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
	}()

	select {
	case <-ctx.Done():
		logger.Info().
			Str("event", "closing_server").
			Msg("Closing http server...")
		ctxShutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(ctxShutdown)
	case err := <-errChan:
		return err
	}
}
