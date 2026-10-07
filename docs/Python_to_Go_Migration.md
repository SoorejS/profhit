# Python maintenance tools migrated to Go

The PROPHIT backend was already Go. The four tracked Python files were historical
frontend refactoring scripts, not API handlers, news ingestion services or deployment
dependencies. They are now replaced by `backend/cmd/frontend-migrate` and the
standard-library-only `backend/internal/frontendmigration` package. The application's
HTML, CSS, JavaScript, authentication, database schema, wallet and settlement logic
are unchanged by this migration. No production deployment is needed for these tools.

| Removed Python script | Go operation | Purpose |
| --- | --- | --- |
| `refactor_js.py` | `refactor-js` | Split the legacy API client and generate the original component modules |
| `refactor_pages.py` | `refactor-pages` | Move legacy page scripts into modules and expose existing inline handlers |
| `update_html_css.py` | `update-html-css` | Update historical stylesheet references |
| `update_html_scripts.py` | `update-html-scripts` | Update historical script references and component imports |

## Use

Run from `backend`, with the project directory explicitly selected:

```text
go run ./cmd/frontend-migrate -root .. -operation update-html-css
go run ./cmd/frontend-migrate -root .. -operation update-html-scripts
```

The default is a read-only preview. Review its paths and inspect the selected tree
before running the same command with `-apply`. Each invocation prepares its plan
again; a previous preview is not an approval token. Do not edit files concurrently.

The two `refactor-*` operations exist for old, unmigrated project copies only.
They refuse to overwrite existing destination modules. `refactor-js` requires the
original legacy sources and export signature; its embedded JavaScript templates
reproduce the original Python script, not today's application components. Never use
it to regenerate the current application or deploy those historical templates.

## Safeguards and recovery

- Plans validate all inputs before writing. Paths must stay within the selected
  project, symlinks are rejected, and source files must contain valid UTF-8.
- Apply uses an exclusive migration lock, verifies file snapshots and writes
  destinations before removing legacy sources.
- Every changed original is backed up under
  `.frontend-migrate-backups/migration-*/`, retaining its relative path.
  `manifest.json` records every change, whether the file existed, and its file mode.
- Writes use a same-directory temporary file and rename. Detected apply errors
  trigger rollback of completed changes; failures include the retained backup path.
- A process crash or filesystem failure can still require manual recovery. Restore
  files marked `existed: true` from that backup, using the recorded permissions,
  and remove only newly created files marked `existed: false`. Inspect current
  changes before restoring. If a lock remains after a crash, verify that no migration
  is active before removing it. Backups and locks are ignored by Git.
- Existing files retain their CRLF/LF newline convention. Re-running the HTML
  operations on their migrated results makes no further changes.

Two intentional fixes accompany the port: admin module imports point to `../api`
and `../components`, and a missing topbar import is repaired even when the sidebar
import already exists.

## Verification

Before deleting the Python scripts, their outputs were captured on isolated,
disposable legacy fixture trees. Expected results and embedded template strings are
stored as JSON to preserve original whitespace without introducing trailing spaces
in the repository. The Go tests compare complete resulting file
inventories and contents against those static snapshots, accounting explicitly for
the corrected admin imports. Python is not needed to run any migration or test.
The tests also verify preview-only behavior, backups, stale-source rejection,
overwrite protection, invalid inputs, migration locking, rollback after an injected
write failure, path confinement, symlink rejection and newline preservation.

Windows may lack permission to create test symlinks; that case is skipped locally.
Linux CI requires the symlink test to execute, runs the full Go suite with the race
detector and disposable PostgreSQL, and checks the production container and frontend.
CI also rejects any new tracked Python source or notebook.

Read-only previews against the current application found zero script/page migration
changes and one optional stylesheet-reference update in `rules.html`. That update
was not applied: current application assets remain byte-for-byte unchanged.

Ignored historical `.tmp` and `audit-artifacts` files can include old Python audit
scratch scripts. They are not versioned application code, build inputs or deployment
dependencies and were not rewritten or deleted. Python-related news titles and
source domains are content, not executable Python, and remain intact.

Local validation on 2026-10-07:

| Check | Result |
| --- | --- |
| `go fmt ./...`, `go vet ./...`, `go build ./...`, `go test ./...` | Passed |
| Migration package and CLI tests with `-count=1 -cover` | Passed; 80.2% and 80.6% statement coverage; Windows symlink case skipped |
| `go mod verify` | Passed: all modules verified |
| `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` | Passed; zero reachable vulnerabilities; one advisory in a required module whose affected code is not called |
| `node scripts/check-frontend.mjs` | Passed: 27 pages and 27 script syntax checks, plus links, structure and XSS regressions |
| `git diff --check` | Passed |
| Existing application assets and deployment/module configuration | No changes |

Remote Linux verification must also pass before the migration is considered fully
verified.
