#!/bin/sh
# Начальные данные домена. Повторный запуск безопасен: существующие объекты пропускаются.
set -eu
cd "$(dirname "$0")"

samba_tool() {
  docker compose exec -T samba samba-tool "$@"
}

wait_until_ready() {
  attempt=0
  until docker compose exec -T samba sh -c \
    'LDAPTLS_CACERT=/var/lib/samba/private/panel-tls/ca.pem ldapwhoami -x -H ldaps://localhost -D "Administrator@corp.example.com" -w "$SAMBA_ADMIN_PASSWORD"' \
    >/dev/null 2>&1; do
    attempt=$((attempt + 1))
    if [ "$attempt" -ge 60 ]; then
      echo "samba is not answering on LDAPS" >&2
      exit 1
    fi
    sleep 2
  done
}

ensure_ou() {
  if samba_tool ou list | grep -qx "$1"; then
    echo "exists: $1"
    return
  fi
  samba_tool ou add "$1"
}

ensure_user() {
  login=$1
  shift
  if samba_tool user show "$login" >/dev/null 2>&1; then
    echo "exists: $login"
    return
  fi
  samba_tool user add "$login" "$@"
  samba_tool user setexpiry "$login" --noexpiry
}

ensure_group() {
  if samba_tool group show "$1" >/dev/null 2>&1; then
    echo "exists: $1"
    return
  fi
  samba_tool group add "$1" --groupou=OU=Groups --description="$2"
}

ensure_member() {
  if samba_tool group listmembers "$1" | grep -qx "$2"; then
    echo "exists: $2 in $1"
    return
  fi
  samba_tool group addmembers "$1" "$2"
}

copy_ca_certificate() {
  mkdir -p backend/certs
  docker compose cp samba:/var/lib/samba/private/panel-tls/ca.pem backend/certs/ca.pem
}

wait_until_ready
ensure_ou OU=Staff
ensure_ou OU=Groups
ensure_user svc-panel 'Svc-Panel-Passw0rd' --description="samba-admin service account"
ensure_member "Domain Admins" svc-panel
ensure_group PanelAdmins "samba-admin administrators"
ensure_user alice 'Alice-Secret1' --userou=OU=Staff --given-name=Alice --surname=Admin \
  --mail-address=alice@corp.example.com
ensure_member PanelAdmins alice
copy_ca_certificate
