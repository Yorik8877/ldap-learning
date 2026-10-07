// Package app — composition root: читает конфиг и собирает слои.
// Здесь же подключаются транспорт LDAP → репозитории → сервисы → хендлеры.
package app

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"samba-admin/internal/api/auth"
	"samba-admin/internal/api/groups"
	"samba-admin/internal/api/users"
	"samba-admin/internal/config"
	"samba-admin/internal/server"
)

func Main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("samba-admin stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	loaded, err := config.Load(os.LookupEnv)
	if err != nil {
		return err
	}

	ldapDB, err := mustInitLDAP(&loaded)
	if err != nil {
		return err
	}
	defer ldapDB.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	handler := server.NewHandler(server.Handlers{
		Auth:   auth.New(logger),
		Users:  users.New(logger),
		Groups: groups.New(logger),
	}, logger)
	return server.Run(ctx, loaded.HTTPAddress, handler, logger)
}
