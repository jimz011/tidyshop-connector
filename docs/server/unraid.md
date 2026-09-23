# Install on Unraid

## Option A: Community Applications

1. Open the **Apps** tab and search for **TidyShop**.
2. Click **Install** on *TidyShop-Connector*.
3. Fill in the template as described in [Template settings](#template-settings) below, then
   click **Apply**.

!!! info "Not in the Apps tab yet?"
    Use option B — it installs exactly the same template.

## Option B: Add the template by hand

1. Open a terminal on your Unraid server (the **>_** icon at the top right) and download the
   template into your user templates folder:

    ```bash
    wget -O /boot/config/plugins/dockerMan/templates-user/my-TidyShop-Connector.xml \
      https://raw.githubusercontent.com/jimz011/tidyshop-connector/main/unraid/tidyshop-connector.xml
    ```

2. Go to **Docker → Add Container**.
3. In the **Template** drop-down, choose **TidyShop-Connector** under *User templates*.
4. Fill it in as below and click **Apply**.

## Template settings

| Setting | What to enter |
| --- | --- |
| **Pairing key** | A random string of at least 12 characters. Generate one in the Unraid terminal with `openssl rand -hex 24` and paste it in. Keep a copy in your password manager. |
| **WebUI / API port** | `8787`, unless that port is taken. This is the port your reverse proxy points at. |
| **Appdata** | `/mnt/user/appdata/tidyshop-connector` — where the connector keeps its state. |
| **OIDC issuer (optional)** | Leave empty, unless you want [single sign-on](sso.md). |

Under **Show more settings** you'll find the OIDC audience, the data file path and the pairing
key file. You don't need to change them.

!!! tip "Keeping the key out of the template"
    Unraid stores template values on the flash drive. If you would rather not have the key there,
    leave **Pairing key** empty and put it in a file instead:

    ```bash
    mkdir -p /mnt/user/appdata/tidyshop-connector
    openssl rand -hex 24 > /mnt/user/appdata/tidyshop-connector/pairing-key.txt
    chmod 600 /mnt/user/appdata/tidyshop-connector/pairing-key.txt
    ```

    The connector reads `/data/pairing-key.txt` whenever the variable is empty.

## Check that it runs

Click the container icon and choose **Logs**. You should see:

```
single sign-on disabled; devices enrol with an invite code
TidyShop-Connector listening on :8787
```

**WebUI** in the same menu opens `http://<your-server>:8787/health`, which answers
`{"service":"TidyShop-Connector","status":"ok"}`. There is no web interface beyond that —
everything is managed from the app.

If the container stops straight away with
`PAIRING_KEY or PAIRING_KEY_FILE must provide at least 12 characters`, the key is missing or too
short.

## Using a custom Docker network

If your reverse proxy (SWAG, Nginx Proxy Manager, …) runs on a custom network such as `proxynet`,
you can put the connector on it too: edit the container, set **Network Type** to that network,
and point the proxy at `TidyShop-Connector:8787` instead of `<server-ip>:8787`. The port mapping
can then be removed, so the unencrypted port is not exposed on your LAN at all.

## Next

[:octicons-arrow-right-24: Set up the reverse proxy](reverse-proxy.md)
