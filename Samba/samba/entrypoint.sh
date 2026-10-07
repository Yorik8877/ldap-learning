#!/bin/sh
# Первый старт: выпускает сертификат LDAPS и создаёт домен. Следующие старты: только запуск samba.
set -eu

tls_dir=/var/lib/samba/private/panel-tls

# Сертификат, который Samba выпускает сама, содержит имя хоста только в CN, без subjectAltName.
# Go (crypto/x509) смотрит только на subjectAltName, поэтому выпускаем свой — с localhost.
issue_certificates() {
  mkdir -p "$tls_dir"
  cd "$tls_dir"
  openssl req -x509 -newkey rsa:2048 -nodes -days 3650 \
    -subj "/CN=samba-admin dev CA" -keyout ca.key -out ca.pem 2>/dev/null
  openssl req -newkey rsa:2048 -nodes -subj "/CN=localhost" \
    -keyout key.pem -out server.csr 2>/dev/null
  printf 'subjectAltName=DNS:localhost,IP:127.0.0.1,DNS:%s\n' "$(hostname)" > san.ext
  openssl x509 -req -in server.csr -CA ca.pem -CAkey ca.key -CAcreateserial \
    -days 3650 -extfile san.ext -out cert.pem 2>/dev/null
  chmod 600 key.pem ca.key
  rm -f server.csr san.ext
}

# xattr_tdb: без --privileged контейнер не может писать ACL в расширенные атрибуты файлов,
# поэтому Samba хранит их в своей базе.
provision_domain() {
  samba-tool domain provision \
    --realm="$SAMBA_REALM" --domain="$SAMBA_DOMAIN" \
    --server-role=dc --dns-backend=SAMBA_INTERNAL \
    --adminpass="$SAMBA_ADMIN_PASSWORD" \
    --option="vfs objects = dfs_samba4 acl_xattr xattr_tdb" \
    --option="tls keyfile = $tls_dir/key.pem" \
    --option="tls certfile = $tls_dir/cert.pem" \
    --option="tls cafile = $tls_dir/ca.pem"
}

if [ ! -f /var/lib/samba/private/sam.ldb ]; then
  issue_certificates
  provision_domain
fi
exec samba --foreground --no-process-group
