# Nexora

Nexora is a local service for connecting this computer to ChatGPT through OpenAI's official `tunnel-client`. Configuration, permissions, connection status, approvals, and diagnostics are managed in a browser-based web interface, so Nexora also works on headless servers without a desktop environment. Its local MCP compatibility endpoint listens only on `127.0.0.1:5060`; that port is retained for existing tunnel-client configuration and is not the product name. Nexora does not open an inbound internet port.

## Current MVP

- Serves a browser-based configuration interface split into Overview, Connection, Permissions, and Diagnostics tabs.
- Keeps the web UI bound to loopback; remote headless hosts are configured through SSH local port forwarding.
- Provides an MCP Streamable HTTP endpoint at `http://127.0.0.1:5060/mcp`.
- Provides system information, command jobs, file listing/read/write, desktop screenshot/window list/pointer/key/text, and browser URL opening tools.
- Saves the runtime key in the operating system credential store where available; falls back to a user-private config file when the credential store is unavailable.
- Requires a separate random token on every MCP request. The token is stored in the OS credential store (or the private fallback file) and injected by `tunnel-client` only to the local MCP origin using `MCP_EXTRA_HEADERS`.
- Starts the official tunnel client with the runtime key supplied through its environment, not command-line arguments. The UI separately checks the tunnel client's local `/readyz` endpoint.
- Starts `tunnel-client` with a filtered environment: only basic OS variables, locale, and documented proxy/CA/client-certificate settings are inherited, so unrelated app/cloud credentials are not passed to it.

The MCP HTTP endpoint uses the official [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk) and supports the stateless `2026-07-28` protocol plus handshake versions `2025-11-25`, `2025-06-18`, and `2025-03-26`. It returns JSON responses and does not open an SSE GET stream. Nexora keeps its own per-installation `X-5060-Token` check in front of the protocol handler. The official [`tunnel-client` v0.0.17 release notes](https://github.com/openai/tunnel-client/releases/tag/v0.0.17) mention `server/discover` support for `2026-07-28`; the [client overview](https://github.com/openai/tunnel-client) says self-contained requests using that protocol pass through without a handshake. This is source-level compatibility evidence; an actual ChatGPT connector run is still required. See the [MCP 2026-07-28 transport specification](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports) and [versioning rules](https://modelcontextprotocol.io/specification/2026-07-28/basic/versioning).

This is an early, unsigned developer build, not a finished consumer installer. It can run as a headless service; only optional screen and keyboard tools need an interactive desktop session. Command, file, desktop, browser-opening, and administrator permissions are off by default; older configurations receive a one-time migration that revokes older implicit grants, and browser-opening requires explicit opt-in. File access is limited to folders explicitly selected in the configuration UI; operations use Go's rooted filesystem handles to keep symlink traversal within the selected folder. Command execution still allows arbitrary shell commands when enabled: it can access or change anything the current account can access, including files outside the file-folder allowlist. `command_start` returns a job immediately; the command waits for one-time approval in the Nexora web interface, which expires after 90 seconds. If the Nexora service is stopped, the request times out without running. At most five jobs can run or await approval, and job history is capped at 20 entries. Command cancellation and its five-minute timeout attempt to stop the launched process tree. Automated Unix and Windows runtime checks are now included in CI, but have not yet run on remote runners; command cleanup still needs verification on each supported OS. Login auto-start is available in settings, but target-session behavior still needs verification. Signed installers/notarization, self-update, browser accessibility-tree reading, and fully native per-OS permission onboarding are not implemented yet. Windows desktop tools require a logged-in interactive desktop; macOS keyboard/mouse control requires Accessibility permission and `cliclick`; Linux keyboard/mouse/window controls use `xdotool` and `wmctrl` under X11. Wayland is not supported for full desktop input. If enabled in settings, `command_start` with `admin: true` requests macOS authorization, Linux PolicyKit authorization, or a Windows UAC prompt.

Read [Privacy and data flow](docs/privacy-data-flow.md) before connecting the app to ChatGPT. MCP tool arguments and results pass through OpenAI Tunnel; local execution does not mean tool data stays on the device.

## Build

Requires Go 1.27.4 or later. The build pins Go 1.27.4 to include current standard-library security fixes.

```sh
go build -o Nexora ./cmd/nexora
./Nexora
```

Local builds print `dev` with `./Nexora -version`. Release builds embed the version from the tag; the UI status and MCP `serverInfo` report the same value.

Nexora serves its configuration interface at `http://127.0.0.1:18080` with a temporary, per-launch access token. Start with `./Nexora -no-open` to print the setup URL without opening a browser. On a headless remote server, run `ssh -L 18080:127.0.0.1:18080 user@server`, then open the one-time URL printed by Nexora in your local browser. Existing `5060` user configuration and credential-store entries are migrated when Nexora first launches.

To build all supported OS/architecture binaries:

```sh
./scripts/build-all.sh
```

Set `VERSION` to the release version when building manually, for example `VERSION=1.2.3 ./scripts/build-all.sh`.
By default artifacts go to `dist/`; set `DIST_DIR` to a separate directory to create a reviewable preview without replacing another build's checksum manifest, for example `DIST_DIR=dist/preview VERSION=1.2.3-preview ./scripts/build-all.sh`.
The script also creates `.tar.gz` packages for macOS/Linux and `.zip` packages for Windows, with install, rollback, uninstall, and headless web-configuration instructions. Upgrades preserve one previous binary for a one-step rollback, and `dist/SHA256SUMS` covers both binaries and packages. On Windows, run `install-windows.ps1`; on macOS/Linux, run `install-unix.sh`. Start Nexora from a terminal or service manager so headless users can retrieve the one-time setup URL from its output. Use **退出 Nexora** in the setup page to stop the service and disconnect its Tunnel. These are portable archives, not signed native installers. Uninstall keeps settings and credentials by default. After explicit confirmation, it asks the installed app to delete its config files and entries from the OS credential store before removing itself; a credential-store failure stops uninstall and leaves the app available for recovery. If no interactive terminal is available, settings and credentials are kept. Windows removes the application directory and rollback copy; Unix retains `~/.local/bin/Nexora.previous` for manual recovery. Linux removes menu/icon files only when their contents still match the installer's recorded SHA256.

GitHub tag releases additionally publish per-binary SPDX SBOMs and Sigstore build provenance attestations. Before upload, the release workflow verifies the exact 18 versioned binaries, archives, and SBOMs, validates all 12 binary/archive checksums, and rejects stale or unexpected files. See [Release verification](docs/release-verification.md) for checksum and provenance checks.

## Connect a tunnel

1. Install official `tunnel-client` v0.0.17 or later. On macOS, run `brew install openai/tools/tunnel-client`, then `tunnel-client --version`; this is OpenAI's supported installation path. Direct release ZIPs are not notarized, so do not bypass Gatekeeper. On Windows and Linux, download the ZIP matching your OS and CPU (`amd64` means x86_64; `arm64` means ARM) from the [OpenAI release page](https://github.com/openai/tunnel-client/releases), then follow its [SHA256 and Sigstore provenance verification steps](https://github.com/openai/tunnel-client#validate-a-downloaded-release). Those steps require GitHub CLI (`gh`). Nexora checks the minimum version before startup but does not verify the binary's signature, checksum, or download source.
2. Create or select a Platform tunnel, then create a Restricted Runtime API Key with only Tunnels Read + Use access to that tunnel. Do not use an admin key for the long-running client.
3. In the Nexora web interface, open the **连接** tab and enter the tunnel ID and runtime key. Download the matching OpenAI `tunnel-client` build for your OS and CPU architecture from the official releases page, then use **选择文件** to locate it; if it is already on `PATH`, keep the default `tunnel-client` value. Save and start.
4. Wait for the UI to report **Tunnel 已就绪**.
5. In ChatGPT, add a custom MCP server using the Tunnel connection and select the same tunnel. Set its display name to `Nexora`.

ChatGPT workspace access and Platform tunnel access are controlled by OpenAI. Keep the connector private and grant Tunnel Use only to the intended account. Nexora's HTTP services accept loopback requests from approved local Host values. Its management APIs also require a per-process `X-Nexora-UI-Session` credential. The `/mcp` endpoint separately requires the private `X-5060-Token` header. The Tunnel client injects it for requests to this MCP origin; unauthenticated local MCP calls receive HTTP 401.

## Local configuration

Configuration is stored under the current user's OS config directory in `Nexora/config.json`. Secrets use the platform credential store where supported. The fallback file `secrets.json` is created with owner-only permissions on Unix-like systems. On Windows it resides inside the current user's profile and relies on that profile's ACL. Existing `5060` config and secrets are copied to the new directory at first launch. The Nexora web interface can clear the locally saved Tunnel Runtime API Key while keeping the local MCP token. This does not revoke the key in OpenAI Platform; revoke it there separately if it may be compromised.

## MCP tools

`system_info`, `command_start`, `command_status`, `command_stop`, `file_list`, `file_read`, `file_write`, `desktop_screenshot`, `desktop_windows`, `desktop_pointer`, `desktop_key`, `desktop_type`, `browser_open`.

Command and file tools normally run as the account that launched Nexora. File access is confined to the folders listed in the local settings; directory listings are capped at 1,000 entries, and file read/write requests at 1 MiB. Admin command requests require both the setting to be enabled and the operating system's authorization prompt. Commands run for at most five minutes; command output is capped at 1 MiB per stream.
