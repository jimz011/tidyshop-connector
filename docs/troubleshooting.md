# Troubleshooting

## In the app

### Notifications don't arrive

Work through these in order:

1. **Settings → Notifications → What to tell me about** isn't **Off** (it is by default).
2. Android isn't blocking TidyShop's notifications. If it is, TidyShop shows a warning with
   **Open Android settings**.
3. The list isn't muted: open it and check the list menu for **Unmute notifications**.
4. It isn't within your **Quiet hours**.
5. The change was made on a **family** list by **someone else**. Personal lists never notify, and
   TidyShop doesn't notify you while it's open in front of you.
6. With any frequency other than **Live**, Android decides when TidyShop may check, and battery
   savers can delay that a lot. For prompt notifications, choose **Live**, or exempt TidyShop from
   battery optimisation in Android's settings.

### An invite link or QR code doesn't open TidyShop

Install TidyShop first (see [Join the beta](beta.md)), then scan or tap again. If Android offers a
choice of apps, pick TidyShop. You can always join by hand instead: **Family sharing → Pair with a
family server**, with the server address and the code shown under the QR code.

### "Set up first" when creating a family list

Family lists need a family server. [Join a family](guide/family.md) first, or keep the list
**Personal**.

### "List is too long to send"

A copy sent by link carries the whole list inside the link, and this one doesn't fit. Split it into
two lists, or share it as a [family list](guide/sharing.md#share-a-family-list).

### My lists are gone after reinstalling

Lists live on the phone, so uninstalling removes them. Restore from a backup with **Restore a
backup** on the first screen: [Backup and restore](guide/backup.md). Family lists come back once
the phone [reconnects to your family](guide/family.md#after-a-reinstall-or-on-a-new-phone).

### I forgot my app-lock PIN

It can't be reset from inside the app. Reinstall TidyShop, then restore your latest backup. Family
members reconnect with their recovery code.

### "Could not reach the family connector"

The server address is wrong, the server is down, or its certificate isn't trusted. Open
`https://<server-address>/health` in your phone's browser: it should show
`{"service":"TidyShop-Connector","status":"ok"}`. If it doesn't, tell whoever runs the server, and
point them at [Family sharing server](#family-sharing-server) below.

## Family sharing server

For whoever runs the connector. Start with the container log (`docker compose logs
tidyshop-connector`, or **Logs** on the container icon in Unraid) and with
`https://<your-hostname>/health` opened in your phone's browser. Between them they narrow down most
problems.

### The container stops right after starting

**`PAIRING_KEY or PAIRING_KEY_FILE must provide at least 12 characters`**

The connector found no pairing key, or one that's too short. Either set the `PAIRING_KEY`
variable, or make sure the file named by `PAIRING_KEY_FILE` exists inside the container and holds
at least 12 characters. On Unraid, the file must be at
`/mnt/user/appdata/tidyshop-connector/pairing-key.txt` to appear as `/data/pairing-key.txt`.

### The app can't reach the server

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
    See [Reverse proxy and HTTPS](server/reverse-proxy.md).

### "Pairing key rejected"

The key typed in the app doesn't match the server's. Keys are case-sensitive; watch for a stray
space or a missing character when copying. If you changed the key, restart the container so it's
read again.

### "That invite is not valid any more"

The invite was already used, has expired, or was revoked. Invites are single-use — create a new
one. If someone scanned the QR code twice, the first scan already spent it.

### A reinstalled phone joined as a new person

It was paired with a **new** invite instead of being reconnected to its old identity. Have the
owner remove the duplicate member, then reconnect the phone with **Reinstalled this phone?
Reconnect it** — see [Recovery and the pairing key](server/recovery.md).

### Lists only update when I reopen the app

The live stream (`/v1/events`) is being buffered or cut off by the reverse proxy. The connector
already asks Nginx not to buffer it and pings every 25 seconds, so this usually means one of:

- a proxy **read timeout shorter than 25 seconds**;
- **response buffering** switched on somewhere that ignores `X-Accel-Buffering` — a CDN, or an
  extra proxy in front of your proxy;
- **gzip** applied to `text/event-stream`.

Add the dedicated `/v1/events` block from [Reverse proxy and HTTPS](server/reverse-proxy.md)
for your proxy, which turns buffering off and raises the timeout explicitly.

### Single sign-on fails

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

[Open an issue](https://github.com/jimz011/tidyshop-connector/issues/new) with what you did and what the app shows. For server problems, add the
connector log and which reverse proxy you use. Remove your hostname if you prefer, and **never**
include the pairing key, recovery codes or `state.json`.
