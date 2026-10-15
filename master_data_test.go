package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"sync"
	"testing"

	"github.com/heroiclabs/nakama-common/api"
	"github.com/heroiclabs/nakama-common/runtime"
)

type masterDataStorageStub struct {
	runtime.NakamaModule
	objects []*api.StorageObject
	err     error
}

func (s masterDataStorageStub) StorageRead(_ context.Context, _ []*runtime.StorageRead) ([]*api.StorageObject, error) {
	return s.objects, s.err
}

func masterDataFixture(t *testing.T) masterDataStorageStub {
	t.Helper()
	s := masterDataStorageStub{}
	for _, key := range masterDataKeys {
		raw, err := os.ReadFile(key + ".json")
		if err != nil {
			t.Fatal(err)
		}
		s.objects = append(s.objects, &api.StorageObject{Collection: "catalog", Key: key, Value: string(raw)})
	}
	return s
}

func TestMasterDataReloadAuthorization(t *testing.T) {
	for _, allowlist := range []string{"", "admin-one,admin-two"} {
		t.Setenv("MASTER_DATA_ADMIN_USER_IDS", allowlist)
		for _, id := range []string{"player", "admin-two"} {
			if err := authorizeMasterDataReload(context.WithValue(context.Background(), runtime.RUNTIME_CTX_USER_ID, id)); err != nil {
				t.Fatalf("authenticated user %q rejected: %v", id, err)
			}
		}
	}
	if err := authorizeMasterDataReload(context.WithValue(context.Background(), runtime.RUNTIME_CTX_USER_ID, "")); err == nil {
		t.Fatal("empty user ID accepted")
	}
	// Unauthorized requests must be rejected before any storage call.
	if _, err := ReloadMasterDataRPC(context.Background(), nil, nil, nil, "{}"); err == nil {
		t.Fatal("anonymous reload accepted")
	}
}

func TestMasterDataReloadPublicationAndFailure(t *testing.T) {
	fixture := masterDataFixture(t)
	ctx := context.Background()
	t.Cleanup(func() {
		if _, err := reloadMasterData(ctx, fixture); err != nil {
			t.Error(err)
		}
	})
	result, err := reloadMasterData(ctx, fixture)
	if err != nil || len(result.Version) != 64 || result.Counts["pets"] == 0 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	changed := masterDataFixture(t)
	changed.objects[3].Value = `{"pets":[{"id":"pet_reload_test","name":"Test","description":"Reloaded","rarity":"R"}]}`
	next, err := reloadMasterData(ctx, changed)
	if err != nil || next.Version == result.Version || !isPetItemID("pet_reload_test") {
		t.Fatalf("reload not applied: %+v %v", next, err)
	}
	oldFlowers := listFlowerDefinitions()
	oldShop := listShopItemDefinitions()
	oldExp := expToNextLevel(1)
	for _, kind := range []string{"invalid", "missing", "storage_error"} {
		bad := masterDataFixture(t)
		// Earlier catalogs differ, so a sequential partial publish is detectable.
		candidateFlowers, parseErr := parseFlowerDefinitions([]byte(bad.objects[0].Value))
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		candidateFlowers[0].GrowSeconds++
		raw, marshalErr := json.Marshal(flowerCatalogConfig{Flowers: candidateFlowers})
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		bad.objects[0].Value = string(raw)
		switch kind {
		case "invalid":
			bad.objects[3].Value = `{"pets":[]}`
		case "missing":
			bad.objects = bad.objects[:3]
		case "storage_error":
			bad.err = errors.New("unavailable")
		}
		if _, err := reloadMasterData(ctx, bad); err == nil {
			t.Fatalf("accepted %s", kind)
		}
		if !isPetItemID("pet_reload_test") || !reflect.DeepEqual(oldFlowers, listFlowerDefinitions()) || !reflect.DeepEqual(oldShop, listShopItemDefinitions()) || oldExp != expToNextLevel(1) {
			t.Fatalf("%s changed active cache", kind)
		}
	}
}

func TestMasterDataConcurrentReadersAndReload(t *testing.T) {
	fixture := masterDataFixture(t)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				listFlowerDefinitions()
				flowerDefinitionForSeed("seed_rose")
				listShopItemDefinitions()
				listShopItemDefinitionsForCategory("seed")
				shopItemDefinitionForID("seed_rose")
				levelRewardsForLevel(1)
				expToNextLevel(1)
				isPetItemID("pet_monkey")
				GetPetCatalogRPC(context.WithValue(context.Background(), runtime.RUNTIME_CTX_USER_ID, "reader"), nil, nil, nil, "")
			}
		}()
	}
	for i := 0; i < 20; i++ {
		if _, err := reloadMasterData(context.Background(), fixture); err != nil {
			t.Error(err)
		}
	}
	wg.Wait()
}
