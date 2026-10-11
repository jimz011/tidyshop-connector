# Changelog

## 1.1.0

- A product catalogue for the household. Items can now carry a department and a photo, and every
  item saved with either is remembered by name, so the app can suggest it with both attached.
  New routes: `GET /v1/products`, `PUT` and `DELETE /v1/products/{name}`, `PUT /v1/photos` and
  `GET /v1/photos/{id}`. Photos are small JPEGs kept in a `photos/` folder next to `state.json`.
- Items remember when they were added (`CreatedAt`), so a list sorts the same on every phone.
- Fully backwards compatible: an older app that saves a list does not strip departments or photos.

## 1.0.0

First public release, published as `ghcr.io/jimz011/tidyshop-connector` for `amd64` and `arm64`.

- Shared lists with per-member *View*, *Check* and *Edit* permissions, a household-wide
  permission, subscribable lists and "notify family" nudges.
- Live updates over Server-Sent Events at `/v1/events`, opening with a full snapshot so a
  reconnecting phone is consistent without polling, and a keep-alive every 25 seconds.
- Shared item suggestions, owned by the list and de-duplicated per event.
- Device enrolment with hardware-backed EC P-256 keys and short-lived ES256 assertions, so family
  sharing needs no identity provider.
- Optional single sign-on through any OIDC provider, accepted alongside device keys.
- Single-use, expiring invites and device link codes; the family owner decides who may invite
  and who may use more than one device.
- Recovery for reinstalled phones: one-time recovery codes, owner-issued link codes, and the
  master pairing key with a chosen identity.
- Per-member backups on the server, the 14 most recent kept, 2 MiB each.
- Unraid template, Docker health check, and example configurations for Nginx and Caddy.
