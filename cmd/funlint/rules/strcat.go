//go:build gorules
// +build gorules

// Package gorules contains custom linting rules for the codebase.
// This file is used by go-ruleguard to enforce coding standards.
package gorules

import "github.com/quasilyte/go-ruleguard/dsl"

// stringConcatBinary detects simple binary string concatenation using the + operator.
// It suggests using utils.Concat for better performance and consistency.
func stringConcat(m dsl.Matcher) {
	// // Match binary string concatenation (a + b)
	// m.Match(`$a + $b`).
	// 	Where(m["a"].Type.Is("string") && m["b"].Type.Is("string")).
	// 	Report("prefer utils.Concat($a, $b) over string concatenation with + operator").
	// 	Suggest("utils.Concat($a, $b)")

	// // Match chained string concatenation (a + b + c)
	// m.Match(`$a + $b + $c`).
	// 	Where(m["a"].Type.Is("string") && m["b"].Type.Is("string") && m["c"].Type.Is("string")).
	// 	Report("prefer utils.Concat($a, $b, $c) over string concatenation with + operator").
	// 	Suggest("utils.Concat($a, $b, $c)")

	// // Match longer chains (a + b + c + d)
	// m.Match(`$a + $b + $c + $d`).
	// 	Where(m["a"].Type.Is("string") && m["b"].Type.Is("string") && m["c"].Type.Is("string") && m["d"].Type.Is("string")).
	// 	Report("prefer utils.Concat($a, $b, $c, $d) over string concatenation with + operator").
	// 	Suggest("utils.Concat($a, $b, $c, $d)")

	// // Match longer chains (a + b + c + d + e)
	// m.Match(`$a + $b + $c + $d + $e`).
	// 	Where(m["a"].Type.Is("string") && m["b"].Type.Is("string") && m["c"].Type.Is("string") && m["d"].Type.Is("string") && m["e"].Type.Is("string")).
	// 	Report("prefer utils.Concat($a, $b, $c, $d, $e) over string concatenation with + operator").
	// 	Suggest("utils.Concat($a, $b, $c, $d, $e)")

	m.Match(`$a += $b`).
		Where(m["a"].Type.Is("string")).
		Report("avoid += with strings; prefer strings.Builder, utils.Concat, or itertools.JoinStrings")

	// Detect += in for loops
	m.Match(`for $*_ { $*_; $s += $_; $*_ }`).
		Where(m["s"].Type.Is("string")).
		Report("avoid string concatenation in loops; prefer strings.Builder or itertools.JoinStrings")

	// Detect += in range loops
	// m.Match(`for $*_ := range $_ { $*_; $s += $_; $*_ }`).
	// 	Where(m["s"].Type.Is("string")).
	// 	Report("string concatenation in loop is inefficient; use strings.Builder or itertools.JoinStrings instead")
}
