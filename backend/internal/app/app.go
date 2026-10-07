// Package app — composition root: читает конфиг и руками собирает слои
// транспорт → репозитории → сервисы → хендлеры.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"ldap-admin/internal/api/auth"
	"ldap-admin/internal/api/directory"
	"ldap-admin/internal/api/groups"
	"ldap-admin/internal/api/users"
	"ldap-admin/internal/config"
	"ldap-admin/internal/db/ldap_db"
	"ldap-admin/internal/domain/group"
	"ldap-admin/internal/repos/directory_repo"
	"ldap-admin/internal/repos/group_repo"
	"ldap-admin/internal/repos/session_repo"
	"ldap-admin/internal/repos/tree_layout"
	"ldap-admin/internal/repos/user_repo"
	"ldap-admin/internal/server"
	"ldap-admin/internal/services/auth_service"
	"ldap-admin/internal/services/directory_service"
	"ldap-admin/internal/services/group_service"
	"ldap-admin/internal/services/user_service"
)

func Main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("ldap-admin stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	loaded, err := config.Load(os.LookupEnv)
	if err != nil {
		return err
	}
	adminsGroup, err := group.ParseName(loaded.AdminsGroup)
	if err != nil {
		return fmt.Errorf("LDAP_ADMIN_ADMINS_GROUP: %w", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	client := ldap_db.New(ldap_db.Config{URL: loaded.LDAPURL, BindDN: loaded.BindDN, BindPassword: loaded.BindPassword})
	layout := tree_layout.New(loaded.BaseDN)
	userRepo := user_repo.New(client, layout)
	groupRepo := group_repo.New(client, layout)
	directoryRepo := directory_repo.New(client, loaded.BaseDN)
	sessionRepo := session_repo.New()

	authService := auth_service.New(
		userRepo, groupRepo, userRepo, sessionRepo,
		systemClock{}, randomIDGenerator{}, adminsGroup, loaded.SessionTTL,
	)
	userService := user_service.New(userRepo, groupRepo)
	groupService := group_service.New(groupRepo, userRepo, adminsGroup)
	directoryService := directory_service.New(directoryRepo)

	handler := server.NewHandler(server.Handlers{
		Auth:      auth.New(authService, logger),
		Users:     users.New(userService, logger),
		Groups:    groups.New(groupService, logger),
		Directory: directory.New(directoryService, logger),
	}, logger)
	return server.Run(ctx, loaded.HTTPAddress, handler, logger)
}
