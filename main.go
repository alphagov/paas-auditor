package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"code.cloudfoundry.org/lager"
)

func main() {
	ctx, shutdown := context.WithCancel(context.Background())

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Reset(syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		shutdown()
	}()

	cfg := NewConfigFromEnv()

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("Store or DatabaseURL must be provided in Config")
	}
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to database")
	}

	if err := app.StartAppEventCollector(); err != nil {
		return err
	}

	cfg.Logger.Info("started collector")
	return app.Wait()
}
