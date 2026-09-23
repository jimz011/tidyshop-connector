# Configuration

The connector is configured entirely through environment variables. None of them is secret except
the pairing key.

## Environment variables

| Variable | Default | Description |
| --- | --- | --- |
| `PAIRING_KEY` | *(none)* | The master pairing key, at least 12 characters. Takes precedence over `PAIRING_KEY_FILE`. |
| `PAIRING_KEY_FILE` | *(none)* | Path of a file containing the pairing key, used when `PAIRING_KEY` is empty. Surrounding whitespace is ignored. The Unraid template sets it to `/data/pairing-key.txt`. |
| `DATA_FILE` | `/data/state.json` | Where the connector keeps its state. |
| `PORT` | `8787` | The port the connector listens on inside the container. |
| `OIDC_ISSUER` | *(none)* | Turns on [single sign-on](sso.md). The issuer URL; the connector appends `/.well-known/openid-configuration`. Leave unset for device keys only. |
| `OIDC_AUDIENCE` | `tidyshop-android` | The audience an access token must carry. Only used when `OIDC_ISSUER` is set. |
| `ICON_FILE` | `/data/icon.png` | An image served at `/icon.png`, if the file exists. |

One of `PAIRING_KEY` or `PAIRING_KEY_FILE` must yield a key of 12 characters or more; otherwise
the connector logs `PAIRING_KEY or PAIRING_KEY_FILE must provide at least 12 characters` and
exits.

## Volumes

| Path | Description |
| --- | --- |
| `/data` | Persistent state. Must be writable. See [Backups and updates](maintenance.md). |

## Ports

| Port | Description |
| --- | --- |
| `8787/tcp` | Plain HTTP API. Expose it only to your reverse proxy, never to the internet. |

## Health check

The image has a built-in Docker health check that calls `/health` every 30 seconds. You can run the
same check by hand:

```bash
docker exec TidyShop-Connector /tidyshop-connector healthcheck && echo healthy
```

## Timeouts and limits

| Limit | Value |
| --- | --- |
| Request header / body read timeout | 5 s / 15 s |
| Response write timeout | 15 s (not applied to the live stream) |
| Live stream keep-alive | every 25 s |
| Invite lifetime | 24 h by default, 7 days maximum |
| Device link code lifetime | 1 h by default, 24 h maximum |
| Backups per member | 14, the oldest dropped first |
| Backup size | 2 MiB |
