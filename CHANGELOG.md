# Changelog

## 0.2.1 - 2026-11-13

### Changed

- Updated Go and frontend dependencies and the verified npm toolchain
  replacements, retaining Node 26 LTS for development and release builds.
- Moved development setup into the contributing guide and updated local check
  instructions to use mise.
- Moved **Remember Me** beside the Steam password prompt and added an option to
  forget a remembered password from the sign-in dialog.

### Fixed

- Show clear instructions when SteamCMD requests mobile approval during sign-in
  or upload, and continue automatically once approval is received.

[Full comparison](https://github.com/thornity/steamy-releaser/compare/v0.2.0...v0.2.1)

## 0.2.0 - 2026-11-10

### Added

- Set up a game in the desktop app with a guided form that detects project
  details, Godot export presets, and Unity build profiles, and previews the
  generated YAML before creating it.
- Discover or import SteamPipe app-build VDF scripts to populate app and depot
  IDs, content directories, file mappings, and exclusions.
- Sign in to Steam before opening a project. With permission, the desktop app
  offers account names saved by SteamCMD; credentials remain with SteamCMD.

### Changed

- Targets with different content directories can share a depot ID. By default,
  their contents go into depot folders named after the targets. Uploads must
  include every target sharing that depot.
- Expanded Steamworks setup documentation and examples, and pinned Node and
  npm for local desktop development with mise.
- Updated the release toolchain to Go 1.27.4 and current npm replacement
  packages. Local checks explain how to select the required Go version.

### Fixed

- Unity incremental builds can continue with a warning when the current build
  reports success but leaves an existing output unchanged.
- Creating a configuration can use the same depot ID for multiple targets.

Configurations that share a depot across different content directories require
Steam Releaser 0.2.0 or later.

[Full comparison](https://github.com/thornity/steamy-releaser/compare/v0.1.0...v0.2.0)

## 0.1.0 - 2026-11-05

- Initial open-source release.
