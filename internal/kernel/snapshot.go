package kernel

import "sort"

// ScopeInfo describes one open scope.
type ScopeInfo struct {
	Name     string      `json:"name"`
	Effects  []string    `json:"effects,omitempty"`
	Services []string    `json:"services,omitempty"`
	Children []ScopeInfo `json:"children,omitempty"`
}

// UnitInfo describes one supervised unit.
type UnitInfo struct {
	Name    string `json:"name"`
	State   string `json:"state"`
	Cause   string `json:"cause,omitempty"`
	Starts  int    `json:"starts"`
	Crashes int    `json:"crashes_in_window"`
}

// Snapshot is the kernel's whole observable state.
type Snapshot struct {
	Scope ScopeInfo  `json:"scope"`
	Units []UnitInfo `json:"units"`
}

// Snapshot captures open scopes (with pending effect names, LIFO order, and
// services) and unit states. Two snapshots of an idle kernel are equal.
func (k *Kernel) Snapshot() Snapshot {
	snap := Snapshot{Scope: k.root.info(), Units: []UnitInfo{}}
	k.sup.mu.Lock()
	for _, u := range k.sup.units {
		snap.Units = append(snap.Units, UnitInfo{Name: u.name, State: u.state, Cause: u.cause,
			Starts: u.starts, Crashes: len(u.crashes)})
	}
	k.sup.mu.Unlock()
	sort.Slice(snap.Units, func(i, j int) bool { return snap.Units[i].Name < snap.Units[j].Name })
	return snap
}

func (s *Scope) info() ScopeInfo {
	s.mu.Lock()
	in := ScopeInfo{Name: s.name}
	for i := len(s.effects) - 1; i >= 0; i-- {
		in.Effects = append(in.Effects, s.effects[i].name)
	}
	for n := range s.services {
		in.Services = append(in.Services, n)
	}
	children := append([]*Scope(nil), s.children...)
	s.mu.Unlock()
	sort.Strings(in.Services)
	for _, c := range children {
		in.Children = append(in.Children, c.info())
	}
	return in
}
