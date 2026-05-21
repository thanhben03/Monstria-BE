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

	if err := loadFlowerDefinitions(); err != nil {
		return err
	}

	err := initializer.RegisterRpc("ping", PingRPC)
	if err != nil {
		return err
	}

	if err := initializer.RegisterRpc("get_player_inventory", GetPlayerInventoryRPC); err != nil {
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
	if err := initializer.RegisterRpc("plant_seed_in_pot", PlantSeedInPotRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("water_plant_in_pot", WaterPlantInPotRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("get_flower_catalog", GetFlowerCatalogRPC); err != nil {
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
