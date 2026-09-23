# Backups and updates

## What to back up

Everything lives in the folder mounted at `/data`:

| File | Contents |
| --- | --- |
| `state.json` | The household, members, enrolled device public keys, shared lists, invites, and members' backups. |
| `pairing-key.txt` | The master pairing key, if you use the file rather than the variable. |
| `icon.png` | Optional; served at `/icon.png` if present. |

Copy that folder and you have backed up the connector. On Unraid, the **Appdata Backup** plugin
picks up `/mnt/user/appdata/tidyshop-connector` automatically.

`state.json` is written atomically — to a temporary file first, then renamed into place — so a copy
taken while the container runs is never half-written. It is created readable by its owner only;
keep it that way, as it contains every member's data.

## Members' backups

Members can save backups of their TidyShop data to the connector from the app. Each member keeps
their **14 most recent** backups, up to 2 MiB each; older ones are dropped automatically. Backups
are private to the member who made them.

These live inside `state.json`, so backing up the folder backs them up too.

## Updating

=== "Docker Compose"

    ```bash
    docker compose pull
    docker compose up -d
    ```

=== "Unraid"

    **Docker** tab → **Check for Updates**, then **apply update** on TidyShop-Connector. Or let
    the *Auto Update Applications* plugin do it.

Phones reconnect to the live stream on their own once the container is back; a full snapshot
of their lists arrives on reconnect, so nothing needs refreshing by hand.

### Image tags

| Tag | Meaning |
| --- | --- |
| `latest` | The newest release. What the compose file and Unraid template use. |
| `1`, `1.2`, `1.2.3` | Pin to a major, minor or exact release. |
| `edge` | Built from every commit to `main`. For testing only. |

Release notes are in the [changelog](../reference/changelog.md).

## Moving to another server

1. Stop the old container.
2. Copy the `/data` folder to the new machine.
3. Start the connector there with the same pairing key.
4. Point your hostname at the new machine.

As long as the **hostname stays the same**, phones don't notice. A new hostname means changing the
server address in the app on every phone.

## Starting over

Stop the container and delete `state.json`. The next person to pair with the pairing key becomes
the owner of a brand-new household. Phones that were connected need to leave the family and pair
again.
