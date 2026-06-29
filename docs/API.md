# minfin REST API

JSON API

Base path: `/api`. Default server `http://localhost:8080` (`PORT` env var).

## Authentication

Stateless **HS256 JWT**, 7-day expiry returned in the body.

1. `POST /api/signup` or `POST /api/login` → `{"token": "<jwt>"}`.
2. Send it on every other request:
   `Authorization: Bearer <jwt>`.
3. On `401`, the token is missing/expired, log in again. There is no refresh
   token and no server-side logout; clients discard the token to "log out".

`/api/login` and `/api/signup` are the only unauthenticated endpoints. Any other
`/api/*` request without a valid token returns `401 {"error":"unauthorized"}`.

## Onboarding (SimpleFIN)

A new user has no connected accounts until they link a SimpleFIN token:

1. User obtains a base64 **setup token** from <https://bridge.simplefin.org>.
2. `POST /api/setup {"token":"<setup-token>"}` claims it, creates the
   portfolio, and runs an initial sync. → `204`.
3. `GET /api/me` then reports `"connected": true`.

Until connected, read endpoints return empty data (`{"accounts":[]}`) with
`200`, so clients can detect the "needs onboarding" state via `GET /api/me`.

## Conventions

- Request bodies are JSON (`Content-Type: application/json`).
- Money is in **dollars** (floats). Dates are `YYYY-MM-DD`.
- Mutations that return no data respond `204 No Content`.
- Errors: `{"error":"<message>"}` with an appropriate status
  (`400` bad input, `401` unauthenticated, `409` email taken, `5xx` server).
- Object field names in responses are PascalCase (`Balance`,
  `Payee`); the chart/series objects use lowercase (`labels`, `lines`).

## Endpoints

### Auth

| Method & path | Body | Response |
|---|---|---|
| `POST /api/signup` | `{"email","password"}` (password ≥ 8 chars) | `200 {"token"}`, `409` if email taken |
| `POST /api/login` | `{"email","password"}` | `200 {"token"}`, `401` if invalid |

### Session / onboarding

| Method & path | Body | Response |
|---|---|---|
| `GET /api/me` | — | `{"email","connected","lastSync","notices"}` |
| `POST /api/setup` | `{"token"}` | `204`; `502` if SimpleFIN claim fails |
| `POST /api/sync` | — | `204` (re-syncs the active portfolio) |

### Accounts

| Method & path | Body | Response |
|---|---|---|
| `GET /api/accounts` | — | `{"accounts":[…],"types":[…],"assets","liabilities","netWorth"}` |
| `POST /api/accounts/type` | `{"id","type"}` | `204`; `type` ∈ the `types` keys or `""` |
| `POST /api/accounts/nickname` | `{"id","nickname"}` | `204` (empty clears) |
| `POST /api/accounts/asset-value` | `{"id","value"}` | `204`; `value` dollars ≥ 0, 0 clears |

Account object: `ID, Org, Name, Nickname, Currency, Type, Liability, HasAsset,
Balance, AssetValue, TxnCount, LastTxn, Spent30`.

### Transactions

`GET /api/transactions` — query params (all optional):

| Param | Meaning | Default |
|---|---|---|
| `from`, `to` | `YYYY-MM-DD` window (inclusive) | last 30 days |
| `account` | account id | all |
| `category` | exact name, `none` for uncategorized | all |
| `dir` | `all` \| `debit` \| `credit` | `all` |
| `q` | substring match on payee/description | — |
| `page` | 1-based, 100 rows/page | 1 |

Response: `{"rows":[…],"page","hasNext","from","to"}`. Row object: `ID, Posted,
Account, Payee, Description, Category, Amount (signed), Pending, Remembered`.

| Method & path | Body | Response |
|---|---|---|
| `POST /api/transactions/category` | `{"id","category","remember"?,"pattern"?,"payee"?}` | `204` |

`category=""` clears it. When `remember` is true, a payee→category rule is saved
using `pattern` (falling back to `payee`).

### Categories & rules

`GET /api/categories?from&to` →
`{"spend":[…],"income":[…],"categories":[…],"rules":[…],"spendPie":{…},"incomePie":{…},"from","to"}`.
`spend`/`income` are `CategoryStat` (`Category, Color, Count, Amount`);
`spendPie`/`incomePie` are chart-ready `{labels, values, colors}`.

| Method & path | Body / params | Response |
|---|---|---|
| `POST /api/categories` | `{"name"}` | `204` |
| `DELETE /api/categories?name=<name>` | — | `204` (clears it off txns + rules) |
| `POST /api/categories/exclude` | `{"name","exclude"}` | `204` (exclude from totals) |
| `POST /api/categories/recategorize` | — | `200 {"updated":<n>}` (re-applies all rules) |
| `POST /api/categories/rules` | `{"pattern","category"}` | `204` |
| `DELETE /api/categories/rules/{id}` | — | `204` |

### Spending

`GET /api/spending` — query params:

| Param | Meaning | Default |
|---|---|---|
| `range` | preset key (`last-30-days`, `this-month`, `last-12-months`, …) | `last-30-days` |
| `interval` | `daily` \| `weekly` \| `monthly` | `daily` |
| `split` | `1` for one line per account | off |

Response: `{"series":{labels,ranges,lines},"payees":[…],"range","rangeLabel","interval","split"}`.
`payees` are `PayeeStat` (`Payee, Count, Spent`).

## Example

```sh
TOKEN=$(curl -s localhost:8080/api/signup \
  -d '{"email":"a@b.c","password":"password1"}' | jq -r .token)

curl -s localhost:8080/api/me            -H "Authorization: Bearer $TOKEN"
curl -s localhost:8080/api/setup         -H "Authorization: Bearer $TOKEN" \
  -d '{"token":"<simplefin-setup-token>"}'
curl -s localhost:8080/api/accounts      -H "Authorization: Bearer $TOKEN"
curl -s "localhost:8080/api/transactions?dir=debit&q=coffee" \
  -H "Authorization: Bearer $TOKEN"
```

## Not implemented (TODO)

- Rate limiting on login/signup.
- Token revocation / logout — JWTs expire on their own; clients drop them.
- Multi-portfolio switching — the first portfolio a user belongs to is active.
