# Fertilizer RPC

`fertilize_plant_in_pot` takes `{"slotId":"0_0","itemId":"fertilizer_30"}` and returns the updated `inventory` and `garden`.

Supported inventory IDs: `fertilizer_15`, `fertilizer_30`, `fertilizer_50`. Rates in `fertilizerRates` are server-owned and apply to remaining growth time, not total growth time. Admins can add these stacks to the existing `player_state/inventory.items` array; preserve other inventory entries. Shop and gift integration are deferred.

Each live, watered, growing plant can receive fertilizer once. A successful request consumes one item. Dead, unwatered, mature, missing or previously fertilized plants are rejected without a storage write. Inventory and garden writes are atomic and version checked. Duplicate requests cannot consume a second item. A newly planted crop starts unfertilized.

New optional plant fields: `fertilized` (bool), `growthReductionSeconds` (float64). Existing records default to false/zero. Harvest readiness uses `growthStartedAt + growSeconds - growthReductionSeconds`; disease growth progress also includes the reduction. Fractional seconds are preserved.

Deploy the rebuilt backend before the Unity client. The full gameplay, Unity setup, animation hook and manual QA checklist are in `D:/UnityProject/vntm/Documentation/fertilizer.md`.

Validation: `go test ./... -run 'Test(Fertilizer|Garden|SlotPlacement|Consume|NormalizeInventory)' -count=1`, plus `go vet ./...`.
# Shop configuration

`shop_catalog.json` contains three sample offers in `items.fertilizer`:

| ID | Coin price | Required level |
| --- | ---: | ---: |
| fertilizer_15 | 100 | 1 |
| fertilizer_30 | 250 | 5 |
| fertilizer_50 | 500 | 10 |

All samples start with `disabled: true`. Set it to `false` to open sales.
`coinPrice` and `gemPrice` control available payment methods; zero disables that
currency, and at least one price must be positive. `quantity` is the number of
bags per purchase unit. The client selects 1–99 purchase units; the server checks
the level, currency and quantity, then atomically charges the wallet and grants
`quantity * purchaseQuantity` bags to the common inventory.

Keep `grantType: "fertilizer"` and `grantItemId` equal to the existing fertilizer
ID. Names, `summary`, `desc`, prices, levels and availability are backend-owned.
Disabled offers are excluded before catalog pagination and cannot be purchased.
Locked offers remain visible, with their `requiredLevel` returned to the client.

Deploy the updated Go plugin once to enable the new grant type. For an existing
server, editing bundled JSON alone does not replace the persisted shop catalog:
upload the JSON to `MASTER_DATA_DIR`, then call `reload_master_data` (see README).
Later price/level/availability changes need only JSON upload and reload. Reopen
the shop to fetch the latest catalog. No server deployment/reload is performed
by these source changes.
