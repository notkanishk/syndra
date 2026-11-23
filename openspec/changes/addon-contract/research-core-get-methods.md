# TrueNAS `core.get_methods` entry shape — research

Read on 23 Nov 2026 for a schema fingerprint (hash of each method's `accepts` / `returns`). Research only; nothing built. Every fact is from the public `truenas/middleware` source, fetched with `gh api` and `curl` from raw.githubusercontent.com, and read at tag `TS-25.10.8`. Paths are under `src/middlewared/middlewared/`. **Nothing was executed**: no middleware was started and no NAS was touched. So the shapes below are derived from code, not captured from a live reply. Where the source does not settle a point, it says "not verified".

Permalink base: `https://github.com/truenas/middleware/blob/29ed02b8082f38494b0d2896bada8151f0d59716/src/middlewared/middlewared/` (TS-25.10.8). Write it as `P/` below.

## 0. The answer in one screen

| # | Fact | Source |
|---|---|---|
| A | The 25.10 release line is the `TS-25.10.N` tags and the `release/25.10.N` branches, **not** the `stable/goldeye` branch. Latest read: `TS-25.10.8` = `release/25.10.8` = `29ed02b8082f38494b0d2896bada8151f0d59716` (26 Oct 2026). | §1 |
| B | `core.get_methods(service=None, target='WS')` returns `dict[str, Any]`: method name → one dict with **17 keys** (§2). | §2 |
| C | Only **new-style** (pydantic) schemas remain. The legacy `@accepts` path is gone from `core.get_methods`. | §3 |
| D | `accepts` = **list of per-argument schemas**, one per field of the Args model, in field order. `returns` = **list with exactly one schema** (the `result` field). | §3 |
| E | `$ref` / `$defs` never appear: they are inlined. Non-standard keys `_attrs_order_`, `_name_`, `_required_` are added, `title` is overwritten, `items` becomes a one-element list. | §3 |
| F | The entry is built from code. Dict order, `roles` (sorted) and model order are deterministic. **But the set of methods in the reply depends on the caller's credential** (§4.4). Use one fixed credential for fingerprinting. | §4 |
| G | `core.get_methods` reports the **current** API models only. A versioned socket endpoint adapts the call, not the schemas (inference; §4.5). | §4 |
| H | **A patch release can change schemas.** `api/current.py` moved v25_10_0 → v25_10_1 → v25_10_2 → v25_10_4 → v25_10_5 across 25.10 patches, and the model files differ (§6). | §6 |
| I | In 26 (`stable/26`, `master`) the entry **changes shape**: `item_method` and `require_websocket` are gone; `no_authz_required` and `cli_private` are new (§6). | §6 |
| J | The method is `authorization_required=False`: any authenticated caller can call it. Docstring and result doc below (§7). | §7 |

## 1. Which branch is 25.10

Queried the repo's branches, tags and `api/current.py` per tag.

- Tags: `TS-25.10-BETA.1`, `TS-25.10-RC.1`, `TS-25.10.0`, `TS-25.10.0.1`, `TS-25.10.1`, `TS-25.10.2`, `TS-25.10.2.1`, `TS-25.10.2.2`, `TS-25.10.3`, `TS-25.10.3.1`, `TS-25.10.4` … `TS-25.10.8`.
- Branches: `release/25.10.0` … `release/25.10.8` exist beside `stable/goldeye`.
- `TS-25.10.8` and `release/25.10.8` both resolve to `29ed02b8082f38494b0d2896bada8151f0d59716` (committed 2026-10-26).
- `stable/goldeye` is at `af2c5abf50e08e574dd01f023598a43814503d1f` (2026-07-07, message "Stable/goleye runtest.py (#19270)"). Its `api/current.py` still says `from .v25_10_2 import *`. GitHub compare against `TS-25.10.8`: "diverged", 103 ahead, 3 behind. **It is a stale side branch, not the shipped 25.10.** Do not read the schema from it.
- Earlier research in this directory (`research-account-delete.md`) read `TS-25.10.7`. `get_methods` is byte-identical in `TS-25.10.0` and `TS-25.10.8` (the function body diffed empty), so the choice of tag does not change §2.

Not verified: that the NAS the owner runs is exactly 25.10.8. The add-on must read the version from the NAS (`system.version`) and not assume.

## 2. Where `core.get_methods` lives and the entry keys

File: `service/core_service.py` at `TS-25.10.8`. [`P/service/core_service.py#L247-L250`]

```python
@api_method(CoreGetMethodsArgs, CoreGetMethodsResult, authorization_required=False, pass_app=True)
def get_methods(self, app, service, target):
    """
    Return methods metadata of every available service.
    """
```

Parameters, from `api/v25_10_5/core.py:43-49`: `service: str | None = None` ("Filters the result for a single service."), `target: Literal['WS', 'CLI', 'REST'] = 'WS'`.

The entry is built at `core_service.py:364-380`. [`P/service/core_service.py#L364-L380`]

```python
data[method_name] = {
    'description': doc,
    'cli_description': (doc or '').split('\n\n')[0].split('.')[0].replace('\n', ' '),
    'examples': examples,
    'item_method': True if item_method else hasattr(method, '_item_method'),
    'no_auth_required': no_auth_required,
    'filterable': issubclass(method.new_style_accepts, QueryArgs),
    'filterable_schema': None,
    'pass_application': hasattr(method, '_pass_app'),
    'require_websocket': hasattr(method, '_pass_app') and not method._pass_app['rest'],
    'job': hasattr(method, '_job'),
    'downloadable': hasattr(method, '_job') and 'output' in method._job['pipes'],
    'uploadable': hasattr(method, '_job') and 'input' in method._job['pipes'],
    'check_pipes': hasattr(method, '_job') and method._job['pipes'] and method._job['check_pipes'],
    'roles': self.middleware.role_manager.roles_for_method(method_name),
    **method_schemas,
}
```

`method_schemas` (lines 356-359) adds `accepts` and `returns`. Total **17 keys**, exactly:

| Key | Type | Notes |
|---|---|---|
| `description` | str or None | docstring with `.. examples::` sections cut off |
| `cli_description` | str | first sentence of the docstring |
| `examples` | dict[str, list[str]] | keys `__all__`, `cli`, `rest`, `websocket` (lines 343-353) |
| `item_method` | bool | `True` for CRUD `update` and `delete` (lines 278-279) |
| `no_auth_required` | bool | |
| `filterable` | bool | true when Args subclasses `QueryArgs` |
| `filterable_schema` | None | hard-coded `None` in 25.10 |
| `pass_application` | bool | |
| `require_websocket` | bool | |
| `job` | bool | |
| `downloadable` | bool | |
| `uploadable` | bool | |
| `check_pipes` | bool or falsy | `hasattr(...) and pipes and check_pipes` can be `False`, `{}` or `[]`-like, not always a bool. Normalise before hashing. |
| `roles` | list[str] | sorted (§4.2) |
| `accepts` | list | §3 |
| `returns` | list | §3 |

Which methods appear (lines 253-326):
- Services come from `self.middleware.get_services()`; `_should_list_service` (lines 207-215) hides `private` services (except `resttest` for REST) and, for `target == 'CLI'`, `cli_private` services.
- Attributes starting with `_` are skipped (line 261). `terminate` is skipped (312). Methods with `_private is True` are skipped (306). Methods with `_cli_private` are skipped only when `target == 'CLI'` (308).
- For CRUD services `create/update/delete` report the `do_*` method but keep the public name (`user.create`, lines 274-279); `do_create/do_update/do_delete` are skipped (280-281). For config services `update` reports `do_update` (288-297).
- A caller filter exists: see §4.4.

## 3. What `accepts` and `returns` hold

**New-style only.** [`P/service/core_service.py#L355-L362`]

```python
method_schemas = {
    'accepts': get_json_schema(method.new_style_accepts),
    'returns': get_json_schema(method.new_style_returns),
}
```

There is no legacy branch. `method.new_style_accepts` is set by `@api_method` (`api/base/decorator.py:158-159`). A public method with no `@api_method` would raise `AttributeError` here (the `except` logs and re-raises, lines 360-362). A scan of `plugins/` found no `@accepts(` and no import of the old `accepts` decorator (not a proof for every file; the scan used `grep -rln`). Whether something else guarantees every public method is decorated: not verified.

`get_json_schema` is in `api/base/jsonschema.py`. [`P/api/base/jsonschema.py#L10-L15`]

```python
def get_json_schema(model: type["BaseModel"]) -> list:
    schema = model.model_json_schema()
    schema = replace_refs(schema)
    schema = add_attrs(schema)

    return [schema["properties"][name] for name in model.schema_model_fields()]
```

So:
- Source is pydantic's own `model_json_schema()` (default validation mode) on the method's Args and Result models.
- **`accepts` is a list, one element per Args-model field**, in `model_fields` order, keyed by `field.alias or name`. `schema_model_fields` drops fields marked `SkipJsonSchema` (`api/base/model.py:219-224`). A method with no arguments (`alert.list`, `system.info`, `system.version`) gets `[]`.
- **`returns` is a list with one element**: the schema of the `result` field. `@api_method` refuses a Result model whose fields are not exactly `["result"]` (`decorator.py:78-79`).
- Each element is the schema of one **field**, not of the model. So the argument's own name is not a key of the element except through `_name_` / `title` below.

Post-processing (`jsonschema.py:18-62`), the load-bearing parts:

```python
defs = data.pop("$defs", defs)
if "$ref" in data:
    ref = data.pop("$ref")
    data = {**defs[ref.removeprefix("#/$defs/")], **data}
```
→ **`$defs` and `$ref` are inlined**; none should remain. A sibling key next to a `$ref` (for example `description`) overrides the definition's.

```python
schema = {**schema,
    "_attrs_order_": list(schema["properties"].keys()),
    "properties": {k: {**v, "title": k, "_name_": k, "_required_": k in schema.get("required", [])} ...}}
...
schema["items"] = [schema["items"]]  # FIXME: Non-standard compliant
```
→ For every `type: object` with `properties`: adds `_attrs_order_` (list of property names), and on each child property sets `title` = the property name (overwrites pydantic's title), `_name_`, `_required_`. The standard `required` array stays. For `type: array` with a non-list `items`, `items` becomes a **one-element list**. Both are marked `FIXME ... backwards compatibility` in source.

Other keys come from pydantic and the models: `type`, `anyOf`, `enum`, `const`, `default`, `description` (from attribute docstrings; `use_attribute_docstrings=True`, `api/base/model.py:149-155`), `examples` (from `Field(examples=[...])`), `minimum`, `maxLength` (`str_max_length=1024` is a model-wide config), `additionalProperties: false` (`extra="forbid"`). I did not run pydantic, so the exact JSON for any one method is **not verified**; take a real capture from staging before locking a fixture.

**Tidy shape for a CRUD `query`.** `user.query` is not written by hand. `CRUDService` injects `api_method(QueryArgs, query_result_model)` (`service/crud_service.py:71-80`). So `accepts` is the generic two-field list `[filters, options]` and `returns` is one schema of `result: list[Item] | Item | int` (`api/base/model.py:307-314`). `filterable` is `True` for these. `filterable_api_method` (`service/decorators.py:23-56`) does the same for hand-written `query` methods. The item schema inside `returns` is the one that changes when a field is added to an entry model.

## 4. Stability and canonicalizing

### 4.1 Dict order
`dir(svc)` is alphabetical, so method order is stable; but do not rely on JSON object order in a hash. Canonicalise: sort object keys everywhere. **Do not sort the `accepts` list, the `items` list, `_attrs_order_` or `anyOf`**: their order is the argument or declaration order and carries meaning.

### 4.2 `roles`
`RoleManager.roles_for_resource` returns `sorted(roles)` (`role.py:302-315`). Deterministic.

### 4.3 `required`, `enum`, `anyOf`
Not sorted by the middleware. Their order follows the model source (field order, `Literal[...]` order, union order). I found no `set` in the path. Unchanged code gives unchanged order; a reordering in a patch is a real schema edit to pydantic but harmless to a client. If the fingerprint must ignore pure reordering, sort `required` and `enum` in the canonical form and say so. Not verified against a live capture.

### 4.4 Caller-dependent: the reply is filtered by credential
`core_service.py:319-326`:

```python
# Skip methods that are not allowed for the currently authenticated credentials
if app is not None:
    if not no_auth_required:
        if not app.authenticated_credentials:
            continue
        if not no_authz_required and not app.authenticated_credentials.authorize('CALL', method_name):
            continue
```

So a caller with a narrow API-key role sees **fewer methods**. A missing method is not a removed method. The fingerprint must be read with one fixed credential (the add-on's own standing key) and a method that is absent must be reported as "not visible to this key", never as "gone". Which of the §5 methods a given key sees: not verified (depends on its roles).

### 4.5 `target` and API version
- `target` only changes **which methods are listed** (private and CLI-private filtering, §2). Each method's `accepts` / `returns` is the same for WS, CLI and REST. `require_websocket` is per method, not per target.
- `get_json_schema(method.new_style_accepts)` uses the model object on the live method, i.e. the **current** API version (`api/current.py`, §6). A versioned WebSocket endpoint wraps calls in `LegacyAPIMethod` (`api/base/server/legacy_api_method.py:15-106`), which converts **parameters and the result of a call** between the current and the older version using the older model. For `core.get_methods` the result model is `dict[str, Any]`, so it carries the current schemas through. **That is an inference from reading the adapter; I did not test `/api/current` against `/api/v25.10.0`.** Verify on staging before relying on it.
- The per-version schemas do exist in source (`api/v25_10_*/`), and `api/base/server/doc.py` dumps a per-version method list (`APIDump`). That is a different path from `core.get_methods`; whether it is reachable over the API: not verified.

### 4.6 Runtime state
From code, the entry depends on: the loaded plugins, the registered roles and the caller. I found nothing that reads hostname, time or environment in `api/v25_10_5/` (grepped `gethostname`, `time.time`, `datetime.now`, `os.environ`, `subprocess`: no hits). Models import constants only (for example `DEFAULT_HOME_PATH`, `SMBShareField`). Two `default_factory=lambda: ReportingQuery()` exist in `reporting.py`, which is static. Not verified: whether HA or licence state changes which services load (`get_services()` is the loaded set); no such condition was read.
- **Docstrings are part of the entry** (`description`, `cli_description`, `examples`), and field docstrings are in the schema (`description`). A docs-only patch changes a naive hash. Hash `accepts` and `returns` after stripping `description` and `examples` if the aim is to detect contract change, not wording.

### 4.7 Private and undocumented
- Private methods are excluded (`core_service.py:306`, and the service-level check at 207-215). `smb.status` is `private=True` (`plugins/smb_/status.py:52`), so it is **not** in the reply.
- A method with no docstring is **not** excluded: `doc` is simply `None` and `cli_description` is `''` (line 366).

## 5. The named methods at `TS-25.10.8`

Style: all are new-style. Model files are under `api/v25_10_5/`; plugin files under `plugins/`. "generated" means the Args/Result are built by `CRUDService`, not named in the plugin.

| Method | Exists | Args model | Result model | Plugin | Model file |
|---|---|---|---|---|---|
| `user.query` | yes, generated | `QueryArgs` | `UserQueryResult` (`list[UserQueryResultItem] \| UserQueryResultItem \| int`, from `UserEntry`) | `account.py:230` (`UserService`, `entry = UserEntry` at 242) | `user.py:37` |
| `user.create` | yes (`do_create`) | `UserCreateArgs` (`user_create: UserCreate`) | `UserCreateResult` | `account.py:612` | `user.py:225` |
| `user.update` | yes (`do_update`, `item_method`) | `UserUpdateArgs` (`id`, `user_update`) | `UserUpdateResult` | `account.py:764` | `user.py:235` |
| `user.delete` | yes (`do_delete`, `item_method`) | `UserDeleteArgs` (`id`, `options`) | `UserDeleteResult` (`result: int`) | `account.py:1017` | `user.py:252` |
| `group.query` | yes, generated | `QueryArgs` | `GroupQueryResult` | `account.py:1925` (`entry = GroupEntry`, 1934) | `group.py:19` |
| `system.info` | yes | `SystemInfoArgs` (no fields) | `SystemInfoResult` (`@single_argument_result`) | `system/info.py:83`, roles `READONLY_ADMIN` | `system.py:36,40` |
| `system.version` | yes, `authorization_required=False` | `SystemVersionArgs` (no fields) | `SystemVersionResult` (`result: str`) | `system/product.py:108` | `system_product.py:61,65` |
| `alert.list` | yes | `AlertListArgs` (no fields) | `AlertListResult` (`list[Alert]`) | `alert.py:373`, roles `ALERT_LIST_READ` | `alert.py:108,112` |
| `pool.query` | yes, generated | `QueryArgs` | `PoolQueryResult` | `pool_/pool.py:43` (`entry = PoolEntry`, 53) | `pool.py:55` |
| `service.query` | yes, `filterable_api_method` | `QueryArgs` | `ServiceQueryResult` | `service.py:93` | `service.py:16` |
| `pool.dataset.query` | yes, `filterable_api_method` | `QueryArgs` | `PoolDatasetQueryResult` | `pool_/dataset.py:125` | `pool_dataset.py:62` |
| `pool.dataset.get_quota` | yes | `PoolDatasetGetQuotaArgs` (`dataset`, `quota_type`, `filters`) | `PoolDatasetGetQuotaResult` (`list[PoolDatasetQuota]`) | `pool_/dataset_quota.py:150`, roles `DATASET_READ` | `pool_dataset.py:624,635` |
| `sharing.smb.query` | yes, generated | `QueryArgs` | `SmbShareQueryResult` | `smb.py:713` (`namespace = 'sharing.smb'`, `entry = SmbShareEntry` at 725) | `smb.py:597` |
| `smb.status` | exists but **private** | `SMBStatusArgs` | `SMBStatusResult` | `smb_/status.py:52` (`private=True`) | not read |
| `audit.query` | yes | `AuditQueryArgs` (`data: AuditQuery`) | `AuditQueryResult` (`int \| AuditQueryResultItem \| list[...]`) | `audit/audit.py:120` | `audit.py:146,151` |
| `core.get_jobs` | yes, `filterable_api_method` | `QueryArgs` | `CoreGetJobsResult` (from `CoreGetJobsItem`, `authorization_required=False`) | `service/core_service.py:118` | `core.py:84` |
| `api_key.create` | yes (`do_create`) | `ApiKeyCreateArgs` (`api_key_create`) | `ApiKeyCreateResult` (`ApiKeyEntryWithKey`) | `api_key.py:161` | `api_key.py:64,69` |
| `api_key.delete` | yes (`do_delete`) | `ApiKeyDeleteArgs` (`id`) | `ApiKeyDeleteResult` (`Literal[True]`) | `api_key.py:287` | `api_key.py:92,97` |
| `core.get_methods` | yes | `CoreGetMethodsArgs` | `CoreGetMethodsResult` (`result: dict[str, Any]`) | `service/core_service.py:247` | `core.py:43,52` |

Notes:
- "Generated" result names follow `entry.__name__.removesuffix("Entry") + "QueryResult"` (`api/base/model.py:307-314`; `service/decorators.py:31-41`). I derived the names from that rule and did not see them printed, except where a model is named in the plugin.
- `core.get_methods` returns `dict[str, Any]`, so its own `returns` schema says nothing about the entry shape. The shape is only in `core_service.py`. A fingerprint cannot use a schema for it.
- `pool.dataset.query` uses `pass_thread_local_storage=True`; irrelevant to the schema.

## 6. Patch releases and the 26 line

### 6.1 Patch releases change schemas
`api/current.py` (line 8) per tag:

| Tag | `current` is |
|---|---|
| `TS-25.10.0`, `TS-25.10.0.1` | `v25_10_0` |
| `TS-25.10.1` | `v25_10_1` |
| `TS-25.10.2`, `.2.1`, `.2.2`, `.3`, `.3.1` | `v25_10_2` |
| `TS-25.10.4` | `v25_10_4` |
| `TS-25.10.5`, `.6`, `.7`, `.8` | `v25_10_5` |

(`v25_10_3` exists as a directory but no tag read points `current` at it.) Each `api/v25_10_N/` has 129 entries. Differences:
- `v25_10_0` → `v25_10_1`: 14 files differ, including `user.py`, `api_key.py`, `smb.py`, `core.py`, `pool_snapshot.py`.
- `v25_10_4` → `v25_10_5`: only `app_image.py` and `reporting.py`.
- `v25_10_0` → `v25_10_5`: 19 files differ, including `user.py`, `api_key.py`, `audit.py`, `pool.py`, `smb.py`.

Concrete examples between `v25_10_0` and `v25_10_5` (verbatim from `diff`): `user.py` `email: EmailString | None` → `email: EmailStr | None` (type changed, plus a new docstring); `smb.py` adds `hostsallow` and `hostsdeny` list fields to the share models (so `sharing.smb.query` / `create` / `update` schemas gain properties); `audit.py` adds `model_config = ConfigDict(extra='allow')` to one model; `api_key.py` loses a `to_previous` hook (not a schema change by itself). **Many diffs are docstring and examples only**, which still change a raw hash (§4.6).

So a fingerprint taken on 25.10.0 will not match 25.10.8 for `user.*`, `sharing.smb.*` and `audit.query` at least. The fingerprint must be stored with the version it was read from.

### 6.2 `core.get_methods` in 26 and master
Read `stable/26` (`59e637259069cd1b43e33a39e1c300aa1e2f2877`) and `master` (`1f40697f1c17db2f0158c4c5e61eb2d321889483`); their `get_methods` bodies are identical to each other. `P26/service/core_service.py`: the method still exists, is still public, still takes `(service, target)`.

What changed in the **entry shape** (from `_get_all_methods`, `stable/26 core_service.py` lines ~362-378):
- **Removed:** `item_method`, `require_websocket`.
- **Added:** `no_authz_required`, `cli_private`.
- **Unchanged:** `description`, `cli_description`, `examples`, `no_auth_required`, `filterable`, `filterable_schema`, `pass_application`, `job`, `downloadable`, `uploadable`, `check_pipes`, `roles`, `accepts`, `returns`.
- **New mechanism:** the full map is read from a build-time cache `METHODS_CACHE_PATH = '/usr/share/middlewared/methods.json'` (line 57) when it exists, otherwise built at runtime; the filter by credential and `target` is then applied on the cached entries.
- `get_json_schema` is still the producer of `accepts` / `returns` in 26 (`master` lines 380-383).

Consequence: a client that reads `item_method` or `require_websocket` breaks on 26. Read them as optional. I did not check whether `api/base/jsonschema.py` changed in 26: not verified.

## 7. The official wording

Result model (`api/v25_10_5/core.py:52-54`): [`P/api/v25_10_5/core.py#L52-L54`]

> `result: dict[str, Any]` — "Object mapping method names to their signatures, documentation, and metadata."

Method docstring (`core_service.py:249-251`):

> "Return methods metadata of every available service."

Args docs (`core.py:44-49`): `service` — "Filters the result for a single service."; `target` — "Target interface to get methods for. `WS` for WebSocket, `CLI` for command line, `REST` for HTTP API."

## 8. What this means for the fingerprint (design inputs, not a decision)

1. Read with one fixed credential and record which methods were visible; treat "not visible" separately from "gone" (§4.4).
2. Canonical input per method: `accepts` and `returns` only, object keys sorted, list order kept, optional removal of `description` / `examples` / `title` (docstring-only changes are common, §6.1) and of `_attrs_order_` if ordering noise is unwanted. Decide this once and version the canonical form.
3. Store the fingerprint with the NAS version string (`system.version`) and the `api/current` model version if it can be known; patch releases do change models (§6.1).
4. Do not read `item_method` / `require_websocket` as required keys (§6.2).
5. Before locking: capture one real reply from staging (read-only, `core.get_methods` is a read) and check it against §3, especially `$defs` absence, one-element `returns`, `items` as a one-element list, and the `/api/v25.10.0` inference in §4.5. The add-on is read-only on staging by ruling, and this call writes nothing.

## 9. Not verified

- Any live reply from a TrueNAS box.
- The exact pydantic JSON for any method (pydantic was not run).
- That every public method has `@api_method` (only that `get_methods` would raise if one did not).
- Whether HA or licence state changes the loaded service set.
- `/api/current` versus `/api/v25.10.0` returning identical `core.get_methods` content (inferred from the adapter code).
- Which of the §5 methods a given API-key role can see.
- `SMBStatusArgs/Result` file and line (the method is private and out of the reply).
- Whether `api/base/jsonschema.py` or `model.py` changed in 26.
