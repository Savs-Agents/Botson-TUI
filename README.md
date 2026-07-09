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
go build -o bin/botson-tui ./cmd/botson-tui
```

## Running

You'll need a `botson core` already running somewhere reachable (see the
core's own README for how to start one). Then:

```bash
./bin/botson-tui
```

On first run you'll be asked for the core's host/port and a user ID —
this is *your* choice; the core makes no assumptions about what a user ID
looks like. These are remembered for next time in
`<user config dir>/botson-tui/config.json` (on Windows,
`%AppData%\botson-tui\config.json`).

From there: pick an agent, pick or create a session, and chat. If a tool
call needs your approval, the prompt will say so — press `y` to approve or
`n` to deny.

## Scope

This is a v1, chat-only client: connect → pick agent → pick/create session
→ chat, including human-in-the-loop approvals. It does not yet cover
agent management or settings editing — both already exist as `botson.*`
NATS subjects on the core, just without a UI here yet.

## Logs

`<user config dir>/botson-tui/tui.log`.
