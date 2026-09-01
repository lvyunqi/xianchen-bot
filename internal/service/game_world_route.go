package service

import (
	"fmt"
	"strings"

	"xianlv/internal/model"
)

type worldRoutePlanner struct {
	canonicalByAlias map[string]string
	adjacency        map[string][]string
}

func (g *Game) loadWorldRoutePlanner() (worldRoutePlanner, error) {
	var rows []model.WorldLocation
	if err := g.store.DB.Where("enabled = ?", true).Order("sort_order,id").Find(&rows).Error; err != nil {
		return worldRoutePlanner{}, err
	}
	planner := worldRoutePlanner{
		canonicalByAlias: make(map[string]string, len(rows)*2),
		adjacency:        make(map[string][]string, len(rows)),
	}
	for _, row := range rows {
		planner.canonicalByAlias[row.Name] = row.Name
		planner.canonicalByAlias[row.Code] = row.Name
	}
	for _, row := range rows {
		neighbors := decodeTextList(row.NeighborsJSON)
		for _, neighbor := range neighbors {
			if canonical := planner.canonicalByAlias[neighbor]; canonical != "" {
				planner.adjacency[row.Name] = append(planner.adjacency[row.Name], canonical)
			}
		}
	}
	return planner, nil
}

func (planner worldRoutePlanner) shortest(from, destination string) []string {
	from = planner.canonicalByAlias[strings.TrimSpace(from)]
	destination = planner.canonicalByAlias[strings.TrimSpace(destination)]
	if from == "" || destination == "" {
		return nil
	}
	if from == destination {
		return []string{from}
	}
	queue := []string{from}
	previous := map[string]string{from: ""}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, next := range planner.adjacency[current] {
			if _, visited := previous[next]; visited {
				continue
			}
			previous[next] = current
			if next == destination {
				return rebuildWorldRoute(previous, destination)
			}
			queue = append(queue, next)
		}
	}
	return nil
}

func rebuildWorldRoute(previous map[string]string, destination string) []string {
	reversed := []string{destination}
	for current := destination; previous[current] != ""; {
		current = previous[current]
		reversed = append(reversed, current)
	}
	route := make([]string, len(reversed))
	for index := range reversed {
		route[len(reversed)-1-index] = reversed[index]
	}
	return route
}

func worldRouteSummary(route []string) string {
	if len(route) == 0 {
		return "路线尚未贯通"
	}
	if len(route) == 1 {
		return "已在灵脉所在地图"
	}
	steps := len(route) - 1
	if len(route) <= 5 {
		return fmt.Sprintf("%s（%d步）", strings.Join(route, " → "), steps)
	}
	return fmt.Sprintf("%s → %s → … → %s（%d步）", route[0], route[1], route[len(route)-1], steps)
}

func worldRouteLocationMark(route []string) string {
	if len(route) == 1 {
		return "当前地图"
	}
	if len(route) > 1 {
		return fmt.Sprintf("距此%d步 · 下一站%s", len(route)-1, route[1])
	}
	return "路线未贯通"
}

func appendWorldRouteAction(actions []string, route []string) []string {
	if len(route) > 1 {
		return append(actions, "前往 "+route[1])
	}
	return actions
}
