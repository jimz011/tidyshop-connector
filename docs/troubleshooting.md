# Troubleshooting

Start with the container log — `docker compose logs tidyshop-connector`, or **Logs** on the
container icon in Unraid — and with `https://<your-hostname>/health` opened in your phone's
browser. Between them they narrow down most problems.

## The container stops right after starting

**`PAIRING_KEY or PAIRING_KEY_FILE must provide at least 12 characters`**

The connector found no pairing key, or one that's too short. Either set the `PAIRING_KEY`
variable, or make sure the file named by `PAIRING_KEY_FILE` exists inside the container and holds
at least 12 characters. On Unraid, the file must be at
`/mnt/user/appdata/tidyshop-connector/pairing-key.txt` to appear as `/data/pairing-key.txt`.

## The app can't reach the server

Work outwards from the container:

1. **Is it running?** `curl http://127.0.0.1:8787/health` on the server (or the **WebUI** link on
   Unraid) should answer `{"service":"TidyShop-Connector","status":"ok"}`.
2. **Does the proxy reach it?** Open `https://<your-hostname>/health` on a computer. A `502 Bad
   Gateway` means the proxy can't reach the container: check the forward host and port, and — if
   you used the container name — that both containers are on the same custom Docker network.
   The default `bridge` network does not resolve container names.
3. **Does the phone trust it?** Open the same URL in the phone's browser, on mobile data. A
   certificate warning, or no answer at all, is what the app sees too. Self-signed certificates
   don't work.
4. **Is the address right in the app?** It must start with `https://` and must not include
   `:8787`.

!!! note "Using the IP address"
    `http://192.168.x.x:8787` never works: Android blocks unencrypted connections for TidyShop.
    See [Reverse proxy and HTTPS](getting-started/reverse-proxy.md).

## "Pairing key rejected"

The key typed in the app doesn't match the server's. Keys are case-sensitive; watch for a stray
space or a missing character when copying. If you changed the key, restart the container so it's
read again.

## "That invite is not valid any more"

The invite was already used, has expired, or was revoked. Invites are single-use — create a new
one. If someone scanned the QR code twice, the first scan already spent it.

## A reinstalled phone joined as a new person

It was paired with a **new** invite instead of being reconnected to its old identity. Have the
owner remove the duplicate member, then reconnect the phone with **Reinstalled this phone?
Reconnect it** — see [Recovery and the pairing key](guide/recovery.md).

## Lists only update when I reopen the app

The live stream (`/v1/events`) is being buffered or cut off by the reverse proxy. The connector
already asks Nginx not to buffer it and pings every 25 seconds, so this usually means one of:

- a proxy **read timeout shorter than 25 seconds**;
- **response buffering** switched on somewhere that ignores `X-Accel-Buffering` — a CDN, or an
  extra proxy in front of your proxy;
- **gzip** applied to `text/event-stream`.

Add the dedicated `/v1/events` block from [Reverse proxy and HTTPS](getting-started/reverse-proxy.md)
for your proxy, which turns buffering off and raises the timeout explicitly.

## Single sign-on fails

- **The app says it couldn't reach the identity provider** — the issuer URL in the app is wrong,
  or not reachable from the phone. Open `<issuer>/.well-known/openid-configuration` in the phone's
  browser; it should return JSON.
- **Sign-in works but the connector answers 401** — check the log. Usually `OIDC_ISSUER` on the
  connector doesn't match the provider's issuer *exactly* (a trailing slash counts), the token's
  audience isn't `OIDC_AUDIENCE`, or the provider signs with a shared secret (HS256) instead of an
  asymmetric key.
- **The redirect after login goes nowhere** — the provider's redirect URI must be exactly
  `com.jimz011apps.tidyshop://oauth2redirect`.

Remember that SSO is optional: members can always join with an invite and a device key instead.

## Still stuck?

[Open an issue](https://github.com/jimz011/tidyshop-connector/issues/new) with the connector log,
your reverse proxy, and what the app shows. Remove your hostname if you prefer, and **never**
include the pairing key, recovery codes or `state.json`.
