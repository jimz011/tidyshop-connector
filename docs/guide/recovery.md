# Recovery and the pairing key

A device's key lives in that phone's hardware and cannot be copied — not into a backup, not to a
new phone. That is what makes it safe, and it means a **reinstalled or replaced phone has to be
re-admitted** to the identity it had. Joining with a fresh invite would make it a new, second
person, who cannot see the first one's lists or backups.

In the app, choose **Reinstalled this phone? Reconnect it** on the pairing screen. There are three
ways back in; use whichever you have.

## 1. A device link code

If you still have another enrolled device — a tablet, the old phone — create a device link code
there and enter it on the new phone. Nothing else is needed.

The **owner** can also create a link code for somebody else's identity. That is the easiest way to
get a family member back in without handing them the pairing key.

## 2. Your recovery code

Every member is shown a 20-character **recovery code** once, when they first join. Entering it on
the new phone puts that phone back on their identity.

- The code is single-use. The app shows its replacement straight away — save that one too.
- The server only stores a hash of it, so it cannot be looked up later.
- Lost it but still have a working device? You can generate a new one from the app, which
  invalidates the old one.

This is the route for a member who has neither a second device nor the pairing key.

## 3. The master pairing key

Whoever runs the server can use the pairing key to reconnect a phone as **any existing member**:
the app lists the identities on the server and you pick the right one.

!!! danger "The pairing key is the administrator password"
    The pairing key can admit a new owner, claim the owner role, and reconnect a phone as any
    member — which includes access to that member's backups on the connector. Keep it in a
    password manager, don't send it to family members (send invites instead), and don't commit it
    to a public repository.

## Changing the pairing key

Stop the container, replace the key (the `PAIRING_KEY` variable or the contents of
`pairing-key.txt`), and start it again. Phones that are already enrolled are not affected: they
authenticate with their own device keys, not with the pairing key.

The key must be at least 12 characters, or the connector refuses to start.
