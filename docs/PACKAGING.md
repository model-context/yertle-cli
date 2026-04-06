# Packaging & Distribution

How to make `yertle` installable as a proper package across platforms.

## Prerequisites

- GoReleaser (`brew install goreleaser`)
- A GitHub personal access token with `repo` scope
- A GitHub repo for the Homebrew tap (see step 2)

---

## macOS: Homebrew Tap

### 1. Create the tap repo

Create a **public** GitHub repo named `homebrew-yertle` under your account or org (e.g., `youruser/homebrew-yertle`). It can be empty -- GoReleaser will push the formula file automatically.

The naming convention `homebrew-<name>` is required by Homebrew. Users will run:

```bash
brew tap youruser/yertle
brew install yertle
```

### 2. Add GoReleaser config

Create `.goreleaser.yaml` in the root of this repo:

```yaml
project_name: yertle

builds:
  - env:
      - CGO_ENABLED=0
    goos:
      - darwin
      - linux
      - windows
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w -X main.version={{.Version}}

archives:
  - format: tar.gz
    name_template: "{{ .ProjectName }}_{{ .Os }}_{{ .Arch }}"
    format_overrides:
      - goos: windows
        format: zip

brews:
  - repository:
      owner: <your-github-user-or-org>
      name: homebrew-yertle
    homepage: "https://github.com/<your-github-user-or-org>/yertle-cli"
    description: "CLI for exploring the Yertle platform"
    install: |
      bin.install "yertle"

changelog:
  sort: asc

release:
  github:
    owner: <your-github-user-or-org>
    name: yertle-cli
```

Replace `<your-github-user-or-org>` in all three places.

### 3. Add a version variable to main.go

So that `yertle --version` prints the release tag:

```go
var version = "dev"
```

Then set `Version: version` on your root Cobra command. The `-X main.version={{.Version}}` ldflag in the GoReleaser config injects the git tag at build time.

### 4. Tag and release

```bash
git tag v0.1.0
git push origin v0.1.0

export GITHUB_TOKEN=<your-token>
goreleaser release --clean
```

This will:
- Cross-compile for macOS, Linux, and Windows (amd64 + arm64)
- Create a GitHub Release with tarballs/zips attached
- Push a Homebrew formula to your `homebrew-yertle` repo

### 5. Verify

```bash
brew tap youruser/yertle
brew install yertle
yertle --version
```

### Updating

For each new release, tag and run `goreleaser release --clean` again. The formula in the tap repo is updated automatically.

---

## Linux

### Option A: Direct binary download

GoReleaser already produces Linux tarballs in the GitHub Release. Users can:

```bash
curl -LO https://github.com/youruser/yertle-cli/releases/latest/download/yertle_linux_amd64.tar.gz
tar xzf yertle_linux_amd64.tar.gz
sudo mv yertle /usr/local/bin/
```

### Option B: Homebrew on Linux

Homebrew works on Linux too. The same tap works:

```bash
brew tap youruser/yertle
brew install yertle
```

### Option C: APT / YUM repos (future)

GoReleaser supports generating `.deb` and `.rpm` packages via its `nfpms` config. Add this to `.goreleaser.yaml`:

```yaml
nfpms:
  - package_name: yertle
    homepage: "https://github.com/youruser/yertle-cli"
    description: "CLI for exploring the Yertle platform"
    license: ""
    formats:
      - deb
      - rpm
    bindir: /usr/local/bin
```

This produces `.deb` and `.rpm` files attached to the GitHub Release. Users can install directly:

```bash
# Debian/Ubuntu
sudo dpkg -i yertle_0.1.0_linux_amd64.deb

# RHEL/Fedora
sudo rpm -i yertle_0.1.0_linux_amd64.rpm
```

Hosting a proper APT/YUM repository (so users can `apt install yertle`) requires a package registry like Cloudsmith, Packagecloud, or a self-hosted repo. That's more infrastructure -- worth doing once there's real Linux adoption.

---

## Windows

### Option A: Direct download

GoReleaser produces `.zip` archives for Windows. Users download from the GitHub Release and add `yertle.exe` to their PATH.

### Option B: Scoop (Windows package manager)

Scoop is the closest Windows equivalent to Homebrew. GoReleaser supports it natively. Add to `.goreleaser.yaml`:

```yaml
scoops:
  - repository:
      owner: <your-github-user-or-org>
      name: scoop-yertle
    homepage: "https://github.com/youruser/yertle-cli"
    description: "CLI for exploring the Yertle platform"
    license: ""
```

Create a `scoop-yertle` repo on GitHub (like the Homebrew tap). Then users install with:

```powershell
scoop bucket add yertle https://github.com/youruser/scoop-yertle
scoop install yertle
```

### Option C: Chocolatey / winget (future)

Both require more setup (package submission, review process). Worth considering later.

---

## Summary

| Platform | Method | Effort | User experience |
|----------|--------|--------|-----------------|
| macOS | Homebrew tap | Low (GoReleaser automates it) | `brew install yertle` |
| Linux | Binary download | Free (GoReleaser produces it) | curl + mv |
| Linux | Homebrew | Free (same tap) | `brew install yertle` |
| Linux | .deb/.rpm | Low (add `nfpms` config) | `dpkg -i` / `rpm -i` |
| Windows | Binary download | Free (GoReleaser produces it) | Manual PATH setup |
| Windows | Scoop | Low (add `scoops` config) | `scoop install yertle` |

Start with the Homebrew tap. Everything else layers on top of the same GoReleaser release with minimal extra config.
