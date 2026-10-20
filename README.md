# swelo

Disk care for developer Macs. / Developer Mac'leri için disk bakımı.

> **Status: early development.** Only `swelo` (intro) and `swelo permissions` work today.
> See [PROGRESS.md](PROGRESS.md).

swelo finds files that pile up during development and can be regenerated — `node_modules`,
build folders, package manager caches, Docker leftovers, old simulators and SDKs — shows how
much space they take, and deletes only what you approve.

## What swelo never does

- Touch your source code, git-tracked files or documents
- Delete Docker volumes, databases or browser profiles
- Touch projects you changed in the last 7 days
- Run `sudo` without asking (the exact command is always shown first), or send data over the network
- Delete anything without `--apply` and your confirmation

Run `swelo permissions` to see every permission swelo may use and why.

## Commands

| Command | | Status |
|---|---|---|
| `swelo` | What swelo does, permission status, command list | ✅ |
| `swelo permissions` | Every permission (Full Disk Access, sudo) explained | ✅ |
| `swelo clean` | Clean regenerable files; dry run unless `--apply` | planned |
| `swelo analyze` | Where disk space went; deletes nothing | planned |
| `swelo uninstall` | Remove apps and developer toolchains via their own uninstallers | planned |
| `swelo optimize` | Spotlight / Time Machine exclusions, Docker disk, git gc | planned |
| `swelo status` | Disk status and history | planned |

Output language follows your system (Turkish or English); override with `--lang tr|en`.

## Install

Not released yet. From source:

```sh
go install github.com/musacanminaz/swelo/cmd/swelo@latest
```

## License

MIT
