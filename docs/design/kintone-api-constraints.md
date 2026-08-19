# kintone API constraints

The properties of the kintone REST API that this provider is built around. It exists so that a clean
rewrite does not have to rediscover them, and so that a reviewer can tell a measured fact from a
documented one before relying on either.

This note is for implementers. `README.md` states the operator-facing consequences — password
authentication, no app deletion, the deploy wait — and `AGENTS.md` states the rules that follow from this
note. Neither repeats the reasoning below.

## How to read the provenance markers

Every claim carries one marker. They are not interchangeable.

| Marker | Meaning |
| --- | --- |
| `[measured: dev 2026-08-19]` | Observed against this project's development environment on that date, using read-only `GET` requests only. Appendix A lists the requests and their responses. |
| `[measured: poc]` | Measured against a live kintone environment during the proof-of-concept that preceded this repository, and reported from it. Where the measurement date is known it is given in the text. The proof-of-concept is not part of this repository, so these facts cannot be re-read here — only re-measured. |
| `[documented: Sn]` | Stated in published Cybozu documentation. `Sn` indexes the source list in Appendix B. Every cited page was fetched and returned HTTP 200 on 2026-08-19. |
| `[undocumented]` | The documentation was searched for this and does not state it. An absence claim, not a silence. |
| `[inferred]` | A conclusion drawn in this repository from the claims around it. Not an API guarantee. |

A claim marked `[documented]` and nothing else has not been verified against a running kintone. A claim
marked `[measured]` and nothing else was observed once, in one environment, and may be a property of that
environment rather than of kintone.

The knowledge in this note originally came from a proof-of-concept that is not part of this repository. Every
claim that could be re-established here was re-established here, against published documentation, against the
development environment, or both. The facts that neither route could reach — the ones that require writing to
a real app — are carried as `[measured: poc]`, reported from that repository rather than observed here. What
no route settled is in section 7 as an open question.

## 1. Preview and live are two environments

App settings exist twice: as *pre-live* settings that have been written but not yet applied, and as the
*live* app that users see. Endpoints under `/k/v1/preview/` address the pre-live copy; the same path
without `preview` addresses the live app. This is a documented, API-wide convention rather than a property
of any one resource — "The URL path in the case of settings that have not yet been updated to the live App
is `/k/v1/preview/xxxx.json`" `[documented: S1]`.

The split is visible in the two read endpoints for general settings: `GET /k/v1/app/settings.json` returns
the live app's settings, `GET /k/v1/preview/app/settings.json` returns the pre-live settings that have not
yet been deployed `[documented: S5]`. They also require different permissions — record view or add
permission for the live read, app-management permission for the pre-live read `[documented: S5]`.

## 2. Two-phase writes

### The write path

Every app mutation this provider performs writes to the pre-live environment first and takes effect only
after a separate deploy call.

- `POST /k/v1/preview/app.json` creates an app. It accepts only three parameters — `name`, required and at
  most 64 characters, plus the optional `space` and `thread` — and returns `app` and `revision` as strings.
  The documentation directs the caller to the deploy API afterwards `[documented: S2]`.
- `PUT /k/v1/preview/app/settings.json` updates general settings, with the same instruction to deploy
  afterwards `[documented: S3]`.

- `POST /k/v1/preview/app/deploy.json` deploys. It is documented as producing the same result as clicking
  *Update App* or *Discard Changes* in the app's settings screen, and it returns no response body
  `[documented: S4]`.

Everything else a `kintone_app` resource carries — description, theme, title field, number precision, first
month of the fiscal year, the record feature toggles — exists on the settings PUT and not on the create
call `[documented: S2, S3]`. Creating an app as Terraform describes it is therefore three calls before the
deploy completes: create, then settings PUT, then one deploy that carries both writes. Splitting that into
two deploys would double the wait and break the rule that one change produces one deployment `[inferred]`.

Deploy parameters worth stating exactly `[documented: S4]`:

- `apps` — required array, at most 300 entries. Each entry carries `apps[].app`.
- `apps[].revision` — optional optimistic-concurrency check. The request fails unless the value is the
  latest revision, and **the check is skipped when the parameter is omitted or set to `-1`**.
- `revert` — optional, and top-level rather than per-app. `true` discards all pre-live changes; omitting it
  means `false`.

A multi-app deploy is all-or-nothing: if one app fails, every app named in the request is rolled back to
its state before the call `[documented: S4]`.

### Revisions

A revision is one counter per app, shared by every settings API rather than one counter per settings area
`[measured: poc 2026-07-28]`. A pre-live write advances the preview counter only: the live counter does not
move until the deploy, and the deploy makes live catch up to preview rather than advancing the counter again
`[measured: poc 2026-07-28]`. A pre-live write carrying a stale revision is rejected with **HTTP 409 and the
error code `GAIA_CO03`** — not `GAIA_CO02`, the code that circulates in community material
`[measured: poc 2026-07-28]`.

The `revision` parameter of the settings PUT is therefore compared against the preview counter. Whether
`apps[].revision` on the deploy call uses the same counter is open question 2; the proof-of-concept never
sent that parameter, so nothing measured covers it.

Detect a conflict by the 409 status or by the `GAIA_CO` prefix, never by an exact code: an implementation
that matched `GAIA_CO02` would not have matched what was actually observed `[inferred]`.

### The poll

The deploy is asynchronous. The Japanese page says so in as many words and directs the caller to the
deploy-status API `[documented: S6]`; the English page corroborates it indirectly by documenting no
response body for the deploy call `[documented: S4]`. Completion is observed through
`GET /k/v1/preview/app/deploy.json`, which reports a status per app for up to 300 apps `[documented: S7]`.

The status set has **four** values, not three `[documented: S7]`:

| Status | Meaning |
| --- | --- |
| `PROCESSING` | The app settings are being deployed. |
| `SUCCESS` | The app settings have been deployed. |
| `FAIL` | An error occurred and the deployment failed. |
| `CANCEL` | The deployment was cancelled because the deployment of other app settings failed. |

`CANCEL` is documented only as a consequence of a sibling app failing inside a multi-app deploy. The
documentation does not state that a single-app deploy can never return it, so the poll loop treats it as a
terminal, non-successful status regardless of how many apps were deployed `[inferred]`.

Two things the documentation does not give: a polling cadence and a timeout, and any statement about what
status an app reports when no deploy is in flight `[undocumented]`. On the development environment, polling
an app whose last deploy finished long before the request returned `SUCCESS`
`[measured: dev 2026-08-19]`. A poll loop must therefore not assume that the first response after a deploy
call is `PROCESSING`, and must not treat `SUCCESS` as proof that its own deploy has completed — the
revision it deployed is the only thing that identifies its own write `[inferred]`.

One case where the status does not go stale was measured: a status read issued immediately after a
`revert: true` deploy reported the revert job's `PROCESSING`, not the terminal status of the deploy before it
— observed roughly 0.8 seconds after the call `[measured: poc 2026-07-31]`. That closes the window in which a
poll could mistake the previous job's outcome for its own, for the revert path at least.

`CANCEL` was never observed in the proof-of-concept, and whether a single-app deploy can produce it is still
unknown `[measured: poc]`.

### Why drift-detection reads go against the live environment

Terraform's `Read` answers "what is actually in effect", and the pre-live copy is by definition not in
effect. Reading it for drift detection would report changes that a person made in the kintone UI and has
not deployed as though they were the real state, and would hide the case where a deploy failed after a
successful pre-live write. The live app is the source of truth for drift; the pre-live copy is a staging
area the provider writes to and then drains `[inferred]`.

Two documented facts reinforce the choice: `GET /k/v1/apps.json` and `GET /k/v1/app.json` return published
apps only `[documented: S8, S16]`, and the live read needs only record permissions while the pre-live read
needs app-management permission `[documented: S5]` — a read path that requires fewer permissions is the
better default for a data source.

A consequence for the client's API shape: preview reads and live reads are different operations, not one
operation with a boolean flag, because the wrong default here is silent `[inferred]`.

The list read is paginated and capped. `GET /k/v1/apps.json` takes `limit`, which must be between 1 and 100
and defaults to 100, and `offset`, which may go up to 2,147,483,647 `[documented: S8]`; asking for 101
returned HTTP 400 with `{"code":"CB_VA01"}` `[measured: dev 2026-08-19]`. No total count is returned, so a
data source that lists apps must page with `offset` until it receives a page shorter than `limit`
`[inferred]`. Getting this wrong fails silently: on a development subdomain with fewer than 100 apps, a
single-page implementation looks correct.

## 3. There is no app deletion API

The kintone REST API cannot delete an app — neither a deployed one nor one that was created in the pre-live
environment and never deployed. That conclusion is established by the evidence below rather than quoted
from any one page `[inferred]`.

The strongest evidence is the API's own catalogue. `GET /k/v1/apis.json` on the development environment
enumerated 103 entries, and the only ones ending in `delete` were `guests/delete`, `plugin/delete`,
`preview/app/form/fields/delete`, `record/comment/delete`, `records/cursor/delete`, `records/delete` and
`space/delete`. No app-level delete entry exists at either `app/*` or `preview/app/*`
`[measured: dev 2026-08-19]`. The published App API index agrees: it enumerates the app-level endpoints and
contains no delete-app page, and the only lifecycle-adjacent entries are Add Preview App, Move App to Space,
Deploy App Settings and Get App Deploy Status `[documented: S9]`. No Cybozu page states the absence in
words — it is established by enumeration, not by a quotable sentence `[undocumented]`.

Deleting an app is a UI operation performed by an app administrator, and a deleted app can be restored by a
user and system administrator within 14 days `[documented: S10]`.

`revert: true` on the deploy endpoint is not a deletion. It discards the pre-live changes and resets them to
the live app's current settings — the *Discard Changes* button `[documented: S4]`. What it does to an app
that has never been deployed, and therefore has no live settings to reset to, is not documented
`[undocumented]`, but it was measured: the revert is a harmless no-op that reports `SUCCESS`, and the
never-deployed app stays where it is `[measured: poc 2026-07-29]`. Reverting is not an escape hatch for an
interrupted create.

Note what else a revert takes with it. It cancels *all* pre-live changes on the app `[documented: S4]`, so it
also discards a draft a person left unsaved in the kintone UI and any pre-live change another process has not
deployed yet. A provider that reverts to clean up after its own failed deploy destroys work it never wrote
`[inferred]`.

An app created by `POST /k/v1/preview/app.json` is assigned a real app ID at creation time, before any
deploy `[documented: S2]`. Both list and single reads return published apps only — "Note that only
published Apps can be retrieved" appears on the Get App page as well as the Get Apps page
`[documented: S8, S16]`. An abandoned create therefore leaves behind an app that no API can see or remove
`[inferred]`. Whether the app ID it consumed stays occupied is open question 6.

What follows for the provider `[inferred]`:

- `Delete` removes the resource from Terraform state and warns that the app must be deleted by hand. It
  never attempts physical deletion, and no future version adds one.
- No attribute may force replacement. Terraform implements a replacement as destroy-then-create, and a
  destroy that cannot destroy leaves the old app behind for every apply that touches such an attribute. An
  attribute that the API cannot change after creation — `space` and `thread`, which exist on the create
  call and not on the settings PUT — must fail the plan with a diagnostic naming the attribute, not carry a
  `RequiresReplace` plan modifier `[inferred]`.
- Acceptance tests create real apps that survive the test run. Every acceptance-test app name is prefixed
  `tfacc-`, and the created names are reported so a person can clean them up.
- A failed or interrupted create is not free. This is the load-bearing reason for the retry policy in
  section 5.

## 4. Authentication

### The two methods this provider supports

Password authentication sends `X-Cybozu-Authorization`, whose value is the Base64 encoding of
`login:password` `[documented: S11]`. On the development environment a request with a deliberately wrong
password returned HTTP 401 with `{"code":"CB_WA01", ...}` `[measured: dev 2026-08-19]`.

API-token authentication sends `X-Cybozu-API-Token`. Tokens are issued per app rather than per domain, and
"API Tokens generated from an App can only be used for that App" `[documented: S11, S17]`. Several tokens
may be sent comma-separated; the limit of nine per request is stated on the Japanese authentication page
only `[documented: S12]`. Two operational properties matter for diagnostics. Operations performed with a
token are attributed to the user *Administrator* rather than to a service account `[documented: S11]`. And a
newly generated token is inert until the app is deployed — "Generated API Tokens cannot be used until the
**Update App** button has been clicked" — which is stated in the API-token tutorial rather than in the
authentication reference `[documented: S17]`. Presenting a token that does not belong to the target app
returned HTTP 400 with `{"code":"GAIA_IA02", ...}` `[measured: dev 2026-08-19]`.

When both headers are present, password authentication wins; the documented precedence is password, then
API token, then OAuth client, then session `[documented: S11]`.

Session authentication is a browser mechanism: it depends on a kintone session cookie, and writes need a
CSRF token issued to JavaScript running inside kintone `[documented: S11]`. It is therefore not available to
an out-of-process client such as a Terraform provider `[inferred]`. OAuth 2.0 appears in the authentication
list of every endpoint checked for this note `[documented: S2, S8, S16]`, but it is out of scope for
v0.1.0.

### Which operations exclude API tokens

Two endpoints inside the v0.1.0 surface reject API tokens:

- `POST /k/v1/preview/app.json` (Add Preview App) — "API Tokens cannot be used with this API"; its
  authentication list is password, session and OAuth 2.0 only `[documented: S2]`.
- `GET /k/v1/apps.json` (Get Apps) — its authentication list is password, session and OAuth 2.0, and its
  permissions note repeats that API tokens cannot be used `[documented: S8]`.

`PUT /k/v1/preview/app/settings.json`, `POST /k/v1/preview/app/deploy.json`,
`GET /k/v1/preview/app/deploy.json` and `GET /k/v1/app.json` do accept API tokens `[documented: S3, S4, S7,
S16]`.

The exclusion on app creation follows from what a token is. A token is generated inside an existing app and
works only for that app, so a request that creates an app has no app for its token to have come from
`[inferred]`.

So an API-token-only provider configuration can serve neither app creation nor the app-list data source
`[inferred]`. A diagnostic that says "this operation requires password authentication" must name the
operation, because the set is larger than app creation alone.

### Two-factor authentication

A user with two-factor authentication enabled cannot execute the REST API with password authentication. The
documentation's own remedy is to use another authentication method, or to prepare a dedicated integration
user with two-factor authentication disabled `[documented: S12]`. This is stated on the Japanese
authentication page; the English page does not mention two-factor authentication at all `[undocumented]`.

Because app creation requires password authentication and password authentication requires a 2FA-free
account, this provider assumes a dedicated service account without 2FA, holding only the app-administration
permissions it needs `[inferred]`.

## 5. Concurrency and retries

### What kintone documents

- A concurrent-request limit of **100 per domain** — not per app and not per user. Every response carries
  `X-ConcurrencyLimit-Limit` and `X-ConcurrencyLimit-Running` so a client can observe its own headroom
  `[documented: S13]`. Both headers were present on a live response, with values `100` and `1`
  `[measured: dev 2026-08-19]`.
- Exceeding that limit returns HTTP **429**, and requests continue to be rejected until concurrency drops
  back below the limit `[documented: S14]`.
- A per-day quota of 10,000 requests per app, reset at 09:00 JST. Get Apps and Add Preview App are among the
  APIs excluded from that count `[documented: S13, S14]`.
- Settings-changing APIs must not run while a deploy or a cancellation is in progress: "If APIs to change
  the App's settings are run while deploying (or cancelling), an error will be returned" `[documented: S4]`.
  The Japanese page scopes the window precisely — it lasts until the asynchronous deploy finishes, not just
  until the HTTP call returns `[documented: S6]`.
- On a revision mismatch the request fails and the settings are left unchanged `[documented: S6]`.

None of the limits in this subsection has ever been observed. Neither the development environment nor the
proof-of-concept has produced a 429 or hit the daily quota, and the proof-of-concept's client never read the
concurrency headers at all. Treat the numbers as documentation, not as experience.

### What kintone does not document

- No error code accompanies the 429; only the HTTP status is published `[undocumented]`.
- No `Retry-After` header, no wait interval, no retry count and no backoff algorithm are prescribed. The
  only official guidance is preventative — observe the concurrency headers and shape traffic
  `[documented: S15]`.
- No error code or message is published for the deploy-in-progress conflict or for a revision mismatch
  `[undocumented]`. The revision mismatch has since been measured — HTTP 409 with `GAIA_CO03`, see section 2
  `[measured: poc 2026-07-28]` — but the deploy-in-progress conflict has not been produced deliberately by
  anyone here, so its code remains unknown.
- There is no published REST API error-code catalogue at all. The documented contract is only the response
  shape `{"code", "id", "message"}` `[documented: S13]`. Codes observed on the development environment
  include `CB_VA01` for a parameter-validation failure, `GAIA_AP01` with HTTP 404 for an unknown app ID,
  `CB_WA01` with HTTP 401 for a failed password authentication and `GAIA_IA02` with HTTP 400 for a token
  that does not belong to the app `[measured: dev 2026-08-19]`. Treat these as observations, not as a
  contract: matching on them is a fallback, never the primary control flow.

### What the client does, and why

**A per-app mutex serialises operations on the same app.** The conflict it avoids is documented, but the
error it would produce is not — with no published code, a client cannot reliably distinguish "a deploy is in
progress" from an unrelated 4xx and cannot recover from it after the fact. Serialising client-side is the
only documentation-supported way to keep out of the situation `[inferred]`.

The mutex is held across the whole write, not per HTTP call: acquired before the first pre-live write and
released only when the poll reaches a terminal status or times out. A per-call mutex would satisfy the
phrase "serialise operations" and still violate the constraint, because the forbidden window documented on
S6 runs until the asynchronous deploy finishes `[inferred]`. On a poll timeout the mutex is released and the
failure surfaced — a stuck deploy must not deadlock every later operation on that app.

The proof-of-concept held it wider still: through the live read-back that follows the deploy, so that another
resource's deploy could not land between the poll and the read whose values go into state. Carry that over —
the read-back is the write's last step, not a separate operation `[inferred]`. If a failed deploy is followed
by a revert, the revert happens inside the same held lock and inside the same deploy timeout, five minutes in
the proof-of-concept `[measured: poc]`.

The mutex is process-local, and that is a real limit rather than an implementation detail: two Terraform
processes against the same app are not serialised by it. Running acceptance tests in parallel processes made
kintone reject concurrent app creation with `GAIA_DA02` `[measured: poc]`. That is a different failure from
the deploy-in-progress conflict — it was produced by racing Add Preview App, not by writing during a deploy —
and it is worth a note in the provider documentation rather than a retry `[inferred]`.

**The client fixes its own polling cadence and timeout.** kintone documents neither, so both are provider-
side choices rather than API contracts `[inferred]`. Two constraints bound them: a deploy takes long enough
that a tight loop is wasteful, and the 10,000-requests-per-app daily quota above is consumed by polling,
unlike Get Apps and Add Preview App which are excluded from it. When the timeout expires the client reports
the app and the last observed status; it never reports success it did not observe.

The proof-of-concept used a one-second interval — waiting before the first read rather than after it — and a
five-minute deploy timeout covering the deploy and any revert that follows `[measured: poc]`. Those values
were never tuned against measurements, so treat them as a starting point rather than a finding. A
single-setting deploy took a few seconds there; applying several resources to one app took tens of seconds.

**429 is retried with exponential backoff for every method.** Backoff is a provider-side design decision
with no documented contract behind it; kintone neither prescribes nor forbids it `[inferred]`. Retrying it
for non-idempotent methods too rests on the limit being a rejection at admission rather than a failure
part-way through processing. The wording points that way — "a response with an HTTP 429 status code is
returned at the execution of a REST API call" `[documented: S14]` — but it is a reading of that sentence,
not a guarantee kintone gives `[inferred]`.

**5xx responses, transport errors and response-read errors are retried only for idempotent methods.** The
provider cannot see whether a request that failed without a usable response reached kintone. Replaying
`POST /k/v1/preview/app.json` after such a failure can create a second app, and section 3 established that
the first one can never be deleted through the API — it does not even appear in the app list. The
asymmetric policy exists because the cost of a duplicated create is permanent `[inferred]`.

**HTTP idempotence is not the whole test.** The proof-of-concept narrowed several PUTs to 429-only retries
even though PUT is idempotent, because an uncertain replay either consumes a single-use payload — an uploaded
file's `fileKey` — or makes a subsequent revision conflict impossible to attribute `[measured: poc]`. The
axis that actually decides the policy is therefore twofold: whether the response is a definite refusal (a 429
is; a timeout is not), and whether the request body is safe to send twice. Retry on a definite refusal
regardless of method; on an indefinite failure, retry only when both the method and its payload are
replayable `[inferred]`. None of those endpoints is in the v0.1.0 surface, but the rule is what generalises.

**Discard tracked revisions after a revert.** Keeping them made every later write send a stale value, and one
failed deploy cascaded into `GAIA_CO03` conflicts on the same app for the rest of the apply
`[measured: poc]`.

The optimistic-concurrency parameter is the second line of defence. `apps[].revision` on the deploy call,
and `revision` on the settings PUT, cause the request to fail rather than overwrite a concurrent change
`[documented: S4, S3]`. Passing `-1`, or omitting the parameter, disables that check `[documented: S4]`.
Which revision counter each one compares against is open question 2, so it is a defence to switch on
deliberately, after measuring — not a default to send blind `[inferred]`.

## 6. App IDs and revisions are strings

kintone returns numeric identifiers as JSON strings. The response tables type `appId`, `spaceId` and
`threadId` as String, and the sample responses show `"appId": "1"` `[documented: S16, S8]`. Add Preview App
returns both `app` and `revision` as strings `[documented: S2]`. Request parameters are typed "Integer or
String", so the API accepts either on the way in `[documented: S16, S8]`.

Measured on the development environment, all of the following were JSON strings: `apps[].appId` from
`GET /k/v1/apps.json`; `revision` from both `GET /k/v1/app/settings.json` and
`GET /k/v1/preview/app/settings.json`; `numberPrecision.digits`, `numberPrecision.decimalPlaces` and
`numberPrecision.roundingMode` from the live settings read; and `apps[].app` from the deploy-status read
`[measured: dev 2026-08-19]`.

The client therefore carries app IDs and revisions as `string` end to end, and the provider exposes them as
Terraform string attributes. Converting to an integer and back would introduce a formatting decision the API
never asked for, and would turn an unexpected value into a parse error rather than a value that round-trips.
One trap follows from the type: string ordering is not numeric ordering, so app IDs must never be sorted or
compared as strings when the intent is numeric.

## 7. Open questions

These are not settled. Measure each against the development environment — a disposable `tfacc-` app and a
small script — before an implementation relies on it, and record the measured fact here afterwards. Where a
question has a settled half, the settled half is named as such below.

1. **Does a nested object in a settings PUT preserve the children it omits?** The send-shape itself is
   documented: `numberPrecision` and all three of its children are optional, `titleField.selectionMode` is
   required whenever `titleField` is sent, and `titleField.code` is required when `selectionMode` is
   `MANUAL` `[documented: S3]`. What is not documented is the effect of a partial nested object — whether
   sending `numberPrecision` with one child preserves its siblings or resets them. "Parameters that are
   ignored will not be updated" is stated for the table as a whole and never said to apply recursively
   `[documented: S3]`. Measure both objects. Measure the read side too: whether a live read returns a
   non-empty `titleField.code` while `selectionMode` is `AUTO` decides whether an `Optional + Computed`
   attribute for the code produces a perpetual diff.

   What is known: omitting a **top-level** property of the settings PUT preserves the server's value — the
   proof-of-concept relied on that for `theme`, `firstMonthOfFiscalYear` and the record toggles, and its
   acceptance tests passed `[measured: poc]`. It never sent a partial nested object at all, because its
   schema made every child of `numberPrecision` and `titleField` required `[measured: poc]`. Do not
   extrapolate from the top level to the children; that inference is the thing being tested.
2. **Which revision does the deploy check?** The settings PUT is settled: it compares against the preview
   counter, and a stale value is refused with HTTP 409 and `GAIA_CO03` (section 2). `apps[].revision` on the
   deploy call is not — the proof-of-concept never sent it. Until it is measured, omit the parameter, which
   disables the check `[documented: S4]`, and rely on the per-app mutex.
3. **Does a no-op PUT increment the revision?** Relevant to any read-modify-write loop that carries a
   revision forward, and to the "exactly one deployment per settings-only change" criterion. The
   proof-of-concept never found out: it compared the planned settings against state and skipped the PUT
   entirely when nothing had changed `[measured: poc]`.
4. **What error code surfaces for the deploy-in-progress conflict?** The revision mismatch is answered
   (HTTP 409, `GAIA_CO03`). This one is not: nobody has deliberately written to an app mid-deploy, because
   the mutex exists to prevent exactly that. Measure it once, so the client can tell the conflict apart from
   an unrelated 4xx if the mutex is ever bypassed.
5. **What deploy status does an app report when no deploy is in flight, and how long does a finished status
   remain queryable?** One observation exists (`SUCCESS`, section 2) and it is not enough to build a poll
   loop's initial state on. The proof-of-concept never asked, because it only polled after issuing a deploy.
6. **Does a never-deployed app keep its app ID forever?** That no API can remove it is settled in section 3,
   and a revert leaves it in place `[measured: poc 2026-07-29]`. What is not settled is whether the ID is
   ever reclaimed, whether the app counts against any per-domain limit, and whether it can be removed from
   the kintone UI at all. This bounds the damage of an interrupted create.
7. **Can a single-app deploy return `CANCEL`?** Documented only as a multi-app consequence, and never
   observed in the proof-of-concept. The client handles it either way; the answer only tells us whether that
   branch is reachable in the v0.1.0 surface.

## Appendix A — measurements against the development environment

Taken on 2026-08-19 with password authentication. Every request was a `GET`; nothing was created, updated or
deployed. The subdomain and app IDs are omitted deliberately.

| Request | Observation |
| --- | --- |
| `GET /k/v1/apps.json?limit=3` | `apps[].appId` is a JSON string; `spaceId` and `threadId` are `null` for apps outside a space. |
| `GET /k/v1/apps.json?limit=101` | HTTP 400, `{"code":"CB_VA01"}` with a per-field message that the maximum is 100. |
| `GET /k/v1/app/settings.json?app=<id>` | `revision` is a JSON string. `numberPrecision.digits`, `.decimalPlaces` and `.roundingMode` are strings. `titleField` carries `code` and `selectionMode`. The response keys are `name`, `description`, `icon`, `theme`, `titleField`, `numberPrecision`, `firstMonthOfFiscalYear`, `enableThumbnails`, `enableBulkDeletion`, `enableComments`, `enableDuplicateRecord`, `enableInlineRecordEditing`, `revision`. |
| `GET /k/v1/preview/app/settings.json?app=<id>` | `revision` is a JSON string. |
| `GET /k/v1/preview/app/deploy.json?apps%5B0%5D=<id>` | `{"apps":[{"app":"<id>","status":"SUCCESS"}]}` for an app with no deploy in flight. `app` is a string. |
| Any live `GET` | Response headers include `X-ConcurrencyLimit-Limit: 100` and `X-ConcurrencyLimit-Running: 1`. |
| `GET /k/v1/apis.json` | 103 API entries. The only ids ending in `delete` are `guests/delete`, `plugin/delete`, `preview/app/form/fields/delete`, `record/comment/delete`, `records/cursor/delete`, `records/delete`, `space/delete`. No app-level delete exists. |
| `GET /k/v1/app.json?id=<unknown>` | HTTP 404, `{"code":"GAIA_AP01"}`. |
| `GET /k/v1/apps.json` with a wrong password | HTTP 401, `{"code":"CB_WA01"}`. |
| `GET /k/v1/app/settings.json` with a token issued for another app | HTTP 400, `{"code":"GAIA_IA02"}`. |

## Appendix A2 — measurements carried over from the proof-of-concept

Taken in the proof-of-concept repository against a live kintone environment, and reported from it rather than
re-observed here. They are the facts that need a write, which is why this repository could not reproduce
them. Dates are given where they are known.

| Measurement | Date |
| --- | --- |
| A revision is one counter per app, shared by every settings API. | 2026-07-28 |
| A pre-live write advances the preview counter only; the deploy makes live catch up without advancing it further. | 2026-07-28 |
| A pre-live write with a stale revision is refused with HTTP 409 and `GAIA_CO03`, not `GAIA_CO02`. | 2026-07-28 |
| A status read issued about 0.8 s after a `revert: true` deploy reports the revert job's `PROCESSING`, not the previous job's terminal status. | 2026-07-31 |
| A revert against a never-deployed app is a no-op that reports `SUCCESS`; the app is not removed. | 2026-07-29 |
| Omitting a top-level property of the settings PUT preserves the server's value. Partial nested objects were never sent. | — |
| Concurrent `POST /k/v1/preview/app.json` from parallel processes is refused with `GAIA_DA02`. | — |
| A one-second poll interval and a five-minute deploy timeout were sufficient in practice; a single-setting deploy took a few seconds. | — |
| `CANCEL` was never observed, and no 429 or daily-quota rejection was ever produced. | — |

Not carried over: the error codes appearing in the proof-of-concept's unit tests. Its `GAIA_TM01`, `GAIA_ER01`
and `THROTTLE` are `httptest` fixtures invented for those tests, not observations, and must not be treated as
kintone behavior.

## Appendix B — sources

All fetched on 2026-08-19; each returned HTTP 200. Where a fact is documented only in Japanese, the note
says so at the point of use.

| Id | Page |
| --- | --- |
| S1 | Kintone REST API — https://kintone.dev/en/docs/kintone/rest-api/ |
| S2 | Add Preview App — https://kintone.dev/en/docs/kintone/rest-api/apps/add-app/ |
| S3 | Update General Settings — https://kintone.dev/en/docs/kintone/rest-api/apps/settings/update-general-settings/ |
| S4 | Deploy App Settings — https://kintone.dev/en/docs/kintone/rest-api/apps/settings/deploy-app-settings/ |
| S5 | Get General Settings — https://kintone.dev/en/docs/kintone/rest-api/apps/settings/get-general-settings/ |
| S6 | アプリの設定を運用環境へ反映する (Deploy App Settings, Japanese) — https://cybozu.dev/ja/kintone/docs/rest-api/apps/settings/deploy-app-settings/ |
| S7 | Get App Deploy Status — https://kintone.dev/en/docs/kintone/rest-api/apps/settings/get-app-deploy-status/ |
| S8 | Get Apps — https://kintone.dev/en/docs/kintone/rest-api/apps/get-apps/ |
| S9 | App REST API index — https://kintone.dev/en/docs/kintone/rest-api/apps/ |
| S10 | Deleting an App (Kintone Help) — https://get.kintone.help/k/en/app/setup/followup/delete_app.html |
| S11 | Authentication — https://kintone.dev/en/docs/common/authentication/ |
| S12 | 認証 (Authentication, Japanese) — https://cybozu.dev/ja/kintone/docs/rest-api/overview/authentication/ |
| S13 | kintone REST API Overview — https://kintone.dev/en/docs/kintone/rest-api/overview/kintone-rest-api-overview/ |
| S14 | Restrictions and Limitations (Kintone Help) — https://get.kintone.help/k/en/admin/limitation/limit.html |
| S15 | How to avoid kintone REST API limits — https://kintone.dev/en/tutorials/development-productivity/how-to-avoid-kintone-rest-api-limits/ |
| S16 | Get App — https://kintone.dev/en/docs/kintone/rest-api/apps/get-app/ |
| S17 | API Tokens (tutorial) — https://kintone.dev/en/tutorials/introduction-to-kintone-customizations/api-tokens/ |

## Appendix C — why this note lives in `docs/design/`

`docs/` is also where `tfplugindocs` writes the generated provider documentation, so the two have to be
able to share the directory. Before rendering, `tfplugindocs` v0.25.0 deletes `docs/index.md` and the
`resources/`, `data-sources/`, `guides/`, `functions/`, `ephemeral-resources/`, `actions/`,
`list-resources/` and `state-stores/` subdirectories of `docs/` — and nothing else. Everything else in
`docs/` is left alone.

Verified on 2026-08-19 in two ways: by reading the `managedWebsiteFiles` and `managedWebsiteSubDirectories`
lists that drive the deletion in `internal/provider/generate.go` of `terraform-plugin-docs` v0.25.0, and by
running `make docs` twice with a sentinel file in `docs/design/`, which survived both runs.

`docs/design/` is therefore safe. A design note must never be placed at `docs/index.md` or under
`docs/guides/`, both of which `make docs` would delete.
