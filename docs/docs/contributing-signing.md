---
sidebar_position: 90
---

# Code signing

macOS blocks a binary that carries the quarantine flag unless the binary is
signed and notarized. A binary that Claude Desktop extracts from a downloaded
`.mcpb` carries that flag, so a release has to be signed.

The release workflow signs the macOS builds when the certificate is present, and
skips signing when it is absent. A fork builds without any of this.

## What GoReleaser does

GoReleaser signs with [quill](https://github.com/anchore/quill) rather than
Apple tooling, so signing runs on the Linux release runner. It signs the macOS
builds, submits them to Apple, and waits for the answer.

## Secrets the release needs

Add these five secrets in **Settings → Secrets and variables → Actions**.

| Secret | Holds |
| --- | --- |
| `MACOS_SIGN_P12` | The Developer ID Application certificate, as base64 |
| `MACOS_SIGN_PASSWORD` | The password on that certificate |
| `MACOS_NOTARY_ISSUER_ID` | The issuer ID of the App Store Connect key |
| `MACOS_NOTARY_KEY_ID` | The key ID of that key |
| `MACOS_NOTARY_KEY` | The `.p8` private key, as base64 |

Setting `MACOS_SIGN_P12` without the other four fails the release. That is
deliberate: a half configured release would publish unsigned binaries.

## Getting the certificate

1. Open the [Apple Developer portal](https://developer.apple.com/account/resources/certificates/list).
2. Create a certificate of type **Developer ID Application**. Do not pick
   Developer ID Installer, which signs installer packages rather than binaries.
3. Download it and open it, which adds it to the login keychain.
4. In Keychain Access, find the certificate, and export it with its private key
   as a `.p12` file. Set a password.
5. Convert the file to base64:

   ```sh
   base64 -i DeveloperID.p12 | tr -d '\n' | pbcopy
   ```

   Paste the result into `MACOS_SIGN_P12`, and the password into
   `MACOS_SIGN_PASSWORD`.

## Getting the notary key

1. Open [App Store Connect](https://appstoreconnect.apple.com/access/integrations/api).
2. Create a key under **Integrations → App Store Connect API**. Give it the
   **Developer** role, which is enough to notarize.
3. Copy the **Issuer ID** shown above the key list into
   `MACOS_NOTARY_ISSUER_ID`.
4. Copy the **Key ID** from the key row into `MACOS_NOTARY_KEY_ID`.
5. Download the `.p8` file. Apple offers it once.
6. Convert it to base64:

   ```sh
   base64 -i AuthKey_XXXXXXXX.p8 | tr -d '\n' | pbcopy
   ```

   Paste the result into `MACOS_NOTARY_KEY`.

## Checking a release

Notarization takes a few minutes, and the release waits for it. After the
release finishes, check a downloaded binary on a Mac:

```sh
codesign --verify --verbose=2 tekmetric-mcp-darwin-universal
spctl --assess --type execute --verbose tekmetric-mcp-darwin-universal
```

The first command reports the signature. The second reports whether Gatekeeper
accepts the binary.

## Windows

Windows builds are not signed. SmartScreen warns about an unsigned binary the
first time a user runs it.

Signing Windows needs a certificate whose private key sits on hardware or in a
cloud service, which the CA/Browser Forum has required since June 2023. A file
based certificate no longer qualifies.

[SignPath Foundation](https://signpath.org/) gives free certificates to open
source projects and works from GitHub Actions. It is the cheapest route for this
project. Azure Trusted Signing costs about ten dollars a month and needs no
application review.
