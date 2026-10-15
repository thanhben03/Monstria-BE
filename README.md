# Monstria-BE

## Hot reload master data

`reload_master_data` reads the server-owned objects in Nakama Storage collection
`catalog` (no player owner): `flower_catalog`, `level_config`, `shop_catalog`, and
`pet_catalog`. Each object uses the JSON format of the matching repository file.

### Setup (one deployment/restart required)

Admin allowlist checking is temporarily disabled. Any authenticated Nakama user
can call `reload_master_data`; anonymous calls are still rejected.
`MASTER_DATA_ADMIN_USER_IDS` is currently ignored, whether empty or populated.

Deploy the updated plugin once:

```powershell
docker compose up -d --build nakama
```

### Apply data changes without restarting

1. Edit the four objects above in Nakama Console Storage. Keep existing IDs that
   are referenced by player inventories, planted flowers, or pet placements.
2. Authenticate as a Nakama user and call
   RPC `reload_master_data` with payload `{}` using that user's session.
3. Check the returned `version` (SHA-256 of the source JSON) and `counts`
   (`flowers`, `levels`, `shopItems`, `pets`). Clients caching catalogs should
   fetch them again after a successful reload.

All four objects must exist and pass their catalog validators. On a missing
object, invalid data, or storage read failure, the RPC returns an error and keeps
all active caches unchanged. Fix the Storage objects and retry. Reload does not
write Storage or migrate existing player state. Validation covers each catalog's
schema and rules; it does not migrate removed IDs or validate all cross-catalog
references.

Shop and bau cua now read the active shop cache, too: Storage edits become active
only after successful reload (or startup). Publication holds one shared lock, so
readers cannot see a partially published set of caches. An already-running RPC
can perform separate lookups on either side of a reload; it is not pinned to one
catalog version for its entire duration.

This RPC updates **only the Nakama process handling the call**. The current
Compose deployment has one Nakama instance. For multiple instances, invoke it on
each instance directly; a load-balanced call does not broadcast the reload.
Automatic version polling is not implemented.

Editing local JSON files does not update Storage: Docker copies them into the
image, and startup generally prefers existing Storage. Pet JSON is also embedded
as a seed. To roll back, restore the previous Storage JSON and call reload again.
Existing startup behavior still applies, including level data being reseeded from
the bundled file when `ENABLE_DEBUG_RPCS=true`; keep the release JSON in sync with
intended persistent data before restarting.

### Checks

```text
go test ./...
go test -race ./...
```

The race detector requires CGO and a supported C compiler on the test host.
