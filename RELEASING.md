# Release Process

This project uses GoReleaser to build and publish cross-platform binaries for both the Tetris client and server.

## How It Works

- **Client** and **Server** have independent versions using tag prefixes
- GoReleaser automatically builds for Linux, macOS, and Windows (amd64 + arm64)
- Releases are triggered automatically when tagged commits are merged to main

## Creating a Release

1. **Create your PR** with the changes
2. **Tag the PR commit** (or wait until after merge and tag the merge commit):
   ```bash
   # For client release
   git tag client/v0.2.0

   # For server release
   git tag server/v1.0.0

   # For both (same commit, different versions)
   git tag client/v0.2.0
   git tag server/v1.0.0
   ```

3. **Push tags** (can be before or after merge):
   ```bash
   git push origin client/v0.2.0 server/v1.0.0
   # Or push all tags:
   git push origin --tags
   ```

4. **Merge to main** - GitHub Actions will automatically:
   - Detect tags on the merged commit
   - Build binaries for all platforms
   - Create GitHub releases (one per component)
   - Upload all binaries and checksums

### Releasing Both Client and Server

If your PR changes both components:
```bash
# Tag the same commit with both versions
git tag client/v0.3.0
git tag server/v2.1.0
git push origin --tags

# Merge to main
# Both releases will be created automatically
```

## Supported Platforms

Each release includes binaries for:
- **Linux**: amd64, arm64
- **macOS**: amd64, arm64
- **Windows**: amd64, arm64

## Local Testing

Test the release process locally:

```bash
# Test client build
COMPONENT=client BINARY_NAME=tetris goreleaser build --snapshot --clean --single-target

# Test server build
COMPONENT=server BINARY_NAME=tetris-server goreleaser build --snapshot --clean --single-target

# Full release dry-run (no upload)
COMPONENT=client BINARY_NAME=tetris goreleaser release --snapshot --clean
```

## Version Detection

The Makefile uses `git describe` to auto-detect versions:

```bash
# Check current versions
make version

# Client version only
make client-version

# Server version only
make server-version
```

Version format: `v0.1.0-5-gabc1234` where:
- `v0.1.0` - Last tag
- `5` - Commits since tag
- `gabc1234` - Current commit hash
