# TidyShop

![TidyShop](assets/feature-graphic.png){ .tidy-hero }

**A privacy-first shopping-list app for Android, with optional self-hosted family sharing.**

TidyShop keeps your lists on your phone. You don't need an account, there's no TidyShop cloud, and
nothing is sent to the developer. When the household should share lists, you run a small server of
your own, the TidyShop Connector, and every phone syncs through it.

!!! tip "TidyShop is in closed beta"
    The app is in closed testing on Google Play and needs a few more testers before its public
    release. [Join the beta](beta.md) to get it, and to help it get there.

<div class="grid cards" markdown>

-   :material-rocket-launch: **New here?**

    ---

    Get the app, pick a name, and make your first list in under a minute.

    [:octicons-arrow-right-24: Getting started](getting-started/index.md)

-   :material-book-open-variant: **Learn the app**

    ---

    Lists, items, sharing, notifications, backups and app lock, one topic at a time.

    [:octicons-arrow-right-24: App guide](guide/index.md)

-   :material-server: **Run a family server**

    ---

    Install the connector with Docker or on Unraid, put it behind HTTPS, and invite the family.

    [:octicons-arrow-right-24: Family sharing server](server/index.md)

-   :material-help-circle: **Something's wrong**

    ---

    Quick answers, and fixes for the problems people run into most.

    [:octicons-arrow-right-24: FAQ](faq.md) ·
    [:octicons-arrow-right-24: Troubleshooting](troubleshooting.md)

</div>

## What TidyShop does

### Lists that stay tidy

Keep every shop in its own list: pick the store and TidyShop gives the list its logo, or build one
by hand with your own icon. Add items by typing or with one tap on a suggestion. Quantities and
sizes come straight from what you type, so "2x 500g pasta" becomes pasta, ×2, 500 g. Ticked items
move to *Completed*, and you can swipe between lists without going back.

[:octicons-arrow-right-24: Lists](guide/lists.md) ·
[:octicons-arrow-right-24: Items and suggestions](guide/items.md)

### Share with the household

Send anyone a copy of a list as a link. No server is needed for that. Or run a family server, and
lists become shared for real: a change on one phone shows up on the others within a second, and
each list can be shared as *View*, *Check* (tick items off) or *Edit*. When you need someone to go
shopping, **Notify family** does the asking.

[:octicons-arrow-right-24: Sharing](guide/sharing.md) ·
[:octicons-arrow-right-24: Joining a family](guide/family.md)

### Private by design

There's no account to create and no password to forget. Each phone makes its own key in its
hardware keystore when it joins a family, and an optional PIN or fingerprint lock keeps the app
closed to anyone else holding your phone. There's no analytics, no ads and no crash reporting.

[:octicons-arrow-right-24: Profile and app lock](guide/security.md) ·
[:octicons-arrow-right-24: Privacy](reference/privacy.md)

### Backups where you want them

Save a backup to a file anywhere your phone can reach, let TidyShop back up daily to your own
Google Drive, or keep the last 14 backups on your family server.

[:octicons-arrow-right-24: Backup and restore](guide/backup.md)

## Requirements

- Android 6.0 or newer
- For family sharing: a [TidyShop Connector](server/index.md) that you, or someone in the family,
  hosts
