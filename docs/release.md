# Releasing PRadar

PRadar ships as one macOS application bundle built from a clean checkout. The
build needs no drawing tool and no JavaScript toolchain: the icon is generated
by `tools/mkicon` and the interface is served by the application itself.

## Build

```sh
make app                  # build/PRadar.app, versioned from git describe
make app VERSION=1.2.0    # or an explicit version
```

The target compiles the binary with trimmed paths, generates the icon set and
the `.icns`, writes `Info.plist` with the bundle identifier, the version, the
icon and the minimum macOS version, then prints the version the bundle
reports. `make clean-build` removes the whole output directory.

## Sign and notarise

Both steps read credentials the operator supplies; the repository stores none
of them.

| What you supply | How | Used by |
| --- | --- | --- |
| Developer ID Application certificate | Installed in your login keychain | `DEVELOPER_ID` |
| Notarisation credentials | `xcrun notarytool store-credentials <profile>` | `NOTARY_PROFILE` |

```sh
make sign DEVELOPER_ID="Developer ID Application: Your Name (TEAMID)"
make notarise DEVELOPER_ID="..." NOTARY_PROFILE="pradar-notary"
make release  DEVELOPER_ID="..." NOTARY_PROFILE="pradar-notary"
```

`sign` hardens the runtime and verifies the signature. `notarise` submits the
archive, waits for Apple, staples the ticket and validates it. `release`
produces `build/PRadar-<version>.zip`, the file you distribute.

## Before distributing

1. `make verify` passes, and the worktree is clean.
2. `make app` from a fresh clone, on a machine that never built PRadar.
3. Launch the bundle on a machine without a data directory: it must open its
   window, create the directory and offer to follow a first repository.
4. `codesign --verify --strict` and `xcrun stapler validate` both succeed.
5. The version the application reports matches the tag you are publishing.
6. The release notes name the prompt, skill, engine and contract versions, as
   an analysis produced by another combination is not comparable.
