# HTTP API

You don't need this page to run the connector — the app is the only client. It's here for anyone
who wants to understand what the server does, or build something against it.

All bodies are JSON. Errors come back as `{"error": "<message>"}` with a matching status code.

## Authentication

Every route below `/v1` except enrolment expects `Authorization: Bearer <token>`, where the token
is one of:

- **A device assertion** — a JWT the phone signs itself with its enrolled EC P-256 key (ES256),
  with `iss` `tidyshop-device`, `aud` `tidyshop-connector`, `kid` set to the device id, `sub` to
  the member's id, and a lifetime of at most five minutes. Each `jti` is accepted once.
- **An OIDC access token**, when [single sign-on](sso.md) is on, checked against the
  provider's JWKS for signature, issuer, audience and expiry.

The two are told apart by the `iss` claim. Apart from identity inspection, pairing and enrolment,
the caller must already be a member of the household.

## Routes

### Unauthenticated

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/health` | Health check: `{"service":"TidyShop-Connector","status":"ok"}`. |
| `GET` | `/icon.png` | The optional icon from `ICON_FILE`. |
| `POST` | `/v1/devices/enrol` | Enrol a device public key with an invite code, device link code, recovery code, or the master pairing key. |
| `POST` | `/v1/devices/claimable` | With the master pairing key: list the identities a phone could reconnect as. |

### Identity and household

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/v1/me` | The caller's identity. |
| `POST` | `/v1/pair` | Join with an invite code, or create the household with the pairing key (SSO sign-in path). |
| `GET` `POST` | `/v1/households` | The caller's household; create one. |
| `PUT` | `/v1/households/admin` | Claim the owner role with the pairing key. |
| `GET` | `/v1/members` | Identities known to the server. |
| `DELETE` | `/v1/members/{id}` | Owner: remove a member. |
| `PUT` | `/v1/members/{id}/invite-permission` | Owner: allow or stop a member inviting people. |
| `PUT` | `/v1/members/{id}/device-permission` | Owner: allow a member several devices, or limit them to one. |

### Invites and devices

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` `POST` | `/v1/invites` | List open invites; create one. |
| `DELETE` | `/v1/invites/{code}` | Revoke an unused invite. |
| `GET` | `/v1/devices` | The caller's devices. |
| `POST` | `/v1/devices/link` | Create a device link code for the caller, or (owner) for another member. |
| `POST` | `/v1/devices/recovery` | Replace the caller's recovery code. |
| `DELETE` | `/v1/devices/{id}` | Revoke one of the caller's devices. |

### Lists

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` `POST` | `/v1/lists` | Lists the caller can see; create one. |
| `PUT` | `/v1/lists/{id}` | Replace a list. An unknown id creates it, so a device owns its list ids. |
| `DELETE` | `/v1/lists/{id}` | Owner of the list: delete it for everyone. |
| `PUT` | `/v1/lists/{id}/permissions` | Owner of the list: set View / Check / Edit per member or for everyone. |
| `GET` | `/v1/lists/subscribable` | Lists the caller may subscribe to. |
| `POST` | `/v1/lists/{id}/subscribe` | Subscribe to one. |
| `POST` | `/v1/lists/{id}/nudge` | Ask everyone on the list to look at it now. |
| `POST` | `/v1/lists/{id}/suggestions` | Record an added item, for shared suggestions. Idempotent per `eventId`. |
| `DELETE` | `/v1/lists/{id}/suggestions/{name}` | Forget one suggestion. |
| `DELETE` | `/v1/lists/{id}/suggestions` | Forget all suggestions. |
| `GET` | `/v1/admin/lists` | Household owner: every shared list on the server. |
| `DELETE` | `/v1/admin/lists/{id}` | Household owner: delete any list. |

### Backups

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` `POST` | `/v1/backups` | The caller's backups (metadata); store a new one. |
| `GET` `DELETE` | `/v1/backups/{id}` | Fetch or delete one of the caller's backups. |

## Live updates

`GET /v1/events` is a [Server-Sent Events](https://developer.mozilla.org/docs/Web/API/Server-sent_events)
stream. It opens with a `sync` event holding every list the caller can see, then sends events such
as `list.updated`, `list.deleted` and `household.updated` as they happen, each filtered to the
members allowed to see it. A comment line (`: ping`) keeps the connection alive every 25 seconds.
