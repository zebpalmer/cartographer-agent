# Deployment & Release Contract

How cartographer-agent gets built, shipped, installed, and updated — and
the invariants each stage relies on. Change any of these and something
downstream breaks silently.

## Release flow

`stratt release patch|minor|major` verifies, bumps [stratt.toml](stratt.toml),
commits, tags `v<X.Y.Z>`, and pushes. The tag triggers
[release.yaml](.github/workflows/release.yaml): tests → GoReleaser
(per [.goreleaser.yaml](.goreleaser.yaml)) → GitHub Release → Sigstore
attestation of every asset.

## The contract

These invariants are what the fleet depends on. **Do not break them:**

1. **Asset names**: raw executable binaries (not archives) named
   `cartographer-agent_<os>-<arch>`, where `<arch>` is `amd64`, `arm64`,
   `arm6`, or `arm7` (GOARM level appended for 32-bit ARM). Enforced by
   `archives.name_template` + `formats: [binary]` in .goreleaser.yaml.
2. **Tag format**: `v<semver>` (e.g. `v1.5.0`). The GitHub download path is
   `releases/download/v<X.Y.Z>/cartographer-agent_<os>-<arch>`.
3. **Version stamp**: `main.Version` is set via ldflags to the tag without
   the `v`. The self-updater compares this against the commanded
   `target_version`, and `--version` output is used post-download to verify
   the binary runs.
4. **checksums.txt**: published alongside the binaries; the self-updater
   verifies each download's SHA-256 against it (releases predating it are
   tolerated with a warning).
5. **Prereleases**: `vX.Y.Z-rc.N` tags publish as GitHub *prereleases*
   (`prerelease: auto`), so `releases/latest` always points at a stable
   release. Safe to hand an rc version to a canary host; bulk rollout
   configs should reference stable versions.

## Install (bootstrap)

Ansible (`roles/cartographer` in lcg-ansible) installs the binary to
`/usr/local/bin/cartographer-agent` — only when the preflight
`--validate-config` fails (i.e. missing/broken binary; ansible never
upgrades a working install). It also renders `/etc/cartographer/config.yaml`
and the systemd unit. The role maps ansible facts to asset arch:
`x86_64→amd64`, `armv6l→arm6`, `armv7l→arm7` (needs `aarch64→arm64` for
future arm64 hosts).

## Updates (steady state)

1. **cartographer-daemon** (`AgentUpdateQueue`, every minute) decides who
   updates: per-channel target versions from `AGENT_VERSION_STABLE` /
   `_LATEST` / `_TESTING`, gated by `ENABLE_AGENT_UPDATES` and optionally
   `AGENT_UPDATE_PIN_MAJOR_VERSION` (skips cross-major jumps). It queues
   deploys and sends a NATS command `{action: "update", target_version: X}`.
2. **The agent** receives the command and runs `SelfUpdate`
   ([internal/selfupdate.go](internal/selfupdate.go)): builds the URL from
   the `release_url` config template — variables `{{.version}}`,
   `{{.osType}}` (GOOS), `{{.arch}}` (GOARCH + GOARM level on 32-bit ARM) —
   downloads to /tmp, verifies the SHA-256 against the release's
   checksums.txt, verifies `--version` runs, replaces itself, and runs
   `systemctl restart cartographer`. The systemd service name
   (`cartographer`) and binary path are load-bearing.

## Verifying provenance

Every release asset is attested (GitHub artifact attestations, Sigstore):

```sh
gh attestation verify cartographer-agent_linux-amd64 --repo zebpalmer/cartographer-agent
```

## Local builds

- `stratt run dev` — dev binary at `./bin/cartographer-agent`, stamped
  `dev-<git describe>` so it can never be mistaken for a release.
- `stratt run snapshot` (or `stratt build`) — full cross-platform matrix
  into `./dist` via GoReleaser, nothing published.
