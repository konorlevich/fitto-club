# Deploy: Fitto Club on Railway

One Go binary with everything embedded, plus one volume for the content
database. No Dockerfile: Railway builds `go build` from `go.mod`.

## First deploy

1. Create a Railway service from this repository.
2. Add a **volume** mounted at `/data`. Set `DATA_DIR=/data`.
   Without it the database lives on ephemeral disk and every deploy starts
   from the seed again.
3. Set the variables from `.env.example`. For production:
   `ENV=production`, `BASE_URL=https://fitto.club`, `GTM_ID` when the club
   has a container.
4. Health check path: `/healthz`.
5. Custom domain: `fitto.club` (apex, canonical) and `www.fitto.club`
   (answers with a 301 to the apex in production).

The first boot creates `content.db` and fills it from the shipped seed
(`internal/content/seed*.go`). Later boots never re-seed.

## The launch gate

With `ENV=production` the service will not start while
`content.Pending` (in `internal/content/seed.go`) lists facts the club has
not confirmed: the monthly price, closing time, coach photo consent, class
durations, the legal entity. Confirm them, update the seed or the database,
empty the list. For a staging deploy before that, set `ALLOW_PENDING=1`.

## Before launch: keep it out of search

While the site lives on the temporary `*.up.railway.app` domain with
unconfirmed facts, set `NOINDEX=1`. The service then disallows everything in
`robots.txt`, marks every page `noindex,nofollow` and sends `X-Robots-Tag`.
Remove it (together with `ALLOW_PENDING`) when the site launches on
`fitto.club`.

## Backups

The service writes `DATA_DIR/backups/content-YYYYMMDD.db` once a day
(`VACUUM INTO`, consistent while serving) and keeps 14. They sit on the same
volume, so also copy them off it on a schedule (Railway volume backups, or
`railway run` + `scp`).

### Restore (rehearsed in `TestBackupRestoreRehearsal`)

1. Stop the service (or scale to 0).
2. Copy the chosen backup over `DATA_DIR/content.db` and delete
   `content.db-wal` and `content.db-shm` next to it.
3. Start the service. It migrates if needed and serves the restored content.

## Assets

Everything under `web/static` is built from `materials/` by the scripts in
`tools/assets/` and committed; the deploy never runs them:

    python3 -m venv .venv && .venv/bin/pip install fonttools brotli pillow
    .venv/bin/python tools/assets/build_fonts.py    # woff2 subsets
    .venv/bin/python tools/assets/build_images.py   # photos, masks, video (needs ffmpeg)
    .venv/bin/python tools/assets/build_map.py      # static map from OSM tiles
    python3 tools/assets/gen_reviews.py && gofmt -w internal/content/seed_reviews.go

OG images and icons are rendered from `tools/assets/og.html` in Chrome at
1200x630 (`?lang=en|ru|ka`) and 512x512 (`?icon=1`).

CSS and JS are minified at boot and inlined into every page, so what ships is
always minified and there is no build step to forget.

## Checks before calling it done

    gofmt -l . && go vet ./... && go test ./...

## Admin (`/admin`)

The club edits prices, hours, coaches, classes, massage and reviews at
`/admin` (design: `.design/fitto-admin/`). It is mounted only when
`ADMIN_OWNER_LOGIN` is set.

**First login.** Set `ADMIN_OWNER_LOGIN` and `ADMIN_OWNER_PASSWORD` on the
service, open `/admin/login`, log in with them. The admin immediately asks
for a real password and a name; from then on the ENV pair is ignored. Remove
`ADMIN_OWNER_PASSWORD` from the variables afterwards if you like - it is no
longer consulted while the owner has a password in the database.

**Lost owner password.** Run the binary once with `-reset-owner` against the
same `DATA_DIR` (on Railway: a one-off command on the service, or a shell in
the container). It clears the owner's password and ends their sessions; the
ENV pair works again for exactly one login, which forces a new password.
Editors reset their own password through the owner (Users → Reset).

**Uploads.** Coach photos land in `DATA_DIR/uploads/` as `<coach>-<hash>-orig.jpg`
(private, upright, metadata-free) plus `...-<focus>-400.webp` and `-800.webp`
(public, immutable). They are part of the volume and of the backup: copy
`uploads/` together with `content.db`. The restore rehearsal covers both.

**Security notes.** Sessions are random tokens stored hashed with a 30-day
sliding expiry; the cookie is `HttpOnly`, `SameSite=Lax`, `Secure` in
production. Every mutating form carries a per-session CSRF token. Login is
rate-limited: five failures per login or IP lock it for 15 minutes with the
same error text for unknown logins and wrong passwords. Uploads are sniffed,
capped at 12 MB, re-encoded, and never served under a user-supplied name.
`/admin` answers with `Cache-Control: no-store`, `X-Robots-Tag: noindex`, a
strict CSP (`script-src 'nonce-…'`) and is disallowed in `robots.txt`.
