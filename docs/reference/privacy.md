# Privacy

The full, binding text is the **[TidyShop privacy policy](https://jimz011.github.io/tidyshop/privacy.html)**.
This page is the short version.

## The short version

- **Your lists stay on your phone.** There's no TidyShop account and no TidyShop cloud.
- **The developer receives nothing.** No analytics, no crash reporting, no advertising identifier,
  no ads.
- **Anything that leaves your phone goes somewhere you chose**, and only once you turn it on:

| Feature | What's sent | Where |
| --- | --- | --- |
| [Family sharing](../guide/family.md) | Family lists, your name, your device's public key, your server backups | The family server your household runs |
| [Google Drive backup](../guide/backup.md) | Your backups | Your own Google Drive, in the app's private area |
| [Single sign-on](../server/sso.md) | Sign-in, handled by your identity provider | The identity provider your household runs |
| [Gravatar picture](../guide/security.md#your-profile) | A one-way SHA-256 hash of your email | Gravatar |
| [Send a copy](../guide/sharing.md#send-a-copy) | The list, inside the link | Whoever you send it to |

## What's stored where

- **On your phone**: lists, profile, settings, and your app-lock PIN, stored only as a salted
  hash. Uninstalling TidyShop deletes all of it.
- **On a family server**: family lists, household members, and, only if you save them there, your
  backups. Personal lists aren't sent unless they're inside a backup you uploaded. A device's
  private key never leaves the phone's hardware keystore. The server only knows the
  public half.

Because the developer holds no copy of your data, there's nothing for the developer to delete. You
stay in control of every copy.
