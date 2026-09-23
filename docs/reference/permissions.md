# Android permissions

TidyShop asks Android for as little as it can. There's no camera, contacts, location, storage or
microphone permission: invite QR codes are scanned with your phone's own camera app, and backup
files are saved through Android's file picker, which needs no permission.

| Permission | Why |
| --- | --- |
| **Internet** | Syncing with your family server, Google Drive backup, and Gravatar pictures, each only when you use it. |
| **Use biometrics** | Fingerprint or face unlock for the [app lock](../guide/security.md#app-lock), if you turn it on. |
| **Post notifications** | [Notifications](../guide/notifications.md) about family lists. Android 13 and newer asks you first. |
| **Foreground service** (data sync) | Only for the **Live** notification frequency, which keeps the connection to your family server open while the app is closed. No other setting starts a service. |
| **Run at startup** | Restarts **Live** delivery after the phone reboots or TidyShop updates. Scheduled checks survive a reboot without it. |
