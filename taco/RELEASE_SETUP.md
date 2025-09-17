# Release-Please Setup for Taco CLI and Statesman

This document describes the Release-Please configuration for managing releases of the Taco CLI and Statesman service.

## Overview

The setup includes:
- **Taco CLI**: Command-line interface for OpenTaco
- **Taco Statesman**: State management service

Both components are released with tags like `taco/cli/{version}` and `taco/statesman/{version}`.

## Files Created/Modified

### Release-Please Configuration
- `.github/release-please-config.json` - Main configuration for Release-Please
- `.github/workflows/release-please.yml` - GitHub Actions workflow for automated releases

### Version Management
- `taco/cmd/taco/version.txt` - Version file for CLI (starts at 0.0.0)
- `taco/cmd/statesman/version.txt` - Version file for Statesman (starts at 0.0.0)
- `taco/CHANGELOG.md` - Changelog for both components

### Build System Updates
- `taco/Makefile` - Added release build targets and version support
- `taco/Dockerfile_statesman` - Updated with version build args
- `go.work` - Added taco modules to workspace

### Code Updates
- Added version and commit variables to both main.go files
- Added version command to CLI (`taco version`)
- Added version information to health endpoint (`/healthz`)

## Release Process

### Automatic Releases
1. Push changes to `main` branch
2. Release-Please creates/updates PRs with version bumps
3. When PRs are merged, releases are automatically created
4. Binaries are built for multiple platforms (Linux, macOS, Windows)
5. Docker images are built and pushed to GitHub Container Registry

### Manual Release
```bash
# Build release binaries locally
make release-build

# Build Docker image
make docker-build-svc
```

## Release Artifacts

### CLI Releases (`taco/cli/{version}`)
- `taco-linux-amd64`
- `taco-linux-arm64`
- `taco-darwin-amd64`
- `taco-darwin-arm64`
- `taco-windows-amd64.exe`
- `taco-windows-arm64.exe`

### Statesman Releases (`taco/statesman/{version}`)
- `statesman-linux-amd64`
- `statesman-linux-arm64`
- `statesman-darwin-amd64`
- `statesman-darwin-arm64`
- `statesman-windows-amd64.exe`
- `statesman-windows-arm64.exe`
- Docker image: `ghcr.io/{repo}/taco-statesman:{version}`

## Usage

### CLI
```bash
# Check version
./taco version

# Get help
./taco --help
```

### Statesman Service
```bash
# Run locally
./statesman --help

# Run in Docker
docker run -p 8080:8080 -e OPENTACO_STORAGE=memory -e OPENTACO_AUTH_DISABLE=true statesman:latest

# Check health and version
curl http://localhost:8080/healthz
```

## Version Information

Both components include version and commit information:
- Version is read from `version.txt` files
- Commit SHA is automatically detected from git
- Version info is available via CLI `version` command and health endpoint

## Next Steps

1. Push the changes to trigger the first Release-Please PR
2. Review and merge the PR to create the first release
3. Monitor the GitHub Actions workflow for successful builds
4. Verify releases are created with proper artifacts
