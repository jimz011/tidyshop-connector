# Reverse proxy and HTTPS

The connector speaks plain HTTP on port `8787`. A reverse proxy in front of it adds the HTTPS
the app insists on. Pick the one you already run.

Whatever you use, the result must be that `https://tidyshop.example.com/health` — with your own
hostname — answers `{"service":"TidyShop-Connector","status":"ok"}` from your phone's browser,
**with a padlock and no certificate warning**.

!!! info "Live updates"
    While the app is open it holds one long-lived request to `/v1/events`, a
    [Server-Sent Events](https://developer.mozilla.org/docs/Web/API/Server-sent_events) stream.
    The connector already tells Nginx not to buffer it (`X-Accel-Buffering: no`) and sends a
    keep-alive every 25 seconds, so most proxies need no special configuration. If lists only
    update when you reopen them, see
    [Troubleshooting](../troubleshooting.md#lists-only-update-when-i-reopen-the-app).

=== "Nginx Proxy Manager"

    1. **Hosts → Proxy Hosts → Add Proxy Host.**
    2. **Details** tab:
        - *Domain Names*: `tidyshop.example.com`
        - *Scheme*: `http`
        - *Forward Hostname / IP*: your server's IP, or `TidyShop-Connector` if both containers
          share a custom Docker network
        - *Forward Port*: `8787`
        - Enable **Block Common Exploits**. *Websockets Support* is not used and can stay off.
    3. **SSL** tab: *Request a new SSL Certificate*, enable **Force SSL** and **HTTP/2 Support**.
    4. Save.

    Optional, but it keeps the live stream working whatever buffering or timeout settings you
    change later — paste into the **Advanced** tab:

    ```nginx
    location = /v1/events {
        proxy_pass $forward_scheme://$server:$port;
        proxy_http_version 1.1;
        proxy_set_header Connection "";
        proxy_set_header Host $host;
        proxy_set_header Authorization $http_authorization;
        proxy_buffering off;
        proxy_read_timeout 1h;
    }
    ```

=== "SWAG"

    Create `/config/nginx/proxy-confs/tidyshop.subdomain.conf` (on Unraid:
    `/mnt/user/appdata/swag/nginx/proxy-confs/`). This assumes the connector is on the same
    custom Docker network as SWAG.

    ```nginx
    server {
        listen 443 ssl;
        listen [::]:443 ssl;
        server_name tidyshop.*;
        include /config/nginx/ssl.conf;
        client_max_body_size 4m;

        location = /v1/events {
            include /config/nginx/resolver.conf;
            set $upstream_app TidyShop-Connector;
            set $upstream_port 8787;
            set $upstream_proto http;
            proxy_pass $upstream_proto://$upstream_app:$upstream_port;
            proxy_http_version 1.1;
            proxy_set_header Connection "";
            proxy_set_header Host $host;
            proxy_set_header Authorization $http_authorization;
            proxy_buffering off;
            proxy_read_timeout 1h;
        }

        location / {
            include /config/nginx/proxy.conf;
            include /config/nginx/resolver.conf;
            set $upstream_app TidyShop-Connector;
            set $upstream_port 8787;
            set $upstream_proto http;
            proxy_pass $upstream_proto://$upstream_app:$upstream_port;
        }
    }
    ```

    Add `tidyshop` to SWAG's `SUBDOMAINS` variable so the certificate covers it, then restart SWAG.

=== "Nginx"

    A complete virtual host is in
    [`examples/nginx.conf`](https://github.com/jimz011/tidyshop-connector/blob/main/examples/nginx.conf).
    Replace the `server_name` and the two `ssl_certificate*` paths, drop the `include` line if you
    have no security-headers snippet, then validate and reload:

    ```bash
    nginx -t && nginx -s reload
    ```

    In the official Nginx image `nginx -s reload` can fail when the pid file points at
    `/dev/null`; signal the container instead with `docker kill -s HUP <nginx-container>`.

=== "Caddy"

    ```
    tidyshop.example.com {
        reverse_proxy TidyShop-Connector:8787
    }
    ```

    Caddy gets the certificate by itself and never buffers streamed responses, so that is the
    whole configuration. Use `127.0.0.1:8787` instead of the container name if Caddy runs on the
    host.

=== "Traefik"

    Labels on the connector service, assuming a `websecure` entrypoint and a `letsencrypt`
    certificate resolver:

    ```yaml
        labels:
          - traefik.enable=true
          - traefik.http.routers.tidyshop.rule=Host(`tidyshop.example.com`)
          - traefik.http.routers.tidyshop.entrypoints=websecure
          - traefik.http.routers.tidyshop.tls.certresolver=letsencrypt
          - traefik.http.services.tidyshop.loadbalancer.server.port=8787
    ```

    Traefik streams Server-Sent Events without extra settings.

=== "Cloudflare Tunnel"

    Add a public hostname to your tunnel with *Service* `http://TidyShop-Connector:8787` (or
    `http://<server-ip>:8787`). Cloudflare provides the certificate, and no ports need to be
    opened on your router.

## DNS

Create an `A` (or `CNAME`) record for your hostname pointing at your public IP address, and
forward ports 80 and 443 on your router to the reverse proxy. Cloudflare Tunnel users can skip
both — the tunnel creates the record.

!!! danger "Never forward port 8787"
    Only the reverse proxy should face the internet. Forwarding `8787` directly would send every
    request, including the pairing key, unencrypted.

## Next

[:octicons-arrow-right-24: Connect the app](connect-app.md)
