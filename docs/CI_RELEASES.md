# Automated Releases with GitHub Actions

Today, releases are run manually with `make release`. This doc describes how to move that to GitHub Actions so a release happens automatically whenever a `v*` tag is pushed.

## Why

- **No local secrets** — `GITHUB_TOKEN` is provided automatically by Actions; no personal access tokens to manage on your laptop
- **Reproducible builds** — clean environment every time
- **Faster** — GitHub's runners cross-compile in parallel
- **One-step releases** — push a tag, the release happens

After setup, the entire release flow is:

```bash
git tag v0.1.0
git push origin v0.1.0
```

## Setup

### 1. Create a token for the Homebrew tap

The auto-provided `GITHUB_TOKEN` in Actions only has access to the *current* repo (`yertle-cli`). Since GoReleaser also needs to push to `homebrew-yertle` (a different repo), you need a separate token.

1. Go to **GitHub > Settings > Developer settings > Personal access tokens > Fine-grained tokens**
2. Click **Generate new token**
3. Name: `yertle-tap-push`
4. Resource owner: `model-context`
5. Repository access: **Only select repositories** → `homebrew-yertle`
6. Permissions: **Contents: Read and write**
7. Generate and copy the token

### 2. Add the token as a repo secret

1. Go to **model-context/yertle-cli > Settings > Secrets and variables > Actions**
2. Click **New repository secret**
3. Name: `TAP_GITHUB_TOKEN`
4. Value: (paste the token from step 1)

### 3. Update `.goreleaser.yaml`

The `brews` section needs to reference the tap token explicitly:

```yaml
brews:
  - repository:
      owner: model-context
      name: homebrew-yertle
      token: "{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}"
    homepage: "https://github.com/model-context/yertle-cli"
    description: "CLI for exploring the Yertle platform"
    license: "MIT"
    install: |
      bin.install "yertle"
```

### 4. Add the workflow file

Create `.github/workflows/release.yml`:

```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write

jobs:
  goreleaser:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0  # GoReleaser needs full history for changelogs

      - uses: actions/setup-go@v5
        with:
          go-version: '1.26'

      - uses: goreleaser/goreleaser-action@v6
        with:
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          HOMEBREW_TAP_GITHUB_TOKEN: ${{ secrets.TAP_GITHUB_TOKEN }}
```

### 5. Commit and test

```bash
git add .github/workflows/release.yml .goreleaser.yaml docs/CI_RELEASES.md
git commit -m "Add GitHub Actions release workflow"
git push

# Then trigger a release by pushing a tag
git tag v0.1.1
git push origin v0.1.1
```

Watch the workflow run under **Actions** in the GitHub UI. If it succeeds, the new version will appear on the GitHub Releases page and the formula in `homebrew-yertle` will be updated.

## Keeping `make release` as a fallback

You can keep the existing `make release` target as a backup for emergency local releases. Both paths use the same `.goreleaser.yaml`, so they produce identical results.

## Troubleshooting

- **"resource not accessible by integration"** — The `TAP_GITHUB_TOKEN` is missing or doesn't have write access to `homebrew-yertle`
- **"git is in a dirty state"** — A workflow step is modifying tracked files before GoReleaser runs. Check the `setup-go` cache and any custom steps
- **Tag already exists** — Delete the tag locally and remotely (`git tag -d vX.Y.Z && git push origin --delete vX.Y.Z`) before re-tagging

## Optional enhancements

- **Run tests before release** — add a `go test ./...` step before the GoReleaser action
- **Require manual approval** — use a GitHub Environment with required reviewers for the release job
- **Slack/Discord notifications** — GoReleaser supports announce hooks; configure them in `.goreleaser.yaml`
