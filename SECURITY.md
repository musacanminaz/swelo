# Security

swelo deletes files, so safety bugs are security bugs. Please report anything
that makes swelo delete, trash, exclude from backups, or run something it
should not — including paths outside the folders it says it scans, git-tracked
files, files in macOS-protected folders, or commands run from untrusted
repository / app metadata.

**Report privately:** open a [GitHub security advisory](https://github.com/musacanminaz/swelo/security/advisories/new)
. Please include the macOS version, the
swelo version (`swelo --version`) and the steps or folder layout that trigger it.
You will get a reply within a week.

## What swelo does to stay safe

- Every deletion goes through one gate (`internal/safety`): allowed roots only,
  symlinked parents resolved, protected paths, `.git`, `.env*`, database dumps,
  git-tracked files and recently changed or busy projects are refused.
- Nothing is deleted without `--apply` (or a confirmation in the interactive
  views); sudo steps show the exact command and are confirmed one by one, and
  `--yes` never approves them.
- Untrusted input is treated as such: file names are escaped before printing,
  app bundle ids are validated before they become paths, git runs with
  repository hooks and fsmonitor disabled, and `git gc` is skipped for
  repositories whose config sets keys outside a short allowlist.
- swelo makes no network connections.
