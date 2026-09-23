# Single sign-on

Single sign-on is **optional**. Without it, phones enrol with device keys and invite codes, and
that is the recommended setup for most households. Turn it on if you already run an identity
provider and want TidyShop members to be the same accounts as everywhere else in your homelab.

With SSO on, both methods are accepted side by side: a member may sign in through the identity
provider, or enrol a device key with an invite. Nobody is forced to use SSO.

## How it works

The app signs in with the authorization-code flow with PKCE, as a **public** client — there is no
client secret, in the app or on the connector. The connector reads your provider's OIDC discovery
document, fetches its public signing keys (JWKS), and checks every access token's signature,
issuer, audience and expiry. Any standards-compliant OIDC provider works; Authentik is the one
it's tested with.

## Authentik

1. **Applications → Providers → Create → OAuth2/OpenID Provider.**
    - *Name*: `TidyShop`
    - *Authorization flow*: your usual explicit or implicit consent flow
    - *Client type*: **Public**
    - *Client ID*: `tidyshop-android`
    - *Redirect URIs*: `com.jimz011apps.tidyshop://oauth2redirect` (strict)
    - *Signing key*: an **asymmetric** certificate, such as the *authentik Self-signed
      Certificate*. The connector verifies tokens with the public key, so a provider with no
      signing key (HS256) cannot work.
    - *Scopes*: `openid`, `profile`, `email`, `offline_access`
    - *Issuer mode*: per provider (the default)
2. **Applications → Applications → Create.**
    - *Name* `TidyShop`, *slug* `tidyshop`, *provider* the one you just made.
3. Your issuer URL is now:

    ```
    https://auth.example.com/application/o/tidyshop/
    ```

## Configure the connector

Set two variables and restart the container:

| Variable | Value |
| --- | --- |
| `OIDC_ISSUER` | `https://auth.example.com/application/o/tidyshop/` |
| `OIDC_AUDIENCE` | `tidyshop-android` (the default; only change it if your client id differs) |

The log confirms it:

```
single sign-on enabled for https://auth.example.com/application/o/tidyshop/
```

## Configure the app

In **Settings → Family sharing → Server & SSO**, enter the connector URL, the SSO issuer and the
client id. Invite links created afterwards carry these settings, so family members don't have to
type them.

## Other providers

Keycloak, Authelia, Zitadel, Pocket ID and others work the same way: create a public client with
PKCE, the redirect URI `com.jimz011apps.tidyshop://oauth2redirect`, an asymmetric signing
algorithm (RS256 or ES256), and make sure its access tokens are JWTs whose `aud` contains the
client id you set as `OIDC_AUDIENCE`.
