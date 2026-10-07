package group_service

import (
	"context"

	"ldap-admin/internal/domain/group"
	"ldap-admin/internal/domain/user"
)

type GroupStore interface {
	List(ctx context.Context) ([]group.Group, error)
	Get(ctx context.Context, name group.Name) (group.Group, error)
	Create(ctx context.Context, created group.Group) error
	Delete(ctx context.Context, name group.Name) error
	AddMember(ctx context.Context, name group.Name, uid user.UID) error
	RemoveMember(ctx context.Context, name group.Name, uid user.UID) error
}

type UserReader interface {
	Get(ctx context.Context, uid user.UID) (user.User, error)
}

type Service struct {
	groups      GroupStore
	users       UserReader
	adminsGroup group.Name
}

func New(groups GroupStore, users UserReader, adminsGroup group.Name) *Service {
	return &Service{groups: groups, users: users, adminsGroup: adminsGroup}
}

func (s *Service) List(ctx context.Context) ([]group.Group, error) {
	return s.groups.List(ctx)
}

func (s *Service) Get(ctx context.Context, name group.Name) (group.Group, error) {
	return s.groups.Get(ctx, name)
}

// Create проверяет участников заранее: атрибут uniqueMember примет DN и несуществующей
// записи — синтаксис проверяет только формат DN. Возвращает группу, перечитанную из
// каталога: DN участников знает только репозиторий.
func (s *Service) Create(ctx context.Context, created group.Group) (group.Group, error) {
	if err := s.requireUsers(ctx, created.Members); err != nil {
		return group.Group{}, err
	}
	if err := s.groups.Create(ctx, created); err != nil {
		return group.Group{}, err
	}
	return s.groups.Get(ctx, created.Name)
}

// Delete не даёт удалить группу админов: без неё в админку не войдёт никто.
func (s *Service) Delete(ctx context.Context, name group.Name) error {
	if name == s.adminsGroup {
		return group.ErrProtected
	}
	return s.groups.Delete(ctx, name)
}

func (s *Service) AddMember(ctx context.Context, name group.Name, uid user.UID) error {
	if _, err := s.users.Get(ctx, uid); err != nil {
		return err
	}
	return s.groups.AddMember(ctx, name, uid)
}

func (s *Service) RemoveMember(ctx context.Context, name group.Name, uid user.UID) error {
	current, err := s.groups.Get(ctx, name)
	if err != nil {
		return err
	}
	if err := current.CheckRemoval(uid); err != nil {
		return err
	}
	return s.groups.RemoveMember(ctx, name, uid)
}

func (s *Service) requireUsers(ctx context.Context, members []group.Member) error {
	for _, member := range members {
		if member.UID == "" {
			continue
		}
		if _, err := s.users.Get(ctx, member.UID); err != nil {
			return err
		}
	}
	return nil
}
