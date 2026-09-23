# Install with Docker Compose

This page gets the connector running on any machine with Docker. Unraid users can follow
[Install on Unraid](unraid.md) instead.

The image is published for `linux/amd64` and `linux/arm64`:

```
ghcr.io/jimz011/tidyshop-connector:latest
```

## 1. Create a folder and a pairing key

```bash
mkdir -p ~/tidyshop-connector/data
cd ~/tidyshop-connector
umask 077
openssl rand -hex 24 > data/pairing-key.txt
```

The pairing key is the server's administrator credential. You type it into the app **once**, to
become the family owner, and afterwards only need it to recover a phone. Keep a copy somewhere
safe, such as your password manager.

!!! note "No openssl?"
    Any random string of 12 characters or more works. Longer is better; 48 hex characters is what
    the command above produces.

## 2. Create `compose.yml`

=== "Reverse proxy on the same Docker network"

    Recommended. The connector publishes no ports at all; the proxy reaches it by container name.
    Replace `proxy` with the name of the network your reverse proxy is on.

    ```yaml
    services:
      tidyshop-connector:
        image: ghcr.io/jimz011/tidyshop-connector:latest
        container_name: TidyShop-Connector
        restart: unless-stopped
        environment:
          PAIRING_KEY_FILE: /data/pairing-key.txt
        volumes:
          - ./data:/data
        networks:
          - proxy

    networks:
      proxy:
        external: true
    ```

=== "Reverse proxy on the host"

    For a proxy installed directly on the machine, or in a container using host networking.
    The port is bound to `127.0.0.1` so the unencrypted port is never reachable from the network.

    ```yaml
    services:
      tidyshop-connector:
        image: ghcr.io/jimz011/tidyshop-connector:latest
        container_name: TidyShop-Connector
        restart: unless-stopped
        environment:
          PAIRING_KEY_FILE: /data/pairing-key.txt
        volumes:
          - ./data:/data
        ports:
          - "127.0.0.1:8787:8787"
    ```

=== "No reverse proxy yet"

    Runs Caddy alongside the connector. Caddy fetches and renews a Let's Encrypt certificate on
    its own, so ports 80 and 443 must be reachable from the internet and your hostname must
    point at this machine.

    ```yaml
    services:
      tidyshop-connector:
        image: ghcr.io/jimz011/tidyshop-connector:latest
        container_name: TidyShop-Connector
        restart: unless-stopped
        environment:
          PAIRING_KEY_FILE: /data/pairing-key.txt
        volumes:
          - ./data:/data

      caddy:
        image: caddy:2
        restart: unless-stopped
        ports:
          - "80:80"
          - "443:443"
        volumes:
          - ./Caddyfile:/etc/caddy/Caddyfile:ro
          - caddy-data:/data
          - caddy-config:/config

    volumes:
      caddy-data:
      caddy-config:
    ```

    And next to it, a file named `Caddyfile`:

    ```
    tidyshop.example.com {
    	reverse_proxy TidyShop-Connector:8787
    }
    ```

Want single sign-on? Add `OIDC_ISSUER` to `environment` — see [Single sign-on](sso.md).
Every setting is listed in [Configuration](configuration.md).

## 3. Start it

```bash
docker compose up -d
docker compose logs tidyshop-connector
```

A healthy start logs:

```
single sign-on disabled; devices enrol with an invite code
TidyShop-Connector listening on :8787
```

Check the health endpoint (from the machine itself, or through your proxy once it is set up):

```bash
curl http://127.0.0.1:8787/health
# {"service":"TidyShop-Connector","status":"ok"}
```

The container also reports its own health to Docker, so `docker ps` shows `(healthy)` after a few
seconds.

## Next

[:octicons-arrow-right-24: Set up the reverse proxy](reverse-proxy.md) — unless you used the
Caddy option, in which case go straight to [Connect the app](connect-app.md).

## Building the image yourself

If you would rather not pull a prebuilt image:

```bash
git clone https://github.com/jimz011/tidyshop-connector.git
cd tidyshop-connector
docker build -t tidyshop-connector .
```

The build runs the test suite before compiling, so a broken checkout fails to build rather than
producing a broken image. Then use `image: tidyshop-connector` in your compose file.
