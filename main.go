package main

import (
	"context"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/cors"
	"github.com/joho/godotenv"
	"github.com/olusolaa/go-backend/config"
	middleware2 "github.com/olusolaa/go-backend/middleware"
	"github.com/olusolaa/go-backend/pkg"
	"github.com/olusolaa/go-backend/pkg/account"
	"github.com/olusolaa/go-backend/pkg/inbounds"
	"github.com/olusolaa/go-backend/pkg/outbounds"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	httpSwagger "github.com/swaggo/http-swagger"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// @title        Go SMS API
// @version      1.0
// @description  This is an SMS server. You can visit the GitHub repository at https://github.com/olusola/go-backend

// @contact.name   API Support
// @contact.url    http://www.swagger.io/support
// @contact.email  support@swagger.io

// @license.name  MIT
// @license.url   https://opensource.org/licenses/MIT

// @host      localhost:8080
// @BasePath  /
// @securityDefinitions.basic  BasicAuth
func main() {
	godotenv.Load()
	viper.AutomaticEnv()

	config.New(
		config.NewDB,    // postgres
		config.NewRedis, //redis
	)

	//init account_client

	r := initRouter()

	port := "6000"
	envPort := os.Getenv("PORT")
	if envPort != "" {
		port = envPort
	}

	srv := http.Server{
		Addr:         ":" + port,
		Handler:      http.TimeoutHandler(r, time.Minute, "server timed out"),
		ReadTimeout:  time.Minute,
		WriteTimeout: time.Minute,
	}

	var gracefulStop = make(chan os.Signal, 1)
	signal.Notify(gracefulStop, syscall.SIGTERM)
	signal.Notify(gracefulStop, syscall.SIGINT)
	//
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		sig := <-gracefulStop
		log.Printf("caught sig : %+v", sig)

		srv.RegisterOnShutdown(func() {
			// engine.Quit(cancel)
			config.Close()
			cancel()
		})
		log.Println("ENDED", srv.Shutdown(ctx))
	}()

	log.Println("server started on port ", port)
	if err := srv.ListenAndServe(); err != nil {
		log.Println("closing the server", err)
		if err == http.ErrServerClosed {
			select {
			case <-ctx.Done():
				log.Println("Server closed")
			}
		}
	}
}

func initRouter() http.Handler {
	r := chi.NewRouter()
	timeoutDuration := time.Second * 25

	c := cors.New(cors.Options{
		AllowOriginFunc:  func(r *http.Request, origin string) bool { return true },
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	})

	r.Use(c.Handler)
	r.Use(middleware.Recoverer)

	r.Use(middleware.AllowContentType("application/json", "multipart/form-data", ""))
	r.Use(middleware.RequestID)
	r.Use(func(handler http.Handler) http.Handler {
		return http.TimeoutHandler(handler, timeoutDuration, `{"status":"timeout error", "message":"unable to process request at the moment. Try again"}`)
	})
	r.Use(middleware.Logger)
	//wrap the response writer to allow more info
	r.Use(func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww, ok := w.(middleware.WrapResponseWriter)
			if !ok {
				ww = middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			}

			h.ServeHTTP(ww, r)
		})
	})
	r.Use(middleware.Timeout(60 * time.Second))

	r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte("Welcome to go backend"))
		if err != nil {
			return
		}
	})

	r.Mount("/swagger", httpSwagger.WrapHandler)
	r.Route("/api", func(r chi.Router) {
		db := config.GetDB()
		rd := config.GetRedis()

		_, _ = db, rd

		accRep := account.NewRepository(db, rd)
		r.Use(middleware2.BasicAuth(accRep.FindByUsername))
		r.Use(pkg.DecodePostRequest())
		r.Use(middleware2.Limit(
			50,           // requests
			24*time.Hour, // per duration,
			middleware2.WithKeyFuncs(middleware2.KeyByIP, middleware2.KeyByFrom),
			middleware2.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
				pkg.Render(w, r, errors.Errorf(`limit reached for from %s`, pkg.GetDecodedPostRequest().From))
			}),
		))
		inboundRouter := inbounds.NewResource(db, rd)
		outboundRouter := outbounds.NewResource(db, rd)
		r.Mount("/inbound", inboundRouter.Router())
		r.Mount("/outbound", outboundRouter.Router())
	})

	return r
}
