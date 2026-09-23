# FAQ

## The app

### How do I get TidyShop?

It's in closed testing on Google Play for now. [Join the beta](beta.md), and once you've been added,
install it from the Play Store.

### Does it cost anything?

No. There are no ads, no subscriptions and nothing to unlock. [Support me](reference/settings.md#elsewhere-in-the-menu)
in the menu has optional tip links, which don't change anything in the app.

### Do I need an account?

No. The first screen asks for your name, and that's it. There's no email, no password, and nothing
is sent to the developer. See [Privacy](reference/privacy.md).

### Do I need a server?

Only for family sharing. Everything else, including [sending someone a copy of a list](guide/sharing.md#send-a-copy),
works without one.

### Does it work offline?

Personal lists, completely: they're stored on the phone, so a basement without reception is no
problem. Family lists stay readable offline too, but they're shared through your family server, so
changes need a connection to reach everyone else. A new family list created offline is uploaded as
soon as the phone is back online. For edits to an existing family list, wait for a connection
before relying on everyone seeing them.

### Is there an iPhone version?

No, TidyShop is Android only.

### Which shops have logos?

Shops in the Netherlands, Germany, Belgium and France for now, from supermarkets to DIY stores.
Missing yours? [Ask for it](https://github.com/jimz011/tidyshop-connector/issues).

### Which languages does it support?

English for now. More languages will follow once the interface settles.

### I lost my phone. Are my lists gone?

Only if there's no backup. Turn on [Google Drive backup](guide/backup.md) or keep backups on your
family server. For family lists, the family server has them, and a new phone gets them back once
it [reconnects](guide/family.md#after-a-reinstall-or-on-a-new-phone).

## The family server

### Do I need to run the server myself?

Only one person in the household does. Everyone else joins with an invite. See
[Family sharing server](server/index.md).

### Is there a hosted family server I can use instead?

No. Keeping your household's lists on your own hardware is the point: there is no TidyShop cloud,
and the developer never sees your data.

### What does the server cost?

Nothing. The connector is free and open source under the
[Mozilla Public License 2.0](https://github.com/jimz011/tidyshop-connector/blob/main/LICENSE).

### What hardware does it need?

Almost none. It's a single static binary that idles at a few megabytes of memory. Anything that
runs Docker on `amd64` or `arm64` — a NAS, a Raspberry Pi 4 or 5, a small VPS — is plenty.

### Why can't I just use the IP address?

Android refuses unencrypted connections for TidyShop, and a phone off your home Wi-Fi can't reach
a LAN address anyway. A hostname with a real certificate solves both. See
[Reverse proxy and HTTPS](server/reverse-proxy.md).

### Do family members need an account anywhere?

No. Each phone generates its own key pair and joins with an invite. There are no passwords to
create or forget. Single sign-on is available if you want it, never required.

### What is stored on the server?

Your household, its members' names, each device's **public** key and name, the shared lists and
their permissions, open invites, hashed recovery codes, and members' backups — all in one file,
`state.json`. Private device keys never leave the phones.

### Can one connector host several households?

It's designed for one household per server. Run a second container, with its own data folder and
hostname, for a second family.

### Is the connector reachable from the internet safe?

It's built to be: every request except the health check and enrolment needs a signed device
assertion or a verified OIDC token, invites are single-use and short-lived, and the pairing key is
compared in constant time. Keep port `8787` closed to the internet, keep the pairing key private,
and keep the image up to date.

### Can I see or edit the data without the app?

`state.json` is readable JSON, but edit it only with the container stopped — the connector keeps its
state in memory and would overwrite your changes on its next save.

## Where do I report a bug or ask for a feature?

On [GitHub Issues](https://github.com/jimz011/tidyshop-connector/issues), for the app as well as the
server, or by email to jimz011apps@gmail.com.
