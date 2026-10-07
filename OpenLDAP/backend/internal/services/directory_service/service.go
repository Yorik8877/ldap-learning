// Package directory_service — просмотр дерева. Бизнес-правил у просмотра нет, сервис
// тонкий и существует ради единообразия слоёв.
package directory_service

import (
	"context"

	"ldap-admin/internal/domain/directory"
)

type Browser interface {
	Children(ctx context.Context, dn *directory.DN) ([]directory.Node, error)
	Entry(ctx context.Context, dn *directory.DN) (directory.Entry, error)
}

type Service struct {
	browser Browser
}

func New(browser Browser) *Service {
	return &Service{browser: browser}
}

func (s *Service) Children(ctx context.Context, dn *directory.DN) ([]directory.Node, error) {
	return s.browser.Children(ctx, dn)
}

func (s *Service) Entry(ctx context.Context, dn *directory.DN) (directory.Entry, error) {
	return s.browser.Entry(ctx, dn)
}
