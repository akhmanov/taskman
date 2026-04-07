package model

import "sort"

func ProjectStateFromEvents(events []Event) ProjectionState {
	return projectionStateFromEvents(events)
}

func TaskStateFromEvents(events []Event) ProjectionState {
	return projectionStateFromEvents(events)
}

func projectionStateFromEvents(events []Event) ProjectionState {
	stateful := make([]Event, 0, len(events))
	for _, event := range events {
		if event.IsStateful() {
			stateful = append(stateful, event)
		}
	}
	state := ProjectionState{
		Status: StatusBacklog,
		Vars:   map[string]string{},
	}
	if len(stateful) == 0 {
		return state
	}
	byParent := map[string][]Event{}
	byID := map[string]Event{}
	for _, event := range stateful {
		byID[event.ID] = event
		byParent[event.ParentHeadID] = append(byParent[event.ParentHeadID], event)
	}
	for key := range byParent {
		sort.Slice(byParent[key], func(i, j int) bool {
			if byParent[key][i].At != byParent[key][j].At {
				return byParent[key][i].At < byParent[key][j].At
			}
			return byParent[key][i].ID < byParent[key][j].ID
		})
	}
	leaves := leafEvents(stateful)
	if len(leaves) == 1 {
		applyChain(&state, chainForLeaf(byID, leaves[0]))
		state.CurrentHeadID = leaves[0].ID
		return state
	}
	common := deepestCommonAncestor(byID, leaves)
	if common != "" {
		applyChain(&state, chainForLeaf(byID, byID[common]))
		state.CurrentHeadID = common
	}
	state.UnresolvedHead = make([]string, 0, len(leaves))
	for _, leaf := range leaves {
		state.UnresolvedHead = append(state.UnresolvedHead, leaf.ID)
	}
	return state
}

func leafEvents(events []Event) []Event {
	hasChild := map[string]bool{}
	byID := map[string]Event{}
	for _, event := range events {
		byID[event.ID] = event
		if event.ParentHeadID != "" {
			hasChild[event.ParentHeadID] = true
		}
	}
	leaves := []Event{}
	for _, event := range events {
		if !hasChild[event.ID] {
			leaves = append(leaves, event)
		}
	}
	sort.Slice(leaves, func(i, j int) bool {
		if leaves[i].At != leaves[j].At {
			return leaves[i].At < leaves[j].At
		}
		return leaves[i].ID < leaves[j].ID
	})
	return leaves
}

func chainForLeaf(byID map[string]Event, leaf Event) []Event {
	chain := []Event{leaf}
	current := leaf
	for current.ParentHeadID != "" {
		parent, ok := byID[current.ParentHeadID]
		if !ok {
			break
		}
		chain = append(chain, parent)
		current = parent
	}
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain
}

func deepestCommonAncestor(byID map[string]Event, leaves []Event) string {
	if len(leaves) == 0 {
		return ""
	}
	common := ancestryIndex(byID, leaves[0])
	for _, leaf := range leaves[1:] {
		next := ancestryIndex(byID, leaf)
		for id := range common {
			if _, ok := next[id]; !ok {
				delete(common, id)
			}
		}
	}
	bestID := ""
	bestDepth := -1
	for id, depth := range common {
		if depth > bestDepth {
			bestID = id
			bestDepth = depth
		}
	}
	return bestID
}

func ancestryIndex(byID map[string]Event, leaf Event) map[string]int {
	index := map[string]int{}
	depth := 0
	current := leaf
	for {
		index[current.ID] = depth
		if current.ParentHeadID == "" {
			break
		}
		parent, ok := byID[current.ParentHeadID]
		if !ok {
			break
		}
		current = parent
		depth++
	}
	return index
}

func applyChain(state *ProjectionState, chain []Event) {
	for _, event := range chain {
		switch event.Kind {
		case EventKindMetadataPatch:
			if event.MetadataPatch == nil {
				continue
			}
			if event.MetadataPatch.Labels != nil {
				state.Labels = NormalizeLabels(event.MetadataPatch.Labels)
			}
			if state.Vars == nil {
				state.Vars = map[string]string{}
			}
			for key, value := range event.MetadataPatch.VarsSet {
				state.Vars[key] = value
			}
			for _, key := range event.MetadataPatch.VarsUnset {
				delete(state.Vars, key)
			}
		case EventKindTransition:
			if event.Transition == nil {
				continue
			}
			state.Status = event.Transition.To
			state.StatusDetail = StatusDetail{
				ReasonType: event.Transition.ReasonType,
				Reason:     event.Transition.Reason,
				ResumeWhen: event.Transition.ResumeWhen,
				Summary:    event.Transition.Summary,
			}
			state.Warnings = append([]string{}, event.Transition.Warnings...)
		}
		state.UpdatedAt = event.At
	}
}
