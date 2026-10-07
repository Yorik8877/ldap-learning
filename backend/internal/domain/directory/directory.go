// Package directory — словарь браузера дерева каталога. DN здесь — непрозрачная строка:
// разбор синтаксиса DN (экранирование, регистр) — знание источника, оно в слое транспорта.
package directory

import "errors"

type DN string

type Node struct {
	DN            DN
	RDN           string
	ObjectClasses []string
	HasChildren   bool
}

// Entry делит атрибуты на обычные и служебные: служебные (memberOf, entryUUID, ...)
// ведёт сам сервер, и это стоит видеть.
type Entry struct {
	DN                    DN
	Attributes            map[string][]string
	OperationalAttributes map[string][]string
}

var (
	ErrNotFound    = errors.New("entry not found")
	ErrOutsideBase = errors.New("dn is outside the directory base")
	ErrInvalidDN   = errors.New("invalid dn")
	// ErrUnavailable — каталог не отвечает. Общая для всех репозиториев поверх LDAP.
	ErrUnavailable = errors.New("directory unavailable")
)
