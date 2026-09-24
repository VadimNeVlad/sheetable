package api

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	. "github.com/SheetAble/SheetAble/backend/api/config"
	"github.com/SheetAble/SheetAble/backend/api/controllers"
	"github.com/SheetAble/SheetAble/backend/api/seed"
)

var (
	server = controllers.Server{}
)

func Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return run(ctx, 0)
}

func run(ctx context.Context, portOverride int) error {
	config := Config()
	if err := config.Validate(); err != nil {
		return err
	}

	if err := server.Initialize(); err != nil {
		return err
	}
	defer server.Close()

	if err := seed.MigrateAndSeed(server.DB, config.AdminEmail, config.AdminPassword); err != nil {
		return err
	}

	port := 8080
	if portOverride != 0 {
		port = portOverride
	} else if config.Port != 0 {
		port = config.Port
	}

	return server.Run(ctx, fmt.Sprintf("0.0.0.0:%d", port), config.Dev)
}

func RunWithPort(port int) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return run(ctx, port)
}
