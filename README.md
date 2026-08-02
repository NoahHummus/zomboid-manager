# zomboid-manager

A small Discord bot for controlling a Project Zomboid server running as a
systemd service (`pzserver`) on the same host. Slash commands only:

- `/start` — `systemctl start <service>`
- `/stop` — `systemctl stop <service>`
- `/restart` — `systemctl restart <service>`
- `/logs [lines]` — `journalctl -u <service> -n <lines>` (default 50, max 200)

Access is restricted to an explicit allow-list of Discord user IDs.

## Configuration

Built with [cobra](https://github.com/spf13/cobra) +
[viper](https://github.com/spf13/viper). Settings can come from CLI flags,
`ZM_*` environment variables, or a `zomboid-manager.yaml` config file, in
that order of precedence. See [zomboid-manager.example.yaml](zomboid-manager.example.yaml).

| Flag                 | Env var              | Default    | Description                                            |
| -------------------- | --------------------- | ---------- | ------------------------------------------------------ |
| `--token`             | `ZM_TOKEN`             | *(none)*   | Discord bot token (required)                            |
| `--guild-id`          | `ZM_GUILD_ID`          | *(none)*   | Guild to register commands to instantly (omit for global, up to ~1h to propagate) |
| `--service-name`      | `ZM_SERVICE_NAME`      | `pzserver` | systemd unit name                                       |
| `--allowed-user-ids`  | `ZM_ALLOWED_USER_IDS`  | *(none)*   | Comma-separated Discord user IDs allowed to run commands (required) |
| `--journalctl-sudo`   | `ZM_JOURNALCTL_SUDO`   | `false`    | Run `journalctl` via `sudo` (see below)                 |
| `--config`            | —                      | *(none)*   | Explicit path to a config file                          |

## Build & run

```sh
go build -o zomboid-manager .
./zomboid-manager --token "$DISCORD_TOKEN" --allowed-user-ids "111111111111111111,222222222222222222"
```

## Host setup

The bot process needs permission to run `systemctl start/stop/restart` on
the `pzserver` unit without an interactive password prompt. Grant that with
a narrow sudoers rule rather than running the bot as root — replace
`botuser` with whatever system user runs the bot, and confirm the
`systemctl` path with `which systemctl`:

```
# /etc/sudoers.d/zomboid-manager
botuser ALL=(root) NOPASSWD: /usr/bin/systemctl start pzserver, /usr/bin/systemctl stop pzserver, /usr/bin/systemctl restart pzserver
```

For `/logs`, prefer adding the bot's user to the `systemd-journal` group
instead of using `--journalctl-sudo`:

```sh
sudo usermod -aG systemd-journal botuser
```

If your journald setup doesn't grant that group read access, set
`--journalctl-sudo`/`ZM_JOURNALCTL_SUDO=true` and add a matching sudoers
line for `journalctl -u pzserver`.

### Running the bot itself as a service

```ini
# /etc/systemd/system/zomboid-manager.service
[Unit]
Description=Zomboid Discord manager bot
After=network-online.target

[Service]
Type=simple
User=botuser
ExecStart=/opt/zomboid-manager/zomboid-manager
WorkingDirectory=/opt/zomboid-manager
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```sh
sudo systemctl enable --now zomboid-manager
```

## Discord app setup

1. Create an application at the [Discord Developer Portal](https://discord.com/developers/applications), add a Bot user, and copy its token.
2. Invite it to your server with the `applications.commands` and `bot` scopes (no elevated Discord permissions are needed since it only uses slash commands).
3. Get your Discord user ID (enable Developer Mode, right-click your name, "Copy User ID") and pass it via `--allowed-user-ids`.
