// Command server is the Pipechat server: an HTTP API that forwards end-to-end
// encrypted messages and files between the people in a group and cannot read them.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"pipechat/internal/config"
	"pipechat/internal/database"
	"pipechat/internal/handlers"
)

func main() {
	settings, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// --host and --port win over CHAT_HOST and CHAT_PORT from the environment or .env.
	host := settings.Host
	port := settings.Port
	flag.StringVar(&host, "host", settings.Host, "address to listen on, for example 127.0.0.1 (empty means all addresses)")
	flag.StringVar(&port, "port", settings.Port, "port to listen on")
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, `Pipechat server: relays end-to-end encrypted messages and files.

Usage:
  server [--host ADDRESS] [--port PORT]

Options:
`)
		flag.PrintDefaults()
		fmt.Fprint(os.Stderr, `
Everything else is configured with environment variables, or a .env file in the
directory the program is started from (see .env.example).
`)
	}
	flag.Parse()

	portNumber, err := strconv.Atoi(port)
	if len(flag.Args()) > 0 || err != nil || portNumber < 1 || portNumber > 65535 {
		fmt.Fprintf(os.Stderr, "error: expected no arguments and a --port between 1 and 65535\nRun with --help for usage.\n")
		os.Exit(2)
	}
	settings.Host = host
	settings.Port = port

	pool, err := database.Connect(context.Background(), settings.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()
	if err := os.MkdirAll(settings.FilesDir, 0o700); err != nil {
		log.Fatalf("files directory: %v", err)
	}

	handlers.Pool = pool
	handlers.Config = settings
	go handlers.SweepSessions()
	go handlers.CleanFiles()

	if settings.TLSCert == "" {
		log.Println("warning: CHAT_TLS_CERT is empty, serving plain HTTP (messages and files are still end-to-end encrypted)")
	}

	// Stop cleanly on Ctrl+C or when the process manager sends SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// No read or write timeout: uploads, downloads and the event stream can last a long time.
	server := &http.Server{
		Addr:              net.JoinHostPort(settings.Host, settings.Port),
		Handler:           handlers.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	go func() {
		log.Printf("Pipechat server listening on %s", server.Addr)
		var err error
		if settings.TLSCert != "" {
			err = server.ListenAndServeTLS(settings.TLSCert, settings.TLSKey)
		} else {
			err = server.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down")
	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	server.Shutdown(shutdownContext)
}
