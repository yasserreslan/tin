# orders: replay checks on Postgres, Redis and a real website

A development seed for tinhub's replay checks (design/tinhub.md §12.1), next to `dev/shop`. Every capsule here
records three kinds of external connection: Postgres queries, Redis commands and an HTTPS call to a public rates
website. Its fixes show every verdict a check gives.

## What the service does

`main.tin` answers `POST /orders/{id}/pay` by charging an order in US dollars:

1. **Postgres:** read the order (customer, amount in cents, currency, status).
2. **Redis:** the exchange rate to USD, cached as `fx:<currency>`.
3. **HTTPS:** on a cache miss, today's rate from `api.frankfurter.dev`, then cache it.
4. **Postgres:** insert the payment, then mark the order paid.
5. Answer with the amount and the card fee's share of it.

## The bugs

| Group | What happens | Why |
|---|---|---|
| `index out of range [1] with length 1` (orders 201, 202) | An order in ARS. The website answers `404 {"message":"not found"}`. | `rateFrom` takes what follows `"USD":`, and the answer has none. |
| 500, duplicate key (orders 301, 302) | Two pay clicks at once. | Both read the order as unpaid; the second payment insert breaks the primary key and is answered 500. |
| `integer divide by zero` (orders 401, 402) | A free order (0 cents). | The payment is written and the order marked paid, then the fee's share divides by 0: an error for a paid order. |
| (no failure group) | The rate is cached for 1 ms, not an hour. | `redis.SetEx` takes nanoseconds; the code passes 3600. Every capsule shows it: `SET fx:GBP 1.322 PX 1`. |

## The fix branches (`branches/fix-<name>.tin` is branch `fix/<name>`'s main.tin)

| Branch | Change | ARS | Double click | Free order |
|---|---|---|---|---|
| `main` | none | panicked | failing | panicked |
| `fix/unknown-currency` | a non-200 from the website is answered 422 | **passed** | failing | panicked |
| `fix/already-paid` | Postgres error 23505 on the payment insert is answered 409 | panicked | **passed** | panicked |
| `fix/free-order` | a 0-cent charge is answered without the fee share | panicked | failing | **passed** |
| `fix/all` | the three above | **passed** | **passed** | **passed** |
| `fix/fee-display` | the fee share in basis points; still divides by the charge | panicked | failing | panicked |
| `fix/upsert-payment` | `INSERT ... ON CONFLICT DO NOTHING` | panicked | diverged | diverged |
| `fix/lock-order` | `SELECT ... FOR UPDATE` (no transaction, so no lock either) | diverged | diverged | diverged |
| `fix/cache-ttl` | `SetEx(..., 3600 * 1000000000)`: right, but a different SET | panicked | diverged | diverged |

Passed means the fix made production's calls in production's order and answered below 500 without a panic. Diverged
means it sent the outside world something production did not (a different query, or a SET with other arguments), so
the recording cannot answer it. That says nothing about whether the fix is right: `fix/cache-ttl` is right.

These are the verdicts on macOS with `runner.sandbox = off`; they do not depend on the platform.

## Recording and checking

`web/dev/seed.sh` does all of this with `SEED_ORDERS=1` and `SEED_ORDERS_PG_ADDR`, `SEED_ORDERS_PG_USER`,
`SEED_ORDERS_PG_PASSWORD` and `SEED_ORDERS_PG_DATABASE` set (see tinhub's README). By hand:

```sh
# a database the script may empty, a Redis, the runner's public key (PUT /api/v1/orgs/<org>/runner answers it)
POSTGRES_ADDR=127.0.0.1:5432 POSTGRES_USER=... POSTGRES_PASSWORD=... POSTGRES_DATABASE=orders_demo \
REDIS_ADDR=127.0.0.1:6379 TIN_REPLAY_RECIPIENTS=<runner public key> TIN_REPLAY_SIGNING_KEY=<64 hex digits> \
  sh products/tinhub/dev/orders/record.sh bin/tinc /tmp/orders-spool
tit replay push /tmp/orders-spool --commit main      # from a clone of the repository holding main.tin
```

The repository's runner settings need the service's environment, though a replay never connects:
`{"entry": "main.tin", "env": ["POSTGRES_ADDR=...", "POSTGRES_USER=...", "POSTGRES_PASSWORD=...",
"POSTGRES_DATABASE=...", "REDIS_ADDR=..."]}`. Then check each branch against each group (`POST …/replay/checks`).

`dump.tin` prints what a capsule recorded, call by call, with a reader's key:

```sh
TIN_REPLAY_IDENTITY=reader.key TIN_REPLAY_SIGNERS=<signer hex> tin products/tinhub/dev/orders/dump.tin CAPSULE.tcap
```

```
  2. postgres@1  asked: Q SELECT customer, amount_cents, currency, status FROM orders WHERE id = I402
  3. redis@1  asked: 3:GET 6:fx:GBP                    got: $-1
  4. wire.http@1  asked: GET https://api.frankfurter.dev/v1/latest?base=GBP&symbols=USD
  5. redis@1  asked: 3:SET 6:fx:GBP 5:1.322 2:PX 1:1   got: +OK
  6. postgres@1  asked: E INSERT INTO payments (order_id, usd_cents, rate) VALUES ( I402 , I0 , S1.322 )
  7. postgres@1  asked: E UPDATE orders SET status = 'paid' WHERE id = I402
```
