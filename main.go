package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/jessevdk/go-flags"
	"github.com/sllt/log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var opts struct {
	Listen  string `short:"l" long:"listen" env:"LISTEN" default:"0.0.0.0:8080" description:"listen on host:port"`
	Message string `short:"m" long:"message" env:"MESSAGE" default:"echo" description:"response message"`
	Dbg     bool   `long:"dbg" env:"DEBUG" description:"debug mode"`
}

var revision = "local"

func main() {
	fmt.Printf("echo-http %s\n", revision)

	p := flags.NewParser(&opts, flags.PrintErrors|flags.PassDoubleDash|flags.HelpFlag)
	p.SubcommandsOptional = true
	if _, err := p.Parse(); err != nil {
		if err.(*flags.Error).Type != flags.ErrHelp {
			log.Errorf("cli error: %v", err)
		}
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx); err != nil {
		if !errors.Is(err, http.ErrServerClosed) {
			log.Errorf("server failed, %v", err)
		} else {
			log.Info("server stopped")
		}
	}
}

func run(ctx context.Context) error {

	router := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		echo := struct {
			Message    string            `json:"message"`
			Request    string            `json:"request"`
			Host       string            `json:"host"`
			Headers    map[string]string `json:"headers"`
			RemoteAddr string            `json:"remote_addr"`
		}{
			Message:    opts.Message,
			Request:    r.Method + " " + r.RequestURI,
			Host:       r.Host,
			Headers:    make(map[string]string),
			RemoteAddr: r.RemoteAddr,
		}

		for k, vv := range r.Header {
			echo.Headers[k] = strings.Join(vv, "; ")
		}

		render.JSON(w, r, &echo)
	})

	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.HandleFunc("/*", router)

	srv := http.Server{Addr: opts.Listen,
		Handler:           r,
		ReadHeaderTimeout: time.Second * 30,
		WriteTimeout:      time.Second * 30,
		IdleTimeout:       time.Second * 30,
	}
	log.Infof("starting server on %s", opts.Listen)

	go func() {
		<-ctx.Done()
		if err := srv.Shutdown(ctx); err != nil {
			log.Warnf("shutdown failed, %v", err)
		}
	}()

	return srv.ListenAndServe()
}
