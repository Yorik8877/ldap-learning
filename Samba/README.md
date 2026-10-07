# samba-admin

Учебная админ-панель для Samba AD DC: бэкенд на Go, фронтенд на Vue (позже).
Каркас бэкенда готов, ручки отвечают `501`; домен, сервисы, репозитории и работу с LDAP
пишет автор проекта.

## Запуск

```bash
docker compose up -d --build   # Samba AD DC; LDAPS на localhost:636
./seed.sh                      # начальные данные и CA для бэкенда

cd backend
cp .env.example .env
make run                       # http://localhost:8081
```

Первый старт контейнера занимает около минуты: создаётся домен `corp.example.com`.
`seed.sh` дожидается готовности сам, повторный запуск безопасен.

Вход в админку: `alice` / `Alice-Secret1`.

## Проверка

```bash
curl -i http://localhost:8081/api/users      # 501 {"code":"not_implemented",...}
cd backend && make test
```

## Что почитать

- `CLAUDE.md` — команды, правила слоёв, проверенные особенности Samba.
- `docs/superpowers/specs/2026-10-07-samba-admin-design.md` — спецификация и контракт API.
