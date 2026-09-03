package core

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// TopoOrder returns every loaded resource in dependency order: a resource
// always appears after everything it references. Ties break by kind then key so
// plan output is stable run to run.
func (l *Loader) TopoOrder() ([]string, error) {
	keys := slices.Collect(maps.Keys(l.sources))
	sortByKindThenKey(l.registry, keys, l.sources)

	deps := map[string][]string{}
	indegree := map[string]int{}
	dependents := map[string][]string{}
	for _, k := range keys {
		indegree[k] = 0
	}
	for _, k := range keys {
		for _, d := range l.dependencies(k) {
			if _, ok := l.sources[d]; !ok {
				continue
			}
			deps[k] = append(deps[k], d)
			indegree[k]++
			dependents[d] = append(dependents[d], k)
		}
	}

	var ready []string
	for _, k := range keys {
		if indegree[k] == 0 {
			ready = append(ready, k)
		}
	}

	var order []string
	for len(ready) > 0 {
		next := ready[0]
		ready = ready[1:]
		order = append(order, next)

		var freed []string
		for _, dependent := range dependents[next] {
			indegree[dependent]--
			if indegree[dependent] == 0 {
				freed = append(freed, dependent)
			}
		}
		if len(freed) > 0 {
			ready = append(ready, freed...)
			sortByKindThenKey(l.registry, ready, l.sources)
		}
	}

	if len(order) != len(keys) {
		var stuck []string
		for _, k := range keys {
			if indegree[k] > 0 {
				stuck = append(stuck, k)
			}
		}
		return nil, fmt.Errorf("reference cycle: %s", describeCycle(stuck, deps))
	}
	return order, nil
}

// dependencies returns the keys a resource references, deduplicated.
func (l *Loader) dependencies(key string) []string {
	seen := map[string]bool{}
	var out []string
	for _, slot := range l.slots[key] {
		for _, entry := range slot.Entries {
			if entry.TargetKey == "" || seen[entry.TargetKey] {
				continue
			}
			seen[entry.TargetKey] = true
			out = append(out, entry.TargetKey)
		}
	}
	slices.Sort(out)
	return out
}

// sortByKindThenKey orders keys by kind rank, dependencies first, then by key.
func sortByKindThenKey(r *Registry, keys []string, sources map[string]*Source) {
	slices.SortFunc(keys, func(a, b string) int {
		return cmp.Or(cmp.Compare(kindRank(r, sources, a), kindRank(r, sources, b)), strings.Compare(a, b))
	})
}

// kindRank is a resource's position in dependency order, by its kind.
func kindRank(r *Registry, sources map[string]*Source, key string) int {
	src, ok := sources[key]
	if !ok {
		return len(r.Kinds())
	}
	return r.rank(src.Kind)
}

// visitState is how far describeCycle's depth-first walk has got with a key.
type visitState int

const (
	visitUnseen visitState = iota
	visitOnStack
	visitDone
)

// describeCycle walks the stuck set to produce an actual cycle path rather
// than an unordered list, because "a → b → a" is the only form of this message
// anyone can act on.
func describeCycle(stuck []string, deps map[string][]string) string {
	inStuck := map[string]bool{}
	for _, k := range stuck {
		inStuck[k] = true
	}
	visited := map[string]visitState{}
	var path []string
	var cycle []string

	var walk func(string) bool
	walk = func(k string) bool {
		visited[k] = visitOnStack
		path = append(path, k)
		for _, d := range deps[k] {
			if !inStuck[d] {
				continue
			}
			if visited[d] == visitOnStack {
				start := slices.Index(path, d)
				cycle = append(append([]string{}, path[start:]...), d)
				return true
			}
			if visited[d] == visitUnseen && walk(d) {
				return true
			}
		}
		path = path[:len(path)-1]
		visited[k] = visitDone
		return false
	}
	for _, k := range stuck {
		if visited[k] == visitUnseen && walk(k) {
			break
		}
	}
	if len(cycle) == 0 {
		return strings.Join(stuck, ", ")
	}
	return strings.Join(cycle, " → ")
}
