# Getting started

Setting up family sharing takes four steps. Most of the time goes into step 2, and if you
already expose other self-hosted apps over HTTPS it is five minutes of copy and paste.

1. **Run the container** — with [Docker Compose](docker.md) or on [Unraid](unraid.md).
2. **Put it behind HTTPS** — a [reverse proxy](reverse-proxy.md) with a real certificate, on a
   hostname your phones can reach from wherever they are.
3. **Connect your own phone** with the master pairing key. That makes you the family owner.
4. **Invite everyone else** with a QR code — [connect the app](connect-app.md).

## Before you start

You need:

- [x] A machine that is always on, running Docker or Unraid. A Raspberry Pi 4 is plenty.
- [x] A **hostname**, such as `tidyshop.example.com`, pointing at that machine — or at your
      router, with ports 80/443 forwarded to the reverse proxy.
- [x] A **reverse proxy** that can get a certificate for it (Let's Encrypt is fine).
- [x] TidyShop on each phone — it is in [closed beta](../beta.md) for now.
- [x] A **pairing key**: a random string of at least 12 characters. The install pages show how
      to generate one.

!!! warning "Plain HTTP will not work"
    Android blocks unencrypted connections for TidyShop, so `http://192.168.1.10:8787` cannot be
    used as the server address — not even on your home Wi-Fi. The address you enter in the app
    must start with `https://` and have a certificate the phone trusts.

!!! tip "Only on the home network?"
    A hostname that only resolves inside your network works, as long as it has a real certificate
    (for example through a DNS challenge). Lists then sync when phones are home and catch up
    automatically the next time they are.

## Why self-hosted?

Shared lists have to live somewhere both phones can reach. Rather than run a service that holds
every family's shopping habits, TidyShop lets you run that piece yourself. Your data sits in one
file on your own disk, and nothing about it reaches the developer.
