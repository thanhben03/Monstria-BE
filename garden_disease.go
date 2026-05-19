package main

import (
	"math/rand"
	"strings"

	"github.com/heroiclabs/nakama-common/runtime"
)

const (
	defaultPlantHealth = 100
	percentScale       = 10000
	secondsPerHour     = int64(3600)
)

// PlantDisease is backend-owned state. Client should render this, not decide it.
type PlantDisease struct {
	Type      string `json:"type"`
	Severity  int    `json:"severity"`
	StartedAt int64  `json:"startedAt"`
}

type DiseaseDefinition struct {
	Type              string
	MinGrowthProgress int
	MaxGrowthProgress int
	RiskPerHourBps    int
	Severity          int
	DamagePerHour     int
}

type TreatmentItemDefinition struct {
	ItemID            string
	ProtectionSeconds int64
}

var diseaseDefinitions = []DiseaseDefinition{
	{
		Type:              "leaf_spot",
		MinGrowthProgress: 25,
		MaxGrowthProgress: 85,
		RiskPerHourBps:    700,
		Severity:          1,
		DamagePerHour:     4,
	},
	{
		Type:              "stem_borer",
		MinGrowthProgress: 60,
		MaxGrowthProgress: 100,
		RiskPerHourBps:    450,
		Severity:          2,
		DamagePerHour:     8,
	},
}

var treatmentItemDefinitions = map[string]TreatmentItemDefinition{
	"item_pesticide": {
		ItemID:            "item_pesticide",
		ProtectionSeconds: 12 * secondsPerHour,
	},
}

type diseaseRoller func(maxExclusive int) int

func randomDiseaseRoller(maxExclusive int) int {
	return rand.Intn(maxExclusive)
}

func normalizePlantRuntimeState(plant *PotPlant, nowUnix int64) bool {
	changed := false
	if plant.Health == 0 && plant.LastCalculatedAt < 1 {
		plant.Health = defaultPlantHealth
		changed = true
	}
	if plant.Health < 0 {
		plant.Health = 0
		changed = true
	}
	if plant.Health > defaultPlantHealth {
		plant.Health = defaultPlantHealth
		changed = true
	}
	if plant.LastCalculatedAt < 1 {
		plant.LastCalculatedAt = plant.PlantedAt
		changed = true
	}
	if plant.LastCalculatedAt > nowUnix {
		plant.LastCalculatedAt = nowUnix
		changed = true
	}
	if plant.DiseaseProtectionUntil < nowUnix && plant.DiseaseProtectionUntil != 0 {
		plant.DiseaseProtectionUntil = 0
		changed = true
	}
	return changed
}

func updateGardenDiseaseState(g *PlayerGarden, nowUnix int64) bool {
	return updateGardenDiseaseStateWithRoller(g, nowUnix, randomDiseaseRoller)
}

func updateGardenDiseaseStateWithRoller(g *PlayerGarden, nowUnix int64, roll diseaseRoller) bool {
	if nowUnix < 1 {
		return false
	}

	changed := false
	for i := range g.Placements {
		plant := g.Placements[i].Plant
		if plant == nil {
			continue
		}
		if normalizePlantRuntimeState(plant, nowUnix) {
			changed = true
		}
		if plant.GrowthStartedAt < 1 || plant.Health < 1 {
			continue
		}

		last := plant.LastCalculatedAt
		if last < plant.GrowthStartedAt {
			last = plant.GrowthStartedAt
		}
		if nowUnix <= last {
			continue
		}

		elapsedHours := (nowUnix - last) / secondsPerHour
		if elapsedHours < 1 {
			continue
		}

		def, err := flowerDefinitionForSeed(plant.SeedItemID)
		if err != nil {
			continue
		}

		for h := int64(1); h <= elapsedHours; h++ {
			at := last + h*secondsPerHour
			if plant.Disease != nil {
				plant.Health -= plant.Disease.Severity * diseaseDamagePerHour(plant.Disease.Type)
				if plant.Health < 0 {
					plant.Health = 0
				}
				continue
			}
			if plant.DiseaseProtectionUntil >= at {
				continue
			}
			if disease, ok := rollDiseaseForPlant(def, at, plant, roll); ok {
				plant.Disease = &disease
			}
		}

		plant.LastCalculatedAt = last + elapsedHours*secondsPerHour
		changed = true
	}
	return changed
}

func rollDiseaseForPlant(def FlowerDefinition, at int64, plant *PotPlant, roll diseaseRoller) (PlantDisease, bool) {
	candidates := diseaseCandidatesForProgress(def, at, plant.GrowthStartedAt)
	if len(candidates) == 0 {
		return PlantDisease{}, false
	}
	for _, candidate := range candidates {
		if candidate.RiskPerHourBps < 1 {
			continue
		}
		if roll(percentScale) < candidate.RiskPerHourBps {
			return PlantDisease{
				Type:      candidate.Type,
				Severity:  candidate.Severity,
				StartedAt: at,
			}, true
		}
	}
	return PlantDisease{}, false
}

func diseaseCandidatesForProgress(def FlowerDefinition, at int64, growthStartedAt int64) []DiseaseDefinition {
	if def.GrowSeconds < 1 || growthStartedAt < 1 {
		return nil
	}
	progress := int((at - growthStartedAt) * 100 / def.GrowSeconds)
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}

	out := make([]DiseaseDefinition, 0, len(diseaseDefinitions))
	for _, def := range diseaseDefinitions {
		if def.Type == "" || def.Severity < 1 {
			continue
		}
		if progress < def.MinGrowthProgress || progress > def.MaxGrowthProgress {
			continue
		}
		out = append(out, def)
	}
	return out
}

func diseaseDamagePerHour(diseaseType string) int {
	t := strings.TrimSpace(diseaseType)
	for _, def := range diseaseDefinitions {
		if def.Type == t && def.DamagePerHour > 0 {
			return def.DamagePerHour
		}
	}
	return 1
}

func gardenTreatPlantDisease(g *PlayerGarden, slotID string, itemID string, nowUnix int64) error {
	sid := strings.TrimSpace(slotID)
	if sid == "" {
		return runtime.NewError("slotId is required", 3)
	}
	id := strings.TrimSpace(itemID)
	if id == "" {
		return runtime.NewError("itemId is required", 3)
	}
	treatment, ok := treatmentItemDefinitions[id]
	if !ok {
		return runtime.NewError("item cannot treat plant disease", 3)
	}

	idx := findPlacementIndex(g, sid)
	if idx < 0 || strings.TrimSpace(g.Placements[idx].PotItemID) == "" {
		return runtime.NewError("no pot in this slot", 3)
	}
	if g.Placements[idx].Plant == nil {
		return runtime.NewError("no plant in this slot", 3)
	}

	plant := g.Placements[idx].Plant
	normalizePlantRuntimeState(plant, nowUnix)
	plant.Disease = nil
	plant.DiseaseProtectionUntil = nowUnix + treatment.ProtectionSeconds
	plant.LastCalculatedAt = nowUnix
	g.Placements = normalizeGardenPlacements(g.Placements)
	return nil
}
