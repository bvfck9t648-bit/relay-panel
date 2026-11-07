# Relay — Ubuntu için yerel reverse proxy ve yönetim paneli

Bu paket, izinli yerel servisleri yayınlamak için başlangıç sürümüdür; bir kimlik bilgisi yakalama veya üçüncü taraf giriş sayfası klonlama aracı değildir. Sunucunuza henüz kurulmadı. Gerçek DNS/TLS ve Ubuntu 26.04 üzerindeki kurulum bu ortamda denenmedi.

## Adresler

- Demo: **https://hiddencontact.dpdns.org/login**
- Panel: **https://hiddencontact.dpdns.org:9999**
- Panel kullanıcı adı: **admin**
- Parola: kurulum sırasında iki kez girilir; açık metin olarak kaynaklara veya yapılandırmaya yazılmaz. Caddy bcrypt özeti saklar. Sohbette paylaştığınız parolayı kalıcı üretim parolası olarak kullanmayın.

## Kurulum

Yeni/dedike Ubuntu VDS üzerinde çalıştırın. Script mevcut Caddyfile dosyasını yedekler ama aktif yapılandırmayı **değiştirir**. Sunucuda çalışan başka web siteleri varsa önce elle birleştirin. Apache/Nginx gibi 80/443 dinleyen servislerle port çakışması olmamalı.

1. DNS A kaydını VDS IPv4 adresine yönlendirin. AAAA kaydı varsa doğru ve erişilebilir IPv6 adresi olmalı; değilse kaldırın. DNS kaydı doğrudan VDS'ye ulaşmalı. Aradaki CDN/proxy 9999 portunu desteklemeyebilir.
2. Bulut güvenlik duvarında ve işletim sisteminde TCP 80, 443, 9999 portlarına izin verin. SSH erişiminizi koruyun. Script güvenlik duvarını değiştirmez.
3. Paketi VDS'ye kopyalayıp açın:

```bash
unzip proxy-panel.zip
cd proxy-panel
sudo bash install.sh
```

Kurulum Go ve Caddy paketlerini Ubuntu depolarından yükler, kaynak kodu derler, servisleri etkinleştirir. İnternet erişimi gerekir. Caddy otomatik sertifika edinir ve yeniler; DNS ve ACME doğrulaması başarılı olmalıdır. `:9999` dahil iki URL de HTTPS kullanır.

```bash
systemctl status relay caddy --no-pager
journalctl -u caddy -u relay -n 100 --no-pager
curl -I https://hiddencontact.dpdns.org/login
curl -I https://hiddencontact.dpdns.org:9999/
# Panel kimlik doğrulaması yokken 401 dönmeli.
```

## Panel ve yönlendirme

Panelde hedef `http://127.0.0.1:9081` olarak başlar; bu süreç içindeki kimlik bilgisi istemeyen demo servisidir. VDS üzerinde size ait uygulama için örneğin `http://127.0.0.1:3000` yazabilirsiniz. Hedef uygulama **/login yoluna gelen istekleri karşılamalıdır**. `/login` ve `/login/*` yolları değiştirilmeden upstream'e gider. Kök yol ve diğer yollar 404 döner. Uygulamanın kökten `/assets` gibi dosya çağrıları varsa bu sürümde ayrıca route tasarımı gereklidir; HTML/JS içerikleri yeniden yazılmaz.

Güvenlik politikası yalnızca `localhost` ve loopback IP hedeflerine izin verir. Uzak hedef desteği bu pakette yoktur. Proxy/panel portlarına doğrudan hedef tanımlamak engellenir; sunucu yöneticisi yine döngü oluşturabilecek diğer yerel servisleri seçmemelidir.

Panel özellikleri:
- Koyu temalı, mobil uyumlu arayüz
- Hedef adresini değiştirme ve diskte kalıcı saklama
- İstek sayısı, son 100 kaydın hata ve süre özetleri
- 3 saniyede bir yenilenen trafik tablosu
- Son 100 kayıt bellekte; yeniden başlatmada sıfırlanır

## Güvenlik ve sınırlar

- Panel internetten erişilir; Caddy HTTPS + HTTP Basic Auth ile hem sayfayı hem API'yi korur. Tarayıcı giriş penceresi kullanılır; özel HTML giriş formu veya MFA yoktur. Basic Auth oturumu tarayıcı tarafından hatırlanabilir; çıkış için ilgili tarayıcı oturumunu kapatın.
- Kimlik doğrulama için ek hız sınırı / hesap kilitleme uygulanmadı. İnternete açık kalıcı üretim kullanımı için VPN veya IP kısıtı, MFA/SSO ve giriş denemesi sınırlaması ekleyin.
- Go dinleyicileri yalnızca 127.0.0.1:9080, 9090 ve 9081 üzerinde çalışır. Bu portları ayrıca internete yayınlamayın.
- İstek gövdeleri, sorgu parametreleri veya başlıklar kaydedilmez. Alt yollar `/login/*` biçiminde maskelenir.
- Authorization, Cookie ve Set-Cookie aktarımı kasıtlı olarak kapalıdır. Gerçek oturum açma/oturum yönetimi gerektiren uygulamalar desteklenmez; demo kimlik bilgisi toplamaz.
- WebSocket ve streaming standart Go ReverseProxy üzerinden aktarılır; uygulamanın kendine özgü Origin, Cookie veya Authorization gereksinimleri nedeniyle her uygulama çalışmayabilir. Bu akışlar ayrıca entegrasyon testinden geçirilmedi.
- İstek gövdesi 10 MiB ile sınırlıdır; upstream bağlantı ve başlık zaman aşımı vardır. WebSocket/uzun akışlar yalnızca kapanınca trafik listesine eklenir.
- İstek duraklatma, gövde düzenleme, HAR kaydı, uzaktan hedefler, çok kullanıcılı yetkilendirme veya kapsamlı MITM özellikleri bulunmaz. Bu paket güvenli bir **başlangıç sürümüdür**, tam kapsamlı bir güvenlik test ürünü değildir.
- Hedef yapılandırması `/var/lib/proxy-panel/config.json` içinde yalnızca servis kullanıcısının erişebileceği şekilde tutulur. Caddy erişim günlüğü bu paket tarafından açılmaz; servis hata kayıtları journald'a gider.
- Aynı VDS üzerindeki güvenilmeyen yerel süreçlere karşı panel API'sinde ikinci bir kimlik doğrulama katmanı yoktur. Sunucu güvenilir ve tek kiracılı olmalıdır.

## Geliştirme ve doğrulama

Go 1.22 veya üzeri, dış Go bağımlılığı yok:

```bash
go test ./...
go vet ./...
mkdir -p .state
CONFIG_PATH="$PWD/.state/config.json" go run .
# Lokal geliştirme paneli: http://127.0.0.1:9090
```

Testler: hedef allowlist, temel proxy yanıtı, hassas başlıkların engellenmesi ve sorguların kayıtlara girmemesi. Kaynak bu ortamda Go ile derlendi; testler ve go vet başarılı oldu. Gerçek VDS'ye dağıtım ve sertifika alınması kullanıcı tarafından doğrulanmalıdır.

## Servis yönetimi

```bash
sudo systemctl restart relay
sudo systemctl restart caddy
sudo journalctl -u relay -f
```

Caddy yapılandırması: `/etc/caddy/Caddyfile`. İlk kurulumdan önceki kopya: `/etc/caddy/Caddyfile.relay-backup`. Mevcut kurulum üzerine script yeniden çalıştırılmaz; güncellemeleri inceleyip manuel uygulayın.
