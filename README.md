# motd

A small terminal welcome for Linux hosts and Macs. It shows the hostname and running OS version, with optional artwork, local weather, and public IPv4.

## Install

Download the binary for your OS and architecture from [Releases](https://github.com/ironicbadger/motd/releases), or build it with Go 1.25 or later.

```sh
CGO_ENABLED=0 go build -trimpath -o motd .
./motd show
```

The binary embeds its artwork. On Linux it reads `os-release`, including the full image version reported by uCore. On macOS it reads the version via `sw_vers`. Fedora (including uCore) and Ubuntu have distro artwork. Ghostty and Kitty display inline images, and other color terminals use ANSI art. Use `motd show -ansi` to force ANSI, or `-color never` for text only.

## Customize

Optional JSON config lives at `~/.config/motd/config.json` on Linux (or under `$XDG_CONFIG_HOME`) and `~/Library/Application Support/motd/config.json` on macOS. Pass `-config /path/to/config.json` to both commands to use another file.

```json
{
  "artwork": "os",
  "hostname": "gateway",
  "hide_hostname": false,
  "hide_os": false,
  "location": "Toronto, CA",
  "weather": { "latitude": 43.65, "longitude": -79.38 },
  "public_ipv4": "masked"
}
```

- Set `artwork` to `os`, `none`, or `flag` with `"country": "ca"`, `"us"`, or `"gb"`. Preview the flags with `motd flags`.
- Use `hostname` as a display alias, or set `hide_hostname` or `hide_os` to omit those fields. Omit `location` or `weather` to leave each out.
- Public IPv4 defaults to `hidden`. Set `masked` to display an address like `203.0.x.x`, or `full` to show all four octets. Masked mode hides the last two octets in the banner. When enabled, refresh stores the full address in the user-private cache. With `hidden`, refresh skips the IP lookup and clears any previous address from the cache.

## Login and refresh

Run `motd show` from an interactive login hook. It reads local data and the cache without making network calls. Run `motd refresh` separately from a timer or cron as the same user, for example every 20 minutes. Missing or expired readings are omitted.

Weather is provided by [Open-Meteo](https://open-meteo.com/). Public IPv4 is measured by [ipify](https://www.ipify.org/) from the machine running refresh. Flag artwork comes from [Flagcdn](https://flagcdn.com/), distro artwork from [Simple Icons](https://simpleicons.org/), and the ANSI variants were rendered with [Chafa](https://hpjansson.org/chafa/). Fedora and Ubuntu are trademarks of their respective owners.
