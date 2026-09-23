# Joining a family

A **family** is everyone who shares lists through the same [family server](../server/index.md).
Somebody in the household runs that server. Everyone else just needs an invite.

Everything on this page is under **menu → Family sharing**.

## Join with an invite

The easiest way in. Ask someone already in the family to create an invite (see
[Invite someone](#invite-someone) below), then either:

- **scan the QR code** with your phone's normal camera app, or
- **tap the invite link** they sent you.

TidyShop opens and asks **Join this TidyShop family?**, showing the address of the family server and,
if the family uses single sign-on, the sign-in provider. Check that the server is the one you
expect. An invite holds only those public addresses and a single-use code, never a password.

Enter your name and tap **Enroll without SSO**. If your family uses single sign-on and you have an
account there, you can use **Continue with SSO** instead.

!!! warning "Save your recovery code"
    Right after joining, TidyShop shows a **recovery code** once. Write it down or put it in a
    password manager. It's the one thing that gets you back in after reinstalling TidyShop or
    switching phones. See [After a reinstall](#after-a-reinstall-or-on-a-new-phone).

### Joining by typing the details

Under **Pair with a family server**, enter the **Server address** (starting with `https://`) and
the **Invite code**, then tap **Join family**.

The person who runs the server uses the same screen with the **master pairing key** instead of an
invite code, plus a **Family name**, and taps **Pair server**. That makes them the family's owner.
See [Connect the app](../server/connect-app.md).

## Invite someone

In **Family sharing**, tap **Invite a new family member**. TidyShop shows:

- a **QR code** for them to scan with their camera app;
- the **code** itself, to read out;
- **Share invitation**, to send the link through any messenger.

An invite works **once** and **expires**, so it's safe to send over chat. You can **Revoke** an
invite that hasn't been used yet.

By default only the family owner can invite people. If you see *A family admin has not allowed
this account to invite new members*, ask them to switch on **May invite new members** for you.

## Your other devices

A tablet, or a second phone, should be **you** rather than a new person. Tap **Link another device
to this family** and scan the QR code on the other device, or type the code there. It joins as the
same family member and sees exactly your lists.

If a family administrator has limited you to one device, linking isn't available.

## After a reinstall, or on a new phone

Each phone's key lives in its hardware and can't be moved or backed up. After reinstalling
TidyShop, or on a new phone, **don't use a new invite**. That would make you a second, separate
family member with none of your lists. Instead, choose **Reinstalled this phone? Reconnect it**
on the pairing screen, enter the server address, and one of:

- a **link code** from another of your own devices, or from a family administrator;
- your **recovery code**.

Whoever runs the server can also use **Or, if you run this server → Master pairing key**, then
**Show the accounts on this server**, and pick the account the phone should return to.

After reconnecting you get your family lists and your backups on the server back, and a **new**
recovery code. Save that one too.

Lost your recovery code but still have a working phone? Under **This device**, **Show a new
recovery code** gives you a fresh one and retires the old one.

## Lists you can join

**Lists you can join** shows family lists whose owner shared them with **everyone**. Tap
**Subscribe** to add one to your phone.

## For family administrators

The family owner, and anyone who has verified the server's master pairing key, is a family
**administrator** and gets extra controls under **Members**:

| Control | Effect |
| --- | --- |
| **May invite new members** | Allow this member to create invites. |
| **May link multiple devices** | When off, this member can't link more than one device. Linking a new one replaces the old. |
| **Remove from family** | Revokes all of their devices. Lists they owned pass to you. |

To become an administrator on your own phone, for instance after the owner's phone was lost, enter
the server's master pairing key under **Enable family administration**.

**Shared-list management** lists every family list on the server, including ones nobody opens any
more, and lets an administrator **Delete permanently** a stale one. That removes it from the server
and every phone, and can't be undone.

## Leaving

**menu → Leave family** removes this phone from the family server. It stops receiving shared lists,
your lists stay on the phone, and the rest of the family is unaffected. You can rejoin later with a
new invite. To wipe the phone completely, use **Log out** in
[Profile](security.md#logging-out) instead.

## Try the demo family

Not sure family sharing is worth setting up a server for? Under **No server yet?**, tap **Explore a
demo family**. TidyShop adds two made-up family members and a couple of shared lists to your phone.
**Pretend a family member added something** shows exactly the notification you'd get.

Nothing is uploaded and no account is created. **Leave the demo** removes the demo lists again;
your own lists aren't touched.
