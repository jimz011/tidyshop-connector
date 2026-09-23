# App changelog

<!-- Copied from CHANGELOG.md in the private app repository. Update both together. -->

## 1.1.4

- New application id and package, `com.jimz011apps.tidyshop`. The single sign-on redirect is now
  `com.jimz011apps.tidyshop://oauth2redirect`; identity providers need that URI added.
- Notifications for family lists: one per new item, or one rolled-up notification per list, plus
  "notify family" nudges. Off until you turn them on, and never while the app is open.
- Live delivery, an opt-in notification frequency that keeps the connector's event stream open in
  a foreground service, so a change reaches a closed phone in seconds. It comes back after a
  reboot or an app update.
- A demo family, to see what sharing does before hosting a server. Leaving it removes its lists.
- More store icons.
- Refreshed launcher icon and Play Store artwork.

## Before 1.1.4

Development from the first beta, `1.0.0-beta-01`, up to 1.1.4, in the order it landed:

- Lists with categories, collapsible groups, sorting with drag feedback, and shops organised by
  country.
- Learned suggestions, shared across family lists.
- Self-hosted family sharing through the connector, with live sync, per-list sharing and
  single-use invites instead of a shared pairing key.
- Single sign-on through any OpenID Connect provider, with provider presets.
- App lock with PIN or biometrics.
- Optional quantity and size on items; hide empty categories; edit icons from the home screen.
- Share a list by link; back up to a file, to the family server, or daily to your own Google Drive.
- Device enrolment with a hardware-backed key, making single sign-on optional.
- The local account replaced by a simple profile, and sign out split into *Leave family* and
  *Reset app*.
