package app

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"BYR-Client/internal/domain"
	"BYR-Client/internal/forum"
	"BYR-Client/internal/ports"
)

// LoadTraffic reads traffic for every board below a directory node. A section
// detail page carries all of its boards, so boards are grouped by their nearest
// section ancestor and fetched once per section.
func (a *App) LoadTraffic(ctx context.Context, node forum.NodeID) error {
	if err := a.requireSession(); err != nil {
		return err
	}
	a.mu.Lock()
	nav := a.nav
	boardNodes := make(map[domain.BoardID]forum.NodeID, len(a.boardNodes))
	for board, nodeID := range a.boardNodes {
		boardNodes[board] = nodeID
	}
	a.trafficLoad[node] = true
	a.trafficErrors[node] = ""
	a.mu.Unlock()
	a.changed()

	if nav == nil {
		return a.finishTraffic(node, nil, fmt.Errorf("目录尚未初始化"))
	}
	boards, err := nav.Boards(node, forum.BFS)
	if err != nil {
		return a.finishTraffic(node, nil, fmt.Errorf("解析流量范围: %w", err))
	}
	source, ok := a.source.(ports.TrafficSource)
	if !ok {
		return a.finishTraffic(node, nil, fmt.Errorf("当前数据源不支持流量统计"))
	}

	groups := make(map[string][]domain.BoardID)
	missing := make([]domain.BoardID, 0)
	for _, board := range boards {
		nodeID, exists := boardNodes[board]
		if !exists {
			missing = append(missing, board)
			continue
		}
		sectionID, ok := sectionIDFor(nav, nodeID)
		if !ok {
			missing = append(missing, board)
			continue
		}
		groups[sectionID] = append(groups[sectionID], board)
	}

	type result struct {
		values map[domain.BoardID]domain.Traffic
		err    error
	}
	results := make(chan result, len(groups))
	sem := make(chan struct{}, 5)
	var wg sync.WaitGroup
	for sectionID, sectionBoards := range groups {
		wg.Add(1)
		go func(sectionID string, sectionBoards []domain.BoardID) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				results <- result{err: ctx.Err()}
				return
			}
			defer func() { <-sem }()
			values, err := source.ReadSectionTraffic(ctx, domain.Scope{SiteID: "byr"}, sectionID, sectionBoards)
			results <- result{values: values, err: err}
		}(sectionID, sectionBoards)
	}
	wg.Wait()
	close(results)

	values := make(map[domain.BoardID]domain.Traffic)
	errorsSeen := make([]string, 0)
	for item := range results {
		if item.err != nil {
			errorsSeen = append(errorsSeen, item.err.Error())
			continue
		}
		for board, traffic := range item.values {
			if traffic.Known {
				values[board] = traffic
			}
		}
	}
	if len(missing) > 0 {
		errorsSeen = append(errorsSeen, fmt.Sprintf("%d 个版块缺少分区标识", len(missing)))
	}
	var finalErr error
	if len(errorsSeen) > 0 {
		finalErr = fmt.Errorf("%s", strings.Join(errorsSeen, "；"))
	}
	return a.finishTraffic(node, values, finalErr)
}

func (a *App) finishTraffic(node forum.NodeID, values map[domain.BoardID]domain.Traffic, err error) error {
	a.mu.Lock()
	for board, traffic := range values {
		a.traffic[board] = traffic
		a.trafficAt[board] = traffic.ObservedAt
	}
	a.trafficLoad[node] = false
	if err != nil {
		a.trafficErrors[node] = err.Error()
	} else {
		a.trafficErrors[node] = ""
	}
	a.mu.Unlock()
	a.changed()
	return err
}

func (a *App) BoardTraffic(board domain.BoardID) (domain.Traffic, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	traffic, ok := a.traffic[board]
	return traffic, ok && traffic.Known
}

func (a *App) TrafficForNode(node forum.NodeID) (domain.Traffic, bool) {
	a.mu.RLock()
	nav := a.nav
	a.mu.RUnlock()
	if nav == nil {
		return domain.Traffic{}, false
	}
	boards, err := nav.Boards(node, forum.BFS)
	if err != nil || len(boards) == 0 {
		return domain.Traffic{}, false
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	aggregate := domain.Traffic{}
	for _, board := range boards {
		traffic, ok := a.traffic[board]
		if !ok || !traffic.Known {
			return domain.Traffic{}, false
		}
		aggregate = aggregate.Add(traffic)
	}
	return aggregate, aggregate.Known
}

func (a *App) TrafficLoading(node forum.NodeID) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.trafficLoad[node]
}

func (a *App) TrafficError(node forum.NodeID) string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.trafficErrors[node]
}

func sectionIDFor(nav *forum.Navigation, boardNode forum.NodeID) (string, bool) {
	path, err := nav.Breadcrumb(boardNode)
	if err != nil {
		return "", false
	}
	for index := len(path) - 1; index >= 0; index-- {
		node := path[index]
		if node.Kind != forum.KindForum && node.Kind != forum.KindSubForum {
			continue
		}
		value := string(node.ID)
		value = strings.TrimPrefix(value, "sec:")
		value = strings.TrimPrefix(value, "sec-")
		if value == "" || strings.HasPrefix(value, "forum-board:") || strings.HasPrefix(value, "board:") {
			return "", false
		}
		return value, true
	}
	return "", false
}
