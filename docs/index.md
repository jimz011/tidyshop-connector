# TidyShop Connector

![TidyShop](assets/feature-graphic.png){ .tidy-hero }

**The self-hosted family sharing server for [TidyShop](https://play.google.com/store/apps/details?id=com.jimz011apps.tidyshop), the shopping-list app for Android.**

TidyShop works on its own, entirely on your phone. The connector is what you add when you want
the rest of the household on the same lists: one small container, on your own server, holding
your family's shared lists in a single file. There is no TidyShop cloud, no account with us and
nothing that phones home.

<div class="grid cards" markdown>

-   :material-docker: **Docker**

    ---

    Any Linux box, NAS or VPS with Docker Compose. One container and one volume.

    [:octicons-arrow-right-24: Install with Docker](getting-started/docker.md)

-   :material-server: **Unraid**

    ---

    Install from Community Applications, or add the template by hand.

    [:octicons-arrow-right-24: Install on Unraid](getting-started/unraid.md)

-   :material-lock: **HTTPS**

    ---

    The app only talks to HTTPS servers. Nginx, Nginx Proxy Manager, SWAG and Caddy examples.

    [:octicons-arrow-right-24: Reverse proxy](getting-started/reverse-proxy.md)

-   :material-cellphone-link: **Connect the app**

    ---

    Become the family owner, then invite everyone else with a QR code.

    [:octicons-arrow-right-24: Connect the app](getting-started/connect-app.md)

</div>

## What the connector does

### Shared lists, live

Family lists sync between every phone in the household. While the app is open, changes arrive
over a live stream within a second — somebody ticks off the milk in the shop and it disappears
from your list at home. Item suggestions are learned per list, so the whole family gets the same
"you usually buy…" pills.

### Members and permissions

The first person to connect becomes the **family owner**. Everyone else joins with a
single-use invite code or QR code. Each list can be shared as **View**, **Check** (tick items off
but not change them) or **Edit**, per person or for everybody. The owner decides who may invite
new people and who may link more than one device.

### No sign-in required

Each phone creates its own key pair in Android's hardware keystore. The private key never leaves
the phone; the connector keeps only the public half and checks every request against it. That
means **no passwords and no identity provider** — though if you already run Authentik or another
OIDC provider, single sign-on is supported too.

[:octicons-arrow-right-24: Single sign-on](guide/sso.md)

### Backups on your own server

Each member can keep up to 14 backups of their TidyShop data on the connector, next to (not
instead of) backups to a file or Google Drive.

### Small and boring on purpose

A single static Go binary with no dependencies beyond the standard library, a small image for
`amd64` and `arm64`, and one JSON file for state. Back it up by copying one folder.

## Requirements

- Docker (or Unraid) on a machine that is always on
- A hostname and a reverse proxy with a valid HTTPS certificate
- [TidyShop](https://play.google.com/store/apps/details?id=com.jimz011apps.tidyshop) on every
  phone that should share lists
