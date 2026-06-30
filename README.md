<p align="center">
  <img src="assets/logo.png" alt="minfin" width="200">
</p>

# minfin

A personal finance app I actually want to use. Every other one is bloated,
nags you to upgrade, or hides your own data behind a subscription. `minfin` is
single-user, local, and reads from [SimpleFIN](https://www.simplefin.org/). 
It syncs your accounts and transactions into a local SQLite file
and shows you balances, spending, and categories. That's it.

Very early. Expect rough edges.

## Build & run

#### Dependencies
- Go 1.26+.
- libadwaita-dev

```sh
make run        # or: make build && ./bin/minfin
```

Then open http://localhost:8080.

Config (all optional):
- `PORT` — HTTP port (default `8080`)
- `MINFIN_DB` — SQLite path (default `minfin.db`)

## SimpleFIN token

1. Get a setup token from [SimpleFIN Bridge](https://beta-bridge.simplefin.org/)
   (or any SimpleFIN provider).
2. Paste it into the setup form when you first open the app.

`minfin` exchanges it for a long-lived access URL, stores that in the database,
and re-syncs every 6 hours.

## License

[MIT](LICENSE)
