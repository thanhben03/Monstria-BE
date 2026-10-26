# Fertilizer RPC

`fertilize_plant_in_pot` takes `{"slotId":"0_0","itemId":"fertilizer_30"}` and returns the updated `inventory` and `garden`.

Supported inventory IDs: `fertilizer_15`, `fertilizer_30`, `fertilizer_50`. Rates in `fertilizerRates` are server-owned and apply to remaining growth time, not total growth time. Admins can add these stacks to the existing `player_state/inventory.items` array; preserve other inventory entries. Shop and gift integration are deferred.

Each live, watered, growing plant can receive fertilizer once. A successful request consumes one item. Dead, unwatered, mature, missing or previously fertilized plants are rejected without a storage write. Inventory and garden writes are atomic and version checked. Duplicate requests cannot consume a second item. A newly planted crop starts unfertilized.

New optional plant fields: `fertilized` (bool), `growthReductionSeconds` (float64). Existing records default to false/zero. Harvest readiness uses `growthStartedAt + growSeconds - growthReductionSeconds`; disease growth progress also includes the reduction. Fractional seconds are preserved.

Deploy the rebuilt backend before the Unity client. The full gameplay, Unity setup, animation hook and manual QA checklist are in `D:/UnityProject/vntm/Documentation/fertilizer.md`.

Validation: `go test ./... -run 'Test(Fertilizer|Garden|SlotPlacement|Consume|NormalizeInventory)' -count=1`, plus `go vet ./...`.
