#!/usr/bin/env bash
set -euo pipefail
[[ $EUID -eq 0 ]] || { echo 'sudo bash install.sh ile çalıştırın.'; exit 1; }
cd "$(dirname "$0")"
DOMAIN=hiddencontact.dpdns.org
if [[ -e /opt/proxy-panel/relay || -e /etc/caddy/Caddyfile.relay-backup ]]; then
 echo 'Mevcut kurulum bulundu. Otomatik üzerine yazılmadı.'; exit 1
fi
printf 'DNS A kaydı VDS IP adresine yönlenmiş olmalı. AAAA varsa o da doğru olmalı.\n80, 443 ve 9999 TCP portları erişilebilir olmalı.\n'
read -r -s -p 'admin için panel parolası (en az 12 karakter): ' PASSWORD; echo
[[ ${#PASSWORD} -ge 12 ]] || { echo 'Parola çok kısa.'; exit 1; }
read -r -s -p 'Parolayı tekrar yazın: ' CONFIRM; echo
[[ "$PASSWORD" == "$CONFIRM" ]] || { echo 'Parolalar eşleşmiyor.'; exit 1; }
export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y caddy golang-go ca-certificates
HASH=$(printf '%s\n' "$PASSWORD" | caddy hash-password)
unset PASSWORD CONFIRM
getent passwd relay >/dev/null || useradd --system --home /var/lib/proxy-panel --shell /usr/sbin/nologin relay
install -d -m 755 /opt/proxy-panel
install -d -o relay -g relay -m 700 /var/lib/proxy-panel
CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /opt/proxy-panel/relay .
cp relay.service /etc/systemd/system/relay.service
if [[ -f /etc/caddy/Caddyfile ]]; then cp -a /etc/caddy/Caddyfile /etc/caddy/Caddyfile.relay-backup; fi
# This server is dedicated to Relay. Review any existing Caddy configuration before running.
cat > /etc/caddy/Caddyfile <<EOF
${DOMAIN} {
    header {
        X-Content-Type-Options nosniff
        Referrer-Policy no-referrer
        -Server
    }
    reverse_proxy 127.0.0.1:9080
}
https://${DOMAIN}:9999 {
    header {
        X-Content-Type-Options nosniff
        Referrer-Policy no-referrer
        X-Frame-Options DENY
        -Server
    }
    basic_auth {
        admin ${HASH}
    }
    reverse_proxy 127.0.0.1:9090
}
EOF
chown root:caddy /etc/caddy/Caddyfile
chmod 640 /etc/caddy/Caddyfile
caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile
systemctl daemon-reload
systemctl enable --now relay
systemctl enable caddy
systemctl restart caddy
printf '\nKurulum tamamlandı.\nDemo: https://%s/login\nPanel: https://%s:9999 (kullanıcı: admin)\nTLS sertifikasının alınması DNS ve port erişimine bağlıdır.\n' "$DOMAIN" "$DOMAIN"
printf 'Kontrol: journalctl -u caddy -u relay -n 60 --no-pager\n'
