# cartographer-agent

Cartographer-Agent is a lightweight system inventory tool written in Go. It runs on your servers to collect various system information and reports them to a centralized server via REST api. Cartographer-Agent helps you maintain a real-time map of your infrastructure.

This project has been around for on my private repo for a few years, in use by a few people. The server isn't open source (yet), but this will post to any REST api so it'd be easy to use the data in your systems. 

> NOTE: This repository is a fresh snapshot of the project, created to keep the original private repository’s history separate. (I didn't feel like cleaning up the history on the old repo...)

## Development

This project uses [stratt](https://stratt.sh) as its task runner:

- `stratt all` — full verification suite (sync, format, lint, test)
- `stratt test` / `stratt lint` / `stratt format` — individual steps
- `stratt run dev` — build a local dev binary at `./bin/cartographer-agent`
- `stratt run snapshot` — cross-compile all release platforms via GoReleaser (no publish)
- `stratt release patch|minor|major` — verify, bump version, tag, and push; GitHub Actions runs GoReleaser to build and publish the release artifacts from the tag

## License
This project is licensed under the Apache License 2.0. See the LICENSE file for details.