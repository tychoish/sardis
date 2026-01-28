// This is a ruleguard rules file for the fun package.
// See https://github.com/quasilyte/go-ruleguard

//go:build ruleguard
// +build ruleguard

package gorules

import "github.com/quasilyte/go-ruleguard/dsl"

// unnecessaryOrderedMapCopy detects cases where dt.OrderedMap or adt.OrderedMap
// are used but their content is immediately copied to a standard map that
// is only used locally within the function, suggesting the standard map
// should be used directly instead.
func unnecessaryOrderedMapCopy(m dsl.Matcher) {
	// Pattern: Collect2 helper to convert OrderedMap to standard map
	m.Match(
		`$m := irt.Collect2($orderedMap.Iterator())`,
	).
		Report("unnecessary ordered map: using Collect2 to convert to standard map").
		Suggest("consider using map directly instead of OrderedMap if insertion order is not needed")

	// Pattern: Manual iteration to copy to map
	m.Match(
		`$result := make(map[$k]$v); for $key, $val := range $orderedMap.Iterator() { $result[$key] = $val }`,
	).
		Report("unnecessary ordered map: content copied to standard map $result").
		Suggest("consider using map[$k]$v directly if insertion order is not needed")
}

// inefficientMapIteration detects inefficient patterns when iterating over maps.
func inefficientMapIteration(m dsl.Matcher) {
	// Using range over Keys() then Get() instead of Iterator()
	m.Match(
		`for $k := range $m.Keys() { $*_; $v := $m.Get($k); $*_ }`,
	).
		Report("inefficient map iteration: use Iterator() instead of Keys() + Get()").
		Suggest("for $k, $v := range $m.Iterator() { ... }")

	// Collecting all values when only need to check existence
	m.Match(
		`$values := irt.Collect($m.Values()); if len($values) > 0 { $*_ }`,
		`$values := irt.Collect($m.Values()); if len($values) == 0 { $*_ }`,
	).
		Report("inefficient: collecting all values just to check if map is non-empty").
		Suggest("use $m.Len() > 0 or $m.Len() == 0 instead")
}

// mapLenCheck detects inefficient length checks.
func mapLenCheck(m dsl.Matcher) {
	// Collecting keys or values just to check length
	m.Match(
		`len(irt.Collect($m.Keys()))`,
		`len(irt.Collect($m.Values()))`,
	).
		Report("inefficient: collecting all items just to check map length").
		Suggest("use $m.Len() instead")
}

// inefficientMapPopulation detects inefficient ways to populate maps using Set/Store in loops.
func inefficientMapPopulation(m dsl.Matcher) {
	// Using Set in a loop for OrderedMap (suggests Extend)
	m.Match(
		`for $k, $v := range $src { $dst.Set($k, $v) }`,
		`for $k, $v := range $src { $dst.Store($k, $v) }`,
	).
		Report("consider using Extend() method for bulk insertions from iterators").
		Suggest("$dst.Extend($src)")
}

// heapWithoutComparator detects Heap initialization without comparator.
func heapWithoutComparator(m dsl.Matcher) {
	m.Match(
		`$h := &dt.Heap[$t]{}; $h.Push($x)`,
		`$h := dt.Heap[$t]{}; $h.Push($x)`,
	).
		Report("Heap.Push will panic without CF comparator function").
		Suggest("initialize Heap with CF: &dt.Heap[$t]{CF: cmp.Compare[$t]}")
}

// ringCapacityNotSet detects Ring without capacity.
func ringCapacityNotSet(m dsl.Matcher) {
	m.Match(
		`$r := &dt.Ring[$t]{}; $r.Push($x)`,
		`$r := dt.Ring[$t]{}; $r.Push($x)`,
	).
		Report("Ring capacity not set: will not store any elements").
		Suggest("initialize Ring with Capacity: &dt.Ring[$t]{Capacity: $n}")
}

// doubleCheck detects redundant existence checks.
func doubleCheck(m dsl.Matcher) {
	m.Match(
		`if $m.Check($key) { $val := $m.Get($key); $*_ }`,
	).
		Report("redundant Check before Get: use Load() for single lookup").
		Suggest("if $val, ok := $m.Load($key); ok { ... }")
}
