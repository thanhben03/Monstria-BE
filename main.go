package main

import (
	"context"
	"database/sql"

	"github.com/heroiclabs/nakama-common/runtime"
)

func InitModule(
	ctx context.Context,
	logger runtime.Logger,
	db *sql.DB,
	nk runtime.NakamaModule,
	initializer runtime.Initializer,
) error {

	logger.Info("Module loaded!")

	if err := bootstrapFlowerDefinitionsStorage(ctx, nk); err != nil {
		return err
	}
	if err := bootstrapShopItemDefinitionsStorage(ctx, nk); err != nil {
		return err
	}
	if err := bootstrapLevelDefinitionsStorage(ctx, nk); err != nil {
		return err
	}

	err := initializer.RegisterRpc("ping", PingRPC)
	if err != nil {
		return err
	}

	if err := initializer.RegisterRpc("get_player_inventory", GetPlayerInventoryRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("get_player_resources", GetPlayerResourcesRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("debug_level_up", DebugLevelUpRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("set_player_inventory", SetPlayerInventoryRPC); err != nil {
		return err
	}

	if err := initializer.RegisterRpc("get_player_garden", GetPlayerGardenRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("place_pot_on_slot", PlacePotOnSlotRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("store_pot_in_inventory", StorePotInInventoryRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("store_layer_pots_in_inventory", StoreLayerPotsInInventoryRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("plant_seed_in_pot", PlantSeedInPotRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("water_plant_in_pot", WaterPlantInPotRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("get_flower_catalog", GetFlowerCatalogRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("get_shop_catalog", GetShopCatalogRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("get_shop_item", GetShopItemRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("purchase_shop_item", PurchaseShopItemRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("harvest_plant_in_pot", HarvestPlantInPotRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("destroy_dead_plant", DestroyDeadPlantRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("treat_plant_disease", TreatPlantDiseaseRPC); err != nil {
		return err
	}

	if err := initializer.RegisterRpc("get_player_cloud_layers", GetPlayerCloudLayersRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("update_cloud_layers", UpdateCloudLayersRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("get_player_decor", GetPlayerDecorRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("place_decor_on_slot", PlaceDecorOnSlotRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("store_decor_in_inventory", StoreDecorInInventoryRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("store_layer_decors_in_inventory", StoreLayerDecorsInInventoryRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("bau_cua_get_state", BauCuaGetStateRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("bau_cua_place_bet", BauCuaPlaceBetRPC); err != nil {
		return err
	}

	err = RegisterAuthHooks(initializer)
	if err != nil {
		return err
	}

	return nil
}

func PingRPC(
	ctx context.Context,
	logger runtime.Logger,
	db *sql.DB,
	nk runtime.NakamaModule,
	payload string,
) (string, error) {

	logger.Info("Ping RPC called")

	return `{"message":"pong"}`, nil
}
