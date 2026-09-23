# Connect the app

With the connector reachable over HTTPS, the rest happens in TidyShop.

## Become the family owner

Do this on your own phone first. The first person to connect with the pairing key becomes the
**family owner**: the one who removes members and decides what everyone else may do.

1. Open TidyShop and go to **Settings → Family sharing**.
2. Choose **Pair with a family server**.
3. Fill in:
    - **Server address** — `https://tidyshop.example.com`
    - **Master pairing key** — the key you generated during installation
    - **Family name** — whatever your household should be called
4. Tap **Pair server**.

The phone creates its own key pair in the Android keystore and registers the public half with the
connector. You're in.

!!! warning "Save your recovery code"
    Right after pairing, the app shows a **recovery code** once. Write it down or store it in your
    password manager. It is how this identity gets back in after a reinstall or on a new phone —
    see [Recovery and the pairing key](recovery.md).

## Invite the rest of the family

1. In **Family sharing**, create an **invite**.
2. The app shows a QR code and a shareable link. Either works:
    - let the other person scan the QR code with their phone's camera, **or**
    - send them the link through any messenger.
3. On their phone, TidyShop opens with the server address and invite code already filled in, and
   asks them to confirm the server's hostname. They tap **Join family**.

Invites are single-use and expire after 24 hours by default (seven days at most). Unused invites
can be revoked from the same screen. Nobody but you ever needs the pairing key.

!!! tip "Only the owner invites, by default"
    The owner can let individual members invite people too, with **May invite new members**. See
    [Members and invites](members.md).

## Share a list

Open a list, tap share, and pick who gets it and at which level:

| Level | Can do |
| --- | --- |
| **View** | See the list. |
| **Check** | Tick items off and back on — the one for whoever is doing the shopping. |
| **Edit** | Add, change and remove items. |

A list can also be shared with **everyone** in the household at one level, and members can
subscribe to lists that are open to them from the family server settings.

## Add a second device of your own

A tablet or second phone should be linked to **your** identity, not invited as a new person.
Create a **device link code** under your devices in Family sharing and enter or scan it on the
other device. It then sees exactly what you see.

## Try it first

Not sure yet whether it's worth setting up a server? The pairing screen offers a **demo family**
that puts a made-up household and a couple of shared lists on your phone, without uploading
anything.
