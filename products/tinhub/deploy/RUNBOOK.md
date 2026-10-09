# tinhub runbook (phase 1)

One Linux server (arm64 or x86-64) runs every role; Postgres is on the same host and packs are on its disk
(design/tinhub.md §9). The commands below are for Debian or Ubuntu, run as root unless shown otherwise.

## Install

1. **Postgres 16 and Redis.** Redis keeps browser sessions and rate limits; without it tit still works, browsers
   cannot sign in, and limits are per process.

   ```sh
   apt-get install -y postgresql redis-server git
   sudo -u postgres createuser tinhub
   sudo -u postgres createdb -O tinhub tinhub
   ```

2. **The user, directories and secrets.** Every secret is a file only tinhub can read.

   ```sh
   useradd --system --home-dir /var/lib/tinhub --shell /usr/sbin/nologin tinhub
   install -d -o tinhub -g tinhub -m 0750 /var/lib/tinhub /var/backups/tinhub
   install -d -o root -g tinhub -m 0750 /etc/tinhub /etc/tinhub/secrets /etc/tinhub/tls
   for s in db nonce cookie; do head -c 32 /dev/urandom | base64 > /etc/tinhub/secrets/$s; done
   chgrp tinhub /etc/tinhub/secrets/*; chmod 0640 /etc/tinhub/secrets/*
   sudo -u postgres psql -c "ALTER ROLE tinhub PASSWORD '$(cat /etc/tinhub/secrets/db)'"
   ```

3. **The binaries.** Build with the Tin toolchain (any machine of the same architecture):

   ```sh
   bin/tinc -o tinhub products/tinhub/main.tin
   bin/tinc -o tit products/tit/main.tin
   install -m 0755 tinhub tit /usr/local/bin/
   install -d /usr/local/lib/tinhub
   install -m 0755 products/tinhub/deploy/lib.sh products/tinhub/deploy/backup.sh \
       products/tinhub/deploy/restore.sh products/tinhub/deploy/mirror-sync.sh /usr/local/lib/tinhub/
   ```

4. **Configuration.** Copy `tinhub.conf.example` to `/etc/tinhub/tinhub.conf` and set `public_url`; put the
   certificate and key at `/etc/tinhub/tls/fullchain.pem` and `privkey.pem` (readable by the tinhub group). tinhub reads
   them again when the files change, so a renewal needs no restart. Copy `backup.env.example` to
   `/etc/tinhub/backup.env`.

5. **Start.** The unit migrates the schema before each start.

   ```sh
   cp products/tinhub/deploy/tinhub*.service products/tinhub/deploy/tinhub*.timer /etc/systemd/system/
   systemctl daemon-reload
   systemctl enable --now tinhub tinhub-backup.timer
   curl -sf https://tinhub.example.org/readyz    # ready
   ```

6. **The first account.** `tinhub admin invite` prints a code; on the admin's machine, `tit key add` spends it and
   registers that machine's key.

   ```sh
   sudo -u tinhub tinhub admin invite you@example.org --site-admin
   tit key add https://tinhub.example.org CODE        # on your machine
   ```

   Repositories are made with the API as it grows; until then, as the tinhub user in `psql`:
   `INSERT INTO repos (owner_id, name, visibility, quota_bytes) SELECT owner_id, 'tin', 'public', 10737418240 FROM users WHERE email = 'you@example.org';`

The container route instead: put the three secrets in `products/tinhub/deploy/secrets/`, then
`docker compose -f products/tinhub/deploy/compose.yaml up -d` (Postgres beside tinhub, packs on a volume).

## The Tin repo as a mirror

GitHub stays the source of truth, with issues and CI, until the cutover. tinhub holds a read-only copy: only the
mirror account can push to it.

```sh
useradd --system --create-home --home-dir /var/lib/tinhub-mirror tinhub-mirror
sudo -u tinhub tinhub admin invite mirror@tinhub.example.org
sudo -iu tinhub-mirror sh -c '
  tit config set --user user.name tinhub-mirror
  tit config set --user user.email mirror@tinhub.example.org
  tit key add https://tinhub.example.org CODE
  git clone https://github.com/yasserreslan/tin.git tin && cd tin && tit adopt && \
  tit remote add tinhub https://tinhub.example.org/OWNER/tin'
```

Grant the mirror account write on the repository (`INSERT INTO grants (repo_id, user_id, role) ...`, `write`) and
nobody else, then `systemctl enable --now tinhub-mirror.timer`: every 5 minutes `mirror-sync.sh` fetches main,
converts what is new with `tit adopt` and pushes it. `journalctl -u tinhub-mirror` shows each run.

## Upgrade

```sh
install -m 0755 tinhub /usr/local/bin/tinhub
systemctl restart tinhub
```

The restart drains: requests in flight finish within `shutdown.deadline`, jobs go back to the queue, and a push cut
off is retried by tit. `ExecStartPre` runs `tinhub migrate` first; a binary older than the database refuses to start
(roll back by restoring, not by running an old binary on a new schema).

## Rotating secrets

- **db**: write the new password to `/etc/tinhub/secrets/db`, `ALTER ROLE tinhub PASSWORD '...'`, restart.
- **cookie**: replace the file and restart. Every browser session ends (people sign in again with `tit login`);
  nothing else is lost.
- **nonce**: replace the file and restart. Signed requests in flight fail once with a fresh challenge and tit retries.
- **TLS**: replace the files; no restart.

## Backup and restore

`tinhub-backup.timer` runs `backup.sh /var/backups/tinhub` nightly: a `pg_dump` into `db/` (the last 14 kept) and
every pack file that `packs/` lacks (packs never change, so each is copied once). Copy `/var/backups/tinhub` off the
server (rsync, or a bucket).

To restore, onto the same server or a fresh one installed as above:

```sh
sudo -u tinhub tinhub check > before.txt          # when the old server still runs, for comparison
systemctl stop tinhub
sudo -u tinhub sh -c 'set -a; . /etc/tinhub/backup.env; /usr/local/lib/tinhub/restore.sh /var/backups/tinhub'
sudo -u tinhub tinhub check                       # every live pack present; refs and packs fingerprints
systemctl start tinhub
```

`tinhub check` exits 1 and names any live pack missing from the store. Its `refs` and `packs` lines are fingerprints:
the same lines before the backup and after the restore mean the same refs and the same live packs.
`products/tinhub/tests/backup.sh` runs this drill in CI.
