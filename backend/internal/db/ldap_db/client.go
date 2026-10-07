// Package ldap_db — транспорт к LDAP-серверу (слой 4). Каждая операция открывает своё
// соединение и делает bind сервисным аккаунтом: так нет устаревших соединений и вопроса,
// под чьим именем сейчас выполнен bind. Для учебного проекта это важнее скорости пула.
package ldap_db

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"
)

const (
	defaultTimeout  = 5 * time.Second
	anyObjectFilter = "(objectClass=*)"
)

type Config struct {
	URL          string
	BindDN       string
	BindPassword string
}

type Client struct {
	config Config
}

func New(config Config) *Client {
	return &Client{config: config}
}

type Scope int

const (
	ScopeBase Scope = iota
	ScopeOneLevel
	ScopeSubtree
)

type SearchRequest struct {
	BaseDN     string
	Scope      Scope
	Filter     string
	Attributes []string
}

type Entry struct {
	DN         string
	Attributes map[string][]string
}

// Values ищет атрибут без учёта регистра: сервер возвращает имя так, как оно записано
// в схеме (memberOf), а не так, как его запросили (memberof).
func (e Entry) Values(attribute string) []string {
	for name, values := range e.Attributes {
		if strings.EqualFold(name, attribute) {
			return values
		}
	}
	return nil
}

func (e Entry) First(attribute string) string {
	values := e.Values(attribute)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

type ChangeOperation int

const (
	ChangeAdd ChangeOperation = iota
	ChangeDelete
	ChangeReplace
)

type Change struct {
	Operation ChangeOperation
	Attribute string
	Values    []string
}

func (c *Client) Search(ctx context.Context, request SearchRequest) ([]Entry, error) {
	filter := request.Filter
	if filter == "" {
		filter = anyObjectFilter
	}
	var entries []Entry
	err := c.withServiceConnection(ctx, func(connection *ldap.Conn) error {
		result, err := connection.Search(ldap.NewSearchRequest(
			request.BaseDN, toLDAPScope(request.Scope), ldap.NeverDerefAliases, 0, 0, false,
			filter, request.Attributes, nil,
		))
		if err != nil {
			return err
		}
		entries = toEntries(result.Entries)
		return nil
	})
	return entries, err
}

func (c *Client) Add(ctx context.Context, dn string, attributes map[string][]string) error {
	request := ldap.NewAddRequest(dn, nil)
	for name, values := range attributes {
		request.Attribute(name, values)
	}
	return c.withServiceConnection(ctx, func(connection *ldap.Conn) error {
		return connection.Add(request)
	})
}

func (c *Client) Modify(ctx context.Context, dn string, changes []Change) error {
	request := ldap.NewModifyRequest(dn, nil)
	for _, change := range changes {
		appendChange(request, change)
	}
	return c.withServiceConnection(ctx, func(connection *ldap.Conn) error {
		return connection.Modify(request)
	})
}

func (c *Client) Delete(ctx context.Context, dn string) error {
	return c.withServiceConnection(ctx, func(connection *ldap.Conn) error {
		return connection.Del(ldap.NewDelRequest(dn, nil))
	})
}

func (c *Client) Compare(ctx context.Context, dn, attribute, value string) (bool, error) {
	var matched bool
	err := c.withServiceConnection(ctx, func(connection *ldap.Conn) error {
		result, err := connection.Compare(dn, attribute, value)
		matched = result
		return err
	})
	return matched, err
}

// SetPassword использует расширенную операцию Password Modify (RFC 3062): сервер сам
// хэширует пароль. Запись атрибута userPassword напрямую сохранила бы его открытым текстом.
func (c *Client) SetPassword(ctx context.Context, dn, password string) error {
	return c.withServiceConnection(ctx, func(connection *ldap.Conn) error {
		_, err := connection.PasswordModify(ldap.NewPasswordModifyRequest(dn, "", password))
		return err
	})
}

// VerifyPassword проверяет пароль bind'ом от имени пользователя на отдельном соединении.
func (c *Client) VerifyPassword(ctx context.Context, dn, password string) error {
	if password == "" {
		// Bind с DN и пустым паролем по стандарту — анонимный вход, и многие серверы
		// отвечают на него «успехом». Такой ответ нельзя принять за проверку пароля.
		return ErrInvalidCredentials
	}
	connection, err := c.dial(ctx)
	if err != nil {
		return err
	}
	defer connection.Close()
	return translate(connection.Bind(dn, password))
}

func (c *Client) withServiceConnection(ctx context.Context, operation func(*ldap.Conn) error) error {
	connection, err := c.dial(ctx)
	if err != nil {
		return err
	}
	defer connection.Close()
	if err := connection.Bind(c.config.BindDN, c.config.BindPassword); err != nil {
		return serviceBindError(err)
	}
	return translate(operation(connection))
}

// serviceBindError отделяет отказ сервисного bind от отказа пользовательского: неверный
// пароль сервисного аккаунта — ошибка конфигурации, а не «неверный логин» пользователя.
func serviceBindError(err error) error {
	translated := translate(err)
	if errors.Is(translated, ErrUnavailable) {
		return translated
	}
	return fmt.Errorf("%w: %v", ErrServiceBind, err)
}

func (c *Client) dial(ctx context.Context) (*ldap.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	timeout := timeoutFrom(ctx)
	connection, err := ldap.DialURL(c.config.URL, ldap.DialWithDialer(&net.Dialer{Timeout: timeout}))
	if err != nil {
		return nil, translate(err)
	}
	connection.SetTimeout(timeout)
	return connection, nil
}

func timeoutFrom(ctx context.Context) time.Duration {
	deadline, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		return defaultTimeout
	}
	return max(time.Until(deadline), time.Millisecond)
}

func appendChange(request *ldap.ModifyRequest, change Change) {
	switch change.Operation {
	case ChangeAdd:
		request.Add(change.Attribute, change.Values)
	case ChangeDelete:
		request.Delete(change.Attribute, change.Values)
	case ChangeReplace:
		request.Replace(change.Attribute, change.Values)
	}
}

func toLDAPScope(scope Scope) int {
	switch scope {
	case ScopeOneLevel:
		return ldap.ScopeSingleLevel
	case ScopeSubtree:
		return ldap.ScopeWholeSubtree
	default:
		return ldap.ScopeBaseObject
	}
}

func toEntries(source []*ldap.Entry) []Entry {
	entries := make([]Entry, 0, len(source))
	for _, entry := range source {
		attributes := make(map[string][]string, len(entry.Attributes))
		for _, attribute := range entry.Attributes {
			attributes[attribute.Name] = attribute.Values
		}
		entries = append(entries, Entry{DN: entry.DN, Attributes: attributes})
	}
	return entries
}
