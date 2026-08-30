package storage

import (
	"regexp"
	"testing"
)

func TestProductionWorldCatalogHasOneThousandUniqueMapsPerWorld(t *testing.T) {
	counts := make(map[string]int, len(cultivationWorldRegions))
	entries := make(map[string]int, len(cultivationWorldRegions))
	names := make(map[string]string, productionWorldLocationLimit)
	digits := regexp.MustCompile(`[0-9]`)
	for index := 1; index <= productionWorldLocationLimit; index++ {
		region, _, _ := worldLocationIdentity(index)
		name := generatedWorldLocationName(index)
		if priorRegion, exists := names[name]; exists {
			t.Fatalf("duplicate map name %q in %s and %s", name, priorRegion, region)
		}
		if digits.MatchString(name) {
			t.Fatalf("map name contains a numeric suffix: %s", name)
		}
		names[name] = region
		counts[region]++
		if isWorldEntryLocation(index) {
			entries[region]++
		}
	}
	if len(counts) != len(cultivationWorldRegions) {
		t.Fatalf("world count=%d want=%d", len(counts), len(cultivationWorldRegions))
	}
	for _, region := range cultivationWorldRegions {
		if counts[region] != 1000 {
			t.Fatalf("%s map count=%d want=1000", region, counts[region])
		}
		if entries[region] != 1 {
			t.Fatalf("%s entry count=%d want=1", region, entries[region])
		}
	}

	adjacency := make(map[string][]string, productionWorldLocationLimit)
	for index := 1; index <= productionWorldLocationLimit; index++ {
		name := generatedWorldLocationName(index)
		for _, neighbor := range generatedWorldNeighborNames(index, productionWorldLocationLimit) {
			if names[neighbor] != names[name] {
				t.Fatalf("cross-world walking edge: %s(%s) -> %s(%s)", name, names[name], neighbor, names[neighbor])
			}
			adjacency[name] = append(adjacency[name], neighbor)
		}
	}
	for regionIndex, region := range cultivationWorldRegions {
		start := generatedWorldLocationName(worldLocationGlobalIndex(regionIndex, 1))
		visited := map[string]bool{start: true}
		queue := []string{start}
		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			for _, next := range adjacency[current] {
				if !visited[next] {
					visited[next] = true
					queue = append(queue, next)
				}
			}
		}
		if len(visited) != 1000 {
			t.Fatalf("%s connected maps=%d want=1000", region, len(visited))
		}
	}
}
