package user_service

import (
	"context"
	"errors"
	"fmt"

	"ldap-admin/internal/domain/group"
	"ldap-admin/internal/domain/user"
)

type UserStore interface {
	List(ctx context.Context) ([]user.User, error)
	Get(ctx context.Context, uid user.UID) (user.User, error)
	Create(ctx context.Context, created user.User) error
	Update(ctx context.Context, updated user.User) error
	SetPassword(ctx context.Context, uid user.UID, password string) error
	Delete(ctx context.Context, uid user.UID) error
}

type GroupReader interface {
	GroupsOf(ctx context.Context, uid user.UID) ([]group.Group, error)
}

type Service struct {
	users  UserStore
	groups GroupReader
}

func New(users UserStore, groups GroupReader) *Service {
	return &Service{users: users, groups: groups}
}

func (s *Service) List(ctx context.Context) ([]user.User, error) {
	return s.users.List(ctx)
}

func (s *Service) Get(ctx context.Context, uid user.UID) (user.User, []group.Name, error) {
	found, err := s.users.Get(ctx, uid)
	if err != nil {
		return user.User{}, nil, err
	}
	groups, err := s.groups.GroupsOf(ctx, uid)
	if err != nil {
		return user.User{}, nil, err
	}
	names := make([]group.Name, 0, len(groups))
	for _, membership := range groups {
		names = append(names, membership.Name)
	}
	return found, names, nil
}

// Create — две операции LDAP без общей транзакции: запись и пароль. Если пароль не
// задался, запись удаляется, чтобы в каталоге не остался пользователь без пароля.
func (s *Service) Create(ctx context.Context, created user.User, password string) error {
	if err := user.ValidatePassword(password); err != nil {
		return err
	}
	if err := s.users.Create(ctx, created); err != nil {
		return err
	}
	if err := s.users.SetPassword(ctx, created.UID, password); err != nil {
		rollbackErr := s.users.Delete(ctx, created.UID)
		return errors.Join(fmt.Errorf("set password: %w", err), rollbackErr)
	}
	return nil
}

func (s *Service) Update(ctx context.Context, updated user.User) error {
	return s.users.Update(ctx, updated)
}

func (s *Service) SetPassword(ctx context.Context, uid user.UID, password string) error {
	if err := user.ValidatePassword(password); err != nil {
		return err
	}
	return s.users.SetPassword(ctx, uid, password)
}

// Delete не чистит группы сам: это делает оверлей refint. Но если пользователь —
// единственный участник группы, refint молча оставит в ней ссылку на удалённую запись,
// поэтому такой случай отсекается заранее.
func (s *Service) Delete(ctx context.Context, actor, uid user.UID) error {
	if actor == uid {
		return user.ErrSelfDelete
	}
	groups, err := s.groups.GroupsOf(ctx, uid)
	if err != nil {
		return err
	}
	if blocking := soleMemberships(groups, uid); len(blocking) > 0 {
		return &group.SoleMemberError{Groups: blocking}
	}
	return s.users.Delete(ctx, uid)
}

func soleMemberships(groups []group.Group, uid user.UID) []group.Name {
	var names []group.Name
	for _, membership := range groups {
		if membership.IsSoleMember(uid) {
			names = append(names, membership.Name)
		}
	}
	return names
}
