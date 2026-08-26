# Getting Started

> [!WARNING]
> **Pre-release.** `build`, `publish` and `verify` are not implemented yet. This
> page documents the intended interface so it can be reviewed before it is
> built. `archivist version` and `archivist inspect` work today.

## Install

Download a signed release archive:

```console
$ curl -fsSLO https://github.com/Guys-Inc-Public/archivist/releases/latest/download/archivist_linux_amd64.tar.gz
$ curl -fsSLO https://github.com/Guys-Inc-Public/archivist/releases/latest/download/checksums.txt
$ sha256sum --check --ignore-missing checksums.txt
$ tar -xzf archivist_linux_amd64.tar.gz archivist
$ sudo install -m 0755 archivist /usr/local/bin/
```

Verify the signature and provenance before you trust it — see
[Verification](Verification.md#verifying-an-archivist-release).

Or, if you have Go:

```console
$ go install github.com/Guys-Inc-Public/archivist/cmd/archivist@latest
```

## Configure

Create `archivist.yml` next to your build output. Every field is documented in
[Configuration](Configuration.md).

```yaml
origin: Example Project
label: Example Project packages
suite: stable
codename: stable
components: [main]
architectures: [amd64, arm64, armhf]
description: Packages for Example Project

signing:
  key_id: 246FE9B7D018E7A8C90694AF173719DDADE2ACFE

publish:
  bucket: example-apt
  endpoint: https://s3.example.com
  public_url: https://apt.example.com
```

Architectures are declared so the repository advertises them consistently even
in a release where one build failed. Packages are still read from their control
stanzas — `archivist` never infers architecture from a filename.

## Build a repository

```console
$ archivist build ./dist --config archivist.yml --out ./repo
```

The key is supplied through the environment, never a flag — a flag lands in
shell history and in the process table:

```console
$ export ARCHIVIST_GPG_KEY="$(cat signing-subkey.asc)"
```

## Try it before publishing

A local repository is a real repository. Point `apt` at it over `file://`:

```console
$ echo "deb [signed-by=$PWD/repo/public.asc] file://$PWD/repo stable main" \
    | sudo tee /etc/apt/sources.list.d/local-test.list
$ sudo apt update && apt policy your-package
```

If that works, the published version will too. This is the entire point of
having a CLI rather than only an Action.

## Publish

```console
$ archivist publish ./repo --bucket example-apt --endpoint https://s3.example.com
```

Credentials are read from the standard AWS environment variables
(`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`), so anything that already works
with the AWS CLI works here.

## In CI

```yaml
- uses: Guys-Inc-Public/archivist@v1
  with:
    packages: ./dist
    config: archivist.yml
    bucket: example-apt
    endpoint: https://s3.example.com
    version: v0.1.0        # pin it
  env:
    ARCHIVIST_GPG_KEY: ${{ secrets.APT_GPG_PRIVATE_KEY }}
    AWS_ACCESS_KEY_ID: ${{ secrets.R2_ACCESS_KEY_ID }}
    AWS_SECRET_ACCESS_KEY: ${{ secrets.R2_SECRET_ACCESS_KEY }}
```

Two concurrent publishes to one bucket will corrupt each other. Give the job a
`concurrency` group:

```yaml
concurrency:
  group: publish-apt
  cancel-in-progress: false
```

## What your users run

`archivist publish` prints the install snippet with your fingerprint and URL
already filled in, so the instructions cannot drift from the repository:

```console
sudo mkdir -p /etc/apt/keyrings
curl -fsSL https://apt.example.com/public.gpg | sudo tee /etc/apt/keyrings/example.gpg > /dev/null
echo "deb [signed-by=/etc/apt/keyrings/example.gpg] https://apt.example.com stable main" \
  | sudo tee /etc/apt/sources.list.d/example.list
sudo apt update && sudo apt install your-package
```
