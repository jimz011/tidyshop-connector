# Backup and restore

Your lists live on your phone. Uninstalling TidyShop, or losing the phone, loses them, unless there's
a backup somewhere. Everything below is under **menu → Backup & restore**.

## What a backup holds

Your lists and how the app is arranged: your appearance settings, for example. A backup never
contains your PIN, a password, a token or a server address, so restoring one never signs anybody
in. Family access stays tied to the phone, which is
why a restored phone [reconnects to the family](family.md#after-a-reinstall-or-on-a-new-phone)
separately.

## Three places to keep one

=== "A file"

    **Save to a file → Save backup** writes one backup file wherever you choose: your phone's
    storage, or any cloud folder your phone can see, such as Google Drive, Nextcloud or OneDrive.
    **Restore** opens a file you saved earlier.

    Good for a one-off copy before switching phones.

=== "Google Drive"

    **Turn on** Google Drive backup and TidyShop backs up **daily** to your own Google Drive, into
    the app's private area. Only TidyShop and you can see those files. Other apps can't, and
    TidyShop gets no access to the rest of your Drive.

    **Back up now** makes one straight away. Each backup in the list can be restored or deleted.
    **Turn off** stops the daily backup.

=== "Your family server"

    Once this phone has [joined a family](family.md), **Save to your family server → Back up now**
    uploads a backup to the connector. It keeps your **14 newest** backups and removes older ones
    automatically. Backups on the server are private to you; nobody else in the family can see
    them.

## Restoring

Pick a backup and TidyShop shows **Restore this backup?** with what it holds. **Replace my lists**
swaps your current lists for the ones in the backup. Lists that aren't in the backup are lost, so
save a backup of the current state first if you're unsure.

### On a new phone

The first screen of a fresh install has **Restore a backup**. Choose **File on this device**,
**Google Drive** or **Family connector**.

To restore from the family connector, the new phone first has to get back onto your account, not
join as someone new. Use **Reinstalled this phone? Reconnect it** with your recovery code, and your
backups come back with it.
