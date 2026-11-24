package gfmt

import (
	"slices"

	"github.com/brawler2011/gate/backend/pkg/formats/spec"
	"github.com/brawler2011/gate/backend/pkg/points"
)

// GroupTest is a test with the group that holds it directly.
type GroupTest struct {
	Group *Group
	Test  Test
}

// Tests lists every test in ordinal order.
func (p *Problem) Tests() []GroupTest {
	var result []GroupTest
	for g := range p.Testset.All() {
		for _, t := range g.Tests {
			result = append(result, GroupTest{Group: g, Test: t})
		}
	}
	slices.SortFunc(result, func(a, b GroupTest) int { return a.Test.Ordinal - b.Test.Ordinal })
	return result
}

// GroupResult is a group's score and the most it can get; Passed means every test in it passed.
type GroupResult struct {
	Group  *Group
	Score  points.Value
	Max    points.Value
	Passed bool
}

// Grade scores the testset of a checked package; results list every group with tests, parents first.
func (p *Problem) Grade(passed func(ordinal int) bool) (points.Value, []GroupResult) {
	type item struct {
		first      int
		score, max points.Value
		passed     bool
	}
	byName := map[string]*Group{}
	complete := map[*Group]bool{}
	var check func(*Group) bool
	check = func(g *Group) bool {
		byName[g.Name] = g
		ok := true
		for _, t := range g.Tests {
			ok = passed(t.Ordinal) && ok
		}
		for i := range g.Groups {
			ok = check(&g.Groups[i]) && ok
		}
		complete[g] = ok
		return ok
	}
	check(&p.Testset)
	satisfied := map[*Group]bool{}
	var met func(*Group) bool
	met = func(g *Group) bool {
		if ok, seen := satisfied[g]; seen {
			return ok
		}
		ok := true
		for _, name := range g.Requires {
			required := byName[name]
			ok = ok && complete[required] && met(required)
		}
		satisfied[g] = ok
		return ok
	}
	var results []GroupResult
	var grade func(*Group) (item, bool)
	grade = func(g *Group) (item, bool) {
		var items []item
		for _, t := range g.Tests {
			ok := passed(t.Ordinal)
			items = append(items, item{first: t.Ordinal, score: map[bool]points.Value{true: t.Points}[ok], max: t.Points, passed: ok})
		}
		at := len(results)
		results = append(results, GroupResult{Group: g})
		for i := range g.Groups {
			if child, ok := grade(&g.Groups[i]); ok {
				items = append(items, child)
			}
		}
		if len(items) == 0 {
			results = slices.Delete(results, at, at+1)
			return item{}, false
		}
		slices.SortFunc(items, func(a, b item) int { return a.first - b.first })
		var scores, maxes []points.Value
		for _, it := range items {
			maxes = append(maxes, it.max)
		}
		for _, it := range items {
			scores = append(scores, it.score)
			if !it.passed && g.StopOnFailure {
				break
			}
		}
		result := item{first: items[0].first, score: spec.Aggregate(g.Aggregate, scores), max: spec.Aggregate(g.Aggregate, maxes), passed: complete[g]}
		// Leaving results out may raise a minimum or an average, but never above what passing everything gives.
		result.score = min(result.score, result.max)
		if !met(g) {
			result.score = 0
		}
		results[at] = GroupResult{Group: g, Score: result.score, Max: result.max, Passed: result.passed}
		return result, true
	}
	root, _ := grade(&p.Testset)
	return root.score, results
}
