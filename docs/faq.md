# FAQ

## Do I need the connector to use TidyShop?

No. TidyShop is a complete shopping-list app on its own, with everything stored on your phone. The
connector is only for sharing lists between people and devices.

## Is there a hosted version I can use instead?

No. Keeping your household's lists on your own hardware is the point: there is no TidyShop cloud,
and the developer never sees your data.

## What does it cost?

Nothing. The connector is free and open source under the
[Mozilla Public License 2.0](https://github.com/jimz011/tidyshop-connector/blob/main/LICENSE).

## What hardware does it need?

Almost none. It's a single static binary that idles at a few megabytes of memory. Anything that
runs Docker on `amd64` or `arm64` — a NAS, a Raspberry Pi 4 or 5, a small VPS — is plenty.

## Why can't I just use the IP address?

Android refuses unencrypted connections for TidyShop, and a phone off your home Wi-Fi can't reach
a LAN address anyway. A hostname with a real certificate solves both. See
[Reverse proxy and HTTPS](getting-started/reverse-proxy.md).

## Do family members need an account anywhere?

No. Each phone generates its own key pair and joins with an invite. There are no passwords to
create or forget. Single sign-on is available if you want it, never required.

## What is stored on the server?

Your household, its members' names, each device's **public** key and name, the shared lists and
their permissions, open invites, hashed recovery codes, and members' backups — all in one file,
`state.json`. Private device keys never leave the phones.

## Can one connector host several households?

It's designed for one household per server. Run a second container, with its own data folder and
hostname, for a second family.

## Is the connector reachable from the internet safe?

It's built to be: every request except the health check and enrolment needs a signed device
assertion or a verified OIDC token, invites are single-use and short-lived, and the pairing key is
compared in constant time. Keep port `8787` closed to the internet, keep the pairing key private,
and keep the image up to date.

## Can I see or edit the data without the app?

`state.json` is readable JSON, but edit it only with the container stopped — the connector keeps its
state in memory and would overwrite your changes on its next save.

## Where do I report a bug or ask for a feature?

On [GitHub Issues](https://github.com/jimz011/tidyshop-connector/issues).
