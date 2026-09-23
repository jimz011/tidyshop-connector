# Members and invites

A connector holds one household. Everyone in it is a **member**, and each member has one or more
**devices**.

## Roles

| | Owner | Member |
| --- | --- | --- |
| Share their own lists, at View, Check or Edit | :material-check: | :material-check: |
| Link another device of their own | :material-check: | :material-check: (unless limited to one) |
| Create invites for new people | :material-check: | only when allowed |
| Allow or stop a member inviting people | :material-check: | |
| Limit a member to one device | :material-check: | |
| Link a device for *another* member | :material-check: | |
| Remove a member | :material-check: | |

The first person to pair with the master pairing key is the owner. Anyone holding the pairing key
can also claim the owner role later from their own, already-joined phone — which is how a
household gets its administrator back if the owner's phone is gone.

## Invites

An invite admits one new person. It is:

- **single-use** — spent the moment somebody joins with it;
- **short-lived** — 24 hours by default, seven days at most;
- **revocable** — until it is used, by the member who made it or by the owner.

An invite link or QR code carries only public settings: the server address, the invite code, and
— when [single sign-on](sso.md) is on — the identity provider's address and public client id. It
never contains the pairing key, a password or a token. Before accepting it, the app shows the
server's hostname and asks for confirmation, so a link cannot quietly point a phone at somebody
else's server.

## Devices

Every phone or tablet enrols its own key pair; the private key never leaves the device's
hardware keystore. To add another device **for the same person**, create a *device link code* on a
device that is already enrolled. Link codes are valid for up to 24 hours and are also single-use.

An owner can limit a member to **one device**. Linking a new device for that member then replaces
the old one rather than adding a second — useful for a child's phone.

Each member can see and revoke their own devices. Revoking deletes the device's public key, so it
is locked out on its very next request; there is no session left to expire.

## Removing a member

Only the owner can remove a member. Removal revokes every device that member had, cancels their
open invites and recovery code, and they stop receiving the household's lists immediately. Lists
the removed member owned are handed over to the owner rather than deleted. To leave a household yourself, use **Leave family**
in the app instead.
