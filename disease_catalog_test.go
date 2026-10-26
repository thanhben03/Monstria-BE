package main

import (
	"context"
	"strings"
	"testing"
)

func TestDiseaseCatalogMismatchPreventsSpawn(t *testing.T) {
	def := FlowerDefinition{SeedItemID: "seed_tuyetduong", GrowSeconds: 60, Disease: []string{"borua"}}
	plant := &PotPlant{GrowthStartedAt: 100}
	if _, ok := rollDiseaseForPlant(def, 130, plant, func(int) int { return 0 }); ok {
		t.Fatal("an undefined species must not become a different species")
	}
	if err := validateFlowerDiseaseReferences([]FlowerDefinition{def}); err == nil || !strings.Contains(err.Error(), "borua") {
		t.Fatalf("missing actionable validation: %v", err)
	}
	def.Disease = []string{"buom"}
	if err := validateFlowerDiseaseReferences([]FlowerDefinition{def}); err != nil {
		t.Fatal(err)
	}
	if disease, ok := rollDiseaseForPlant(def, 130, plant, func(int) int { return 0 }); !ok || disease.Type != "buom" {
		t.Fatalf("valid catalog did not spawn butterfly: %+v, %v", disease, ok)
	}
}

func TestMasterDataRejectsUndefinedDisease(t *testing.T) {
	fixture := masterDataFixture(t)
	fixture.objects[0].Value = strings.ReplaceAll(fixture.objects[0].Value, `"buom"`, `"undefined_species"`)
	if _, err := reloadFixture(t, context.Background(), fixture); err == nil || !strings.Contains(err.Error(), "undefined_species") {
		t.Fatalf("expected rejection before persistence: %v", err)
	}
}
