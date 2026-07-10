# Botson-TUI

A standalone terminal chat client for a running [`botson
core`](https://github.com/xSaVageAU/Botson-ADKv2) — connect, pick an agent,
create or resume a session, and chat. It talks to the core purely over
NATS, the same way any other consumer (a Discord bot, a web UI, anything
else) would; there's no special access and no Go package shared with the
core itself.

This exists as a reference consumer: Botson-ADKv2's whole design is that
its *only* interface is NATS, and that any client, in any language, can
drive it. Botson-TUI is the first real proof of that — everything here
goes through the wire contract documented in
[`docs/nats-api.md`](https://github.com/xSaVageAU/Botson-ADKv2/blob/core-rebuild/docs/nats-api.md)
in the core's own repo.

Built with the [Charmbracelet](https://charm.sh) stack: bubbletea for the
app loop, bubbles for components (list, viewport, textarea, spinner,
help), lipgloss for styling, huh for the connect form, glamour for
markdown-rendered replies, and charmbracelet/log for file logging (the TUI
owns the terminal, so nothing prints to stdout while it's running).

## Building

```bash
go run scripts/build_linux.go     # or build_windows.go on Windows
```
This produces `bin/botson-tui-<os>-<arch>`.

## Running

You'll need a `botson core` already running somewhere reachable (see the
core's own README for how to start one). Then:

```bash
./bin/botson-tui-linux-amd64
```

On first run you'll be asked for the core's host/port, a user ID (*your*
choice; the core makes no assumptions about what one looks like), and a
NATS auth token — the core rejects unauthenticated connections. If the
core is running on this same machine, its token is auto-detected from
`~/.botson/config.json` (the same file `botson setup install` writes) and
the field is pre-filled; for a remote core, paste in the token it printed
at setup time. All of this is remembered for next time in
`<user config dir>/botson-tui/config.json` (on Windows,
`%AppData%\botson-tui\config.json`).

From there: pick an agent, then pick or create a session. Creating a new
session asks for an optional working directory — an absolute path the
agent's file/command tools should use for that session instead of the
core's own default workspace; leave it blank to use the default. Then
chat. Each tool call gets one status line (`▸ toolName  called · {args}`)
that updates in place as it moves through its lifecycle rather than
spawning a new line per stage — `? toolName  awaiting approval · {args}`
means it's waiting on you (the footer names which one `y`/`n` currently
answers); `✓ toolName  deferred` means the core queued a read-only call to
run after this turn's approvals and answered it for you automatically,
no action needed; `✓ toolName  done` or `✗ toolName  error · …` is the
final outcome.

## Scope

This is a v1, chat-only client: connect → pick agent → pick/create session
→ chat, including human-in-the-loop approvals. It does not yet cover
agent management or settings editing — both already exist as `botson.*`
NATS subjects on the core, just without a UI here yet.

## Logs

`<user config dir>/botson-tui/tui.log`.
