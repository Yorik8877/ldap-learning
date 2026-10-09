// Package app — composition root: читает конфиг и собирает слои.
// Здесь же подключаются транспорт LDAP → репозитории → сервисы → хендлеры.
package app

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	authApi "samba-admin/internal/api/auth"
	groupsApi "samba-admin/internal/api/groups"
	usersApi "samba-admin/internal/api/users"
	"samba-admin/internal/config"
	"samba-admin/internal/lib/clock"
	"samba-admin/internal/lib/idgen"
	"samba-admin/internal/repos/session_repo"
	"samba-admin/internal/repos/user_repo"
	"samba-admin/internal/server"
	"samba-admin/internal/services/auth_service"
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

	ldapClient, err := initLDAPClient(loaded)
	if err != nil {
		return err
	}
	defer ldapClient.Close()

	sessions := session_repo.New()
	users := user_repo.New(loaded.BaseDN, ldapClient)

	authService := auth_service.New(
		users,
		sessions,
		idgen.New(),
		clock.New(),
		loaded.AdminGroup,
		loaded.SessionTTL,
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	handler := server.NewHandler(server.Handlers{
		Auth:   authApi.New(logger, authService),
		Users:  usersApi.New(logger),
		Groups: groupsApi.New(logger),
	}, logger)
	return server.Run(ctx, loaded.HTTPAddress, handler, logger)
}
