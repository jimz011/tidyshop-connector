# App changelog

<!-- Copied from CHANGELOG.md in the private app repository. Update both together. -->

## 1.2.3

- Smaller, obfuscated release builds, as Google Play asks. Every build is now installed on an
  emulator and opened before it can ship, which would have caught the 1.2.1 crash.

## 1.2.2

- Fixes TidyShop closing as soon as it was opened on 1.2.1.

## 1.2.1

- **Move open items to…** in a list's menu moves everything not yet ticked off to another list,
  for when a shop did not have it. Photo, department, quantity and size go with each item.
- **Move to…** in an item's menu moves just that item to another list.
- TidyShop now needs Android 7.0 or newer, as Google Play's automatic protection requires.
- On Android 15 and newer, Live notifications no longer try to start themselves after a restart,
  which Android no longer allows. A check every 15 minutes covers you until you next open
  TidyShop, which starts Live again.

## 1.2.0

- Lists now read in the order things were added: first added at the top, last added at the bottom.
  That is the default sort, and in Custom order a new item goes to the bottom too. If you already
  picked a sort, yours is kept; tap **First added** under **Sort items** to switch.
- Search inside a list with the new **⌕** button. It matches an item's name or its department.
- Extra details on an item: a **department** and a **photo**, taken with the camera or picked from
  the gallery. Add them under **Details** next to the add field, or edit them on an existing item.
- Products with a department or a photo are remembered. Family lists share that catalogue, and
  the photos, through the connector (version 1.1.0 or newer); personal lists keep it on the phone.
- Suggestions while you type: enter **br** and the products you know that match appear, with their
  department and photo. Tap one to add it with those details attached.

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
