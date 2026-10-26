# Monstria-BE

## Hot reload master data from JSON files

Upload these four files into the directory visible to the running Nakama process:

- `flower_catalog.json`
- `level_config.json`
- `shop_catalog.json`
- `pet_catalog.json`

Call `reload_master_data` with payload `{}` using an authenticated Nakama session.
The RPC rereads the files on every call, validates all four, writes the validated
catalogs to Storage in one batch, then publishes all four in-memory caches under
one shared lock. It returns `directory`, `version` (SHA-256 of source JSON), and
`counts` (`flowers`, `levels`, `shopItems`, `pets`). Pet reload reads the uploaded
file, not the JSON embedded in the plugin.

### Setup (deploy once)

The default directory is `/nakama/data/modules`, matching this repository's
Dockerfile. To use `/data/modules`, put this in the project `.env` file:

```dotenv
MASTER_DATA_DIR=/data/modules
```

The directory must exist **inside the container**, not just on the host. Compose
forwards `MASTER_DATA_DIR` into Nakama. Deploy the updated code and configuration:

```powershell
docker compose up -d --build nakama
```

Admin allowlist checking remains temporarily disabled. Any authenticated user
can call this RPC; anonymous calls are rejected. `MASTER_DATA_ADMIN_USER_IDS` is
ignored.

### Upload and reload (no restart)

For `/data/modules`, create the destination once and upload the JSON files:

```powershell
docker exec nakama mkdir -p /data/modules
docker cp ./flower_catalog.json nakama:/data/modules/flower_catalog.json
docker cp ./level_config.json nakama:/data/modules/level_config.json
docker cp ./shop_catalog.json nakama:/data/modules/shop_catalog.json
docker cp ./pet_catalog.json nakama:/data/modules/pet_catalog.json
```

If using the default directory, replace `/data/modules` above with
`/nakama/data/modules`. After the first upload, unchanged files can remain there;
upload only changed files, finish uploading, then call `reload_master_data` with
`{}`. The payload does not select a path; `MASTER_DATA_DIR` does.

For a persistent host upload directory, add a bind mount to the Nakama service
and ensure all four files are present there:

```yaml
volumes:
  - ./master-data:/data/modules:ro
```

Set `MASTER_DATA_DIR=/data/modules` with that mount. Deploy the Compose change
once; subsequent file uploads and reload calls need no restart. Avoid mounting
an empty directory over `/nakama/data/modules`, which also contains `backend.so`.
Files copied directly into a container are lost when it is recreated; a bind
mount keeps the host files available.

### Failure behavior and scope

Missing/unreadable files, invalid catalogs, or a failed Storage write prevent
cache publication. Validation errors do not write Storage. Fix the reported file
and retry. Reload does not migrate existing player state. Flower disease IDs
are checked against `diseaseDefinitions` before any Storage write; other
cross-catalog references are not all validated. Preserve IDs referenced by
existing players.

Shop and bau cua use the active cache too. Clients caching catalogs should fetch
them again after successful reload. To roll back, upload the previous JSON files
and call reload again.

Reload updates only the Nakama instance handling the call. With multiple
instances, deploy the files and invoke reload directly on each instance.
Automatic version polling is not implemented. Individual reads are synchronized;
an RPC already in progress can perform separate reads on either side of a reload.

Startup behavior is unchanged: it generally loads persisted Storage, but debug
mode reseeds level data from the bundled level JSON. Keep release JSON in sync
before restarting. `MASTER_DATA_DIR` controls this reload RPC, not the existing
startup catalog path settings.

### Plants stop spawning bugs after changing species

Startup prefers `catalog/flower_catalog` in Storage. Rebuilding the plugin or
editing the bundled JSON does not normally replace an existing stored catalog.
For example, stored `borua`/`bocanhcung` references will match no candidates if
the configured species are only `buom`/`chuonchuon`. Species IDs remain distinct;
there is no automatic aliasing between these insects.

Upload the updated `flower_catalog.json` into `MASTER_DATA_DIR` inside the
running container, then call `reload_master_data` as described above. Verify
`get_flower_catalog` returns the intended disease IDs. Startup now warns about
undefined species, and reload rejects undefined disease references.

Test with newly planted, watered plants: ticks already processed with the old
catalog are not replayed. Bug spawning is probabilistic, and plants which have
already had a disease do not get another during the same crop.

### Checks

```text
go test ./...
go vet ./...
go test -race ./...
```

The race detector requires CGO and a supported C compiler on the test host.
