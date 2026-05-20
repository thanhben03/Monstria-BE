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
	diseaseTickSeconds = int64(15)
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
	RiskPerTickBps    int
	Severity          int
	DamagePerTick     int
}

type TreatmentItemDefinition struct {
	ItemID            string
	ProtectionSeconds int64
}

var diseaseDefinitions = []DiseaseDefinition{
	{
		Type:              "borua",
		MinGrowthProgress: 25,
		MaxGrowthProgress: 85,
		RiskPerTickBps:    2500,
		Severity:          1,
		DamagePerTick:     4,
	},
	{
		Type:              "bocanhcung",
		MinGrowthProgress: 60,
		MaxGrowthProgress: 100,
		RiskPerTickBps:    1800,
		Severity:          2,
		DamagePerTick:     8,
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
	if plant.Disease != nil && !plant.DiseaseOccurred {
		plant.DiseaseOccurred = true
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

		elapsedTicks := (nowUnix - last) / diseaseTickSeconds
		if elapsedTicks < 1 {
			continue
		}

		def, err := flowerDefinitionForSeed(plant.SeedItemID)
		if err != nil {
			continue
		}

		for tick := int64(1); tick <= elapsedTicks; tick++ {
			at := last + tick*diseaseTickSeconds
			if plant.Disease != nil {
				plant.Health -= plant.Disease.Severity * diseaseDamagePerTick(plant.Disease.Type)
				if plant.Health < 0 {
					plant.Health = 0
				}
				continue
			}
			if plant.DiseaseProtectionUntil >= at {
				continue
			}
			if plant.DiseaseOccurred {
				continue
			}
			if disease, ok := rollDiseaseForPlant(def, at, plant, roll); ok {
				plant.Disease = &disease
				plant.DiseaseOccurred = true
			}
		}

		plant.LastCalculatedAt = last + elapsedTicks*diseaseTickSeconds
		changed = true
	}
	return changed
}

func rollDiseaseForPlant(def FlowerDefinition, at int64, plant *PotPlant, roll diseaseRoller) (PlantDisease, bool) {
	candidates := diseaseCandidatesForProgress(def, at, plant.GrowthStartedAt)
	if len(candidates) == 0 {
		return PlantDisease{}, false
	}
	riskPerTickBps := 0
	for _, candidate := range candidates {
		if candidate.RiskPerTickBps > riskPerTickBps {
			riskPerTickBps = candidate.RiskPerTickBps
		}
	}
	if riskPerTickBps < 1 || roll(percentScale) >= riskPerTickBps {
		return PlantDisease{}, false
	}

	candidate := candidates[roll(len(candidates))]
	return PlantDisease{
		Type:      candidate.Type,
		Severity:  candidate.Severity,
		StartedAt: at,
	}, true
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

	allowed := make(map[string]struct{}, len(def.Disease))
	for _, diseaseType := range def.Disease {
		diseaseType = strings.TrimSpace(diseaseType)
		if diseaseType != "" {
			allowed[diseaseType] = struct{}{}
		}
	}
	if len(allowed) == 0 {
		return nil
	}

	out := make([]DiseaseDefinition, 0, len(diseaseDefinitions))
	for _, def := range diseaseDefinitions {
		if def.Type == "" || def.Severity < 1 {
			continue
		}
		if _, ok := allowed[def.Type]; !ok {
			continue
		}
		if progress < def.MinGrowthProgress || progress > def.MaxGrowthProgress {
			continue
		}
		out = append(out, def)
	}
	return out
}

func diseaseDamagePerTick(diseaseType string) int {
	t := strings.TrimSpace(diseaseType)
	for _, def := range diseaseDefinitions {
		if def.Type == t && def.DamagePerTick > 0 {
			return def.DamagePerTick
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
