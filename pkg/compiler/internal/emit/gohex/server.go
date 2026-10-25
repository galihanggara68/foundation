package gohex

import (
	"fmt"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
)

// emitGoServer renders the Go backend entrypoint (cmd/server/main.go) with environment variable
// parsing (PORT, DATABASE_URL), graceful HTTP shutdown on SIGINT/SIGTERM, clean connection pool closure,
// and background outbox relay execution.
func emitGoServer(archIR *arch.ArchitectureIR) string {
	runtimeImport := ""
	relayCode := ""
	if len(archIR.DomainEvents) > 0 {
		runtimeImport = "\t\"github.com/galihanggara68/foundation/contrib/go/runtime\"\n"
		relayCode = `
		if pgContainer.RuntimeDriver != nil && pgContainer.RuntimeDriver.Outbox() != nil {
			relayWorker := runtime.NewOutboxRelayWorker(pgContainer.RuntimeDriver.Outbox(), func(pCtx context.Context, ev runtime.OutboxEvent) error {
				if pgContainer.Events != nil {
					return pgContainer.Events.Publish(pCtx, ev.EventName, ev.Payload)
				}
				return nil
			}, 1*time.Second)
			relayWorker.Start(context.Background())
			defer relayWorker.Stop()
		}`
	}

	return fmt.Sprintf(`package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
%s	"%s/internal/bootstrap"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port

	dbURL := os.Getenv("DATABASE_URL")
	var handler http.Handler
	var db *sql.DB

	if dbURL != "" {
		var err error
		db, err = sql.Open("postgres", dbURL)
		if err != nil {
			log.Fatalf("failed to connect to postgres: %%v", err)
		}
		defer db.Close()
		pgContainer := bootstrap.NewPostgresContainer(db)
		handler = pgContainer.HTTPServer%s
	} else {
		testContainer := bootstrap.NewTestContainer()
		handler = testContainer.HTTPServer
	}

	server := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("Server listening on %%s", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		if err != nil {
			log.Fatalf("server error: %%v", err)
		}
	case sig := <-sigChan:
		log.Printf("Received signal %%s, initiating graceful shutdown...", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("HTTP shutdown error: %%v", err)
		}
		if db != nil {
			_ = db.Close()
		}
		log.Println("Server stopped cleanly")
	}
}
`, runtimeImport, archIR.Module, relayCode)
}
