//go:build ignore
// +build ignore

package gorules

import "github.com/quasilyte/go-ruleguard/dsl"

// convertTestifyToFunAssert converts testify/assert and testify/require to fun/assert
func convertTestifyRequireToFunAssert(m dsl.Matcher) {
	m.Import("github.com/stretchr/testify/require")
	m.Import("github.com/tychoish/fun/assert")

	// assert.NoError -> assert.NotError
	m.Match(`$pkg.NoError($t, $err)`, `$pkg.NoError($t, $err, $*args)`).
		Where(m["pkg"].Text == "require").
		Suggest(`assert.NotError($t, $err)`).
		Report(`Use assert.NotError from fun/assert instead of testify require.NoError`)

	m.Match(`$pkg.Error($t, $err)`, `$pkg.Error($t, $err, $*args)`).
		Where(m["pkg"].Text == "require").
		Suggest(`assert.Error($t, $err)`).
		Report(`Use assert.Error from fun/assert instead of testify require.Error`)

	m.Match(`$pkg.True($t, $cond)`, `$pkg.True($t, $cond, $*args)`).
		Where(m["pkg"].Text == "require").
		Suggest(`assert.True($t, $cond)`).
		Report(`Use assert.True from fun/assert instead of testify require.True`)

	m.Match(`$pkg.False($t, $cond)`, `$pkg.False($t, $cond, $*args)`).
		Where(m["pkg"].Text == "require").
		Suggest(`assert.True($t, !$cond)`).
		Report(`Use assert.True from fun/assert instead of testify require.False`)

	m.Match(`$pkg.Equal($t, $expected, $actual)`, `$pkg.Equal($t, $expected, $actual, $*args)`).
		Where(m["pkg"].Text == "require").
		Suggest(`assert.Equal($t, $expected, $actual)`).
		Report(`Use assert.Equal from fun/assert instead of testify require.Equal`)
	m.Match(`$pkg.NotEqual($t, $expected, $actual)`, `$pkg.NotEqual($t, $expected, $actual, $*args)`).
		Where(m["pkg"].Text == "require").
		Suggest(`assert.True($t, $expected != $actual)`).
		Report(`Use assert.True with != from fun/assert instead of testify require.NotEqual`)
	m.Match(`$pkg.Nil($t, $val)`, `$pkg.Nil($t, $val, $*args)`).
		Where(m["pkg"].Text == "require").
		Suggest(`assert.Zero($t, $val)`).
		Report(`Use assert.Zero from fun/assert instead of testify require.Nil`)
	m.Match(`$pkg.NotNil($t, $val)`, `$pkg.NotNil($t, $val, $*args)`).
		Where(m["pkg"].Text == "require").
		Suggest(`assert.True($t, $val != nil)`).
		Report(`Use assert.True with != nil from fun/assert instead of testify require.NotNil`)
	m.Match(`$pkg.Len($t, $obj, $length)`, `$pkg.Len($t, $obj, $length, $*args)`).
		Where(m["pkg"].Text == "require").
		Suggest(`assert.Equal($t, $length, len($obj))`).
		Report(`Use assert.Equal with len() from fun/assert instead of testify require.Len`)
	m.Match(`$pkg.Empty($t, $obj)`, `$pkg.Empty($t, $obj, $*args)`).
		Where(m["pkg"].Text == "require").
		Suggest(`assert.Zero($t, len($obj))`).
		Report(`Use assert.Zero with len() from fun/assert instead of testify require.Empty`)
	m.Match(`$pkg.NotEmpty($t, $obj)`, `$pkg.NotEmpty($t, $obj, $*args)`).
		Where(m["pkg"].Text == "require").
		Suggest(`assert.True($t, len($obj) > 0)`).
		Report(`Use assert.True with len() > 0 from fun/assert instead of testify require.NotEmpty`)
	m.Match(`$pkg.Contains($t, $container, $element)`, `$pkg.Contains($t, $container, $element, $*args)`).
		Where(m["pkg"].Text == "require").
		Report(`Manually convert Contains from testify to fun/assert/check - use appropriate check for container type`)

	m.Match(`$pkg.Panics($t, $fn)`, `$pkg.Panics($t, $fn, $*args)`).
		Where(m["pkg"].Text == "require").
		Suggest(`assert.Panic($t, $fn)`).
		Report(`Use assert.Panic from fun/assert instead of testify Panics`)

	m.Match(`$pkg.NotPanics($t, $fn)`, `$pkg.NotPanics($t, $fn, $*args)`).
		Where(m["pkg"].Text == "require").
		Suggest(`assert.NotPanic($t, $fn)`).
		Report(`Use check.NotPanic from fun/assert/check instead of testify NotPanics`)

	m.Match(`$pkg.ErrorIs($t, $err, $target)`, `$pkg.ErrorIs($t, $err, $target, $*args)`).
		Where(m["pkg"].Text == "require").
		Suggest(`assert.ErrorIs($t, $err, $target)`).
		Report(`Use assert.ErrorIs from fun/assert instead of testify ErrorIs`)
	m.Match(`$pkg.Zero($t, $val)`, `$pkg.Zero($t, $val, $*args)`).
		Where(m["pkg"].Text == "require").
		Suggest(`assert.Zero($t, $val)`).
		Report(`Use assert.Zero from fun/assert instead of testify require.Zero`)

	m.Match(`$pkg.Greater($t, $a, $b)`, `$pkg.Greater($t, $a, $b, $*args)`).
		Where(m["pkg"].Text == "require").
		Suggest(`assert.True($t, $a > $b)`).
		Report(`Use assert.True with > from fun/assert instead of testify require.Greater`)
	m.Match(`$pkg.GreaterOrEqual($t, $a, $b)`, `$pkg.GreaterOrEqual($t, $a, $b, $*args)`).
		Where(m["pkg"].Text == "require").
		Suggest(`assert.True($t, $a >= $b)`).
		Report(`Use assert.True with >= from fun/assert instead of testify require.GreaterOrEqual`)
	m.Match(`$pkg.Less($t, $a, $b)`, `$pkg.Less($t, $a, $b, $*args)`).
		Where(m["pkg"].Text == "require").
		Suggest(`assert.True($t, $a < $b)`).
		Report(`Use assert.True with < from fun/assert instead of testify require.Less`)
	m.Match(`$pkg.LessOrEqual($t, $a, $b)`, `$pkg.LessOrEqual($t, $a, $b, $*args)`).
		Where(m["pkg"].Text == "require").
		Suggest(`assert.True($t, $a <= $b)`).
		Report(`Use assert.True with <= from fun/assert instead of testify require.LessOrEqual`)
	m.Match(`$pkg.Positive($t, $val)`, `$pkg.Positive($t, $val, $*args)`).
		Where(m["pkg"].Text == "require").
		Suggest(`assert.True($t, $val > 0)`).
		Report(`Use assert.True with > 0 from fun/assert instead of testify require.Positive`)
	m.Match(`$pkg.Negative($t, $val)`, `$pkg.Negative($t, $val, $*args)`).
		Where(m["pkg"].Text == "require").
		Suggest(`assert.True($t, $val < 0)`).
		Report(`Use assert.True with < 0 from fun/assert instead of testify require.Negative`)
	// Methods that require manual conversion - testify/require
	m.Match(`$pkg.NotZero($*_)`, `$pkg.NotContains($*_)`, `$pkg.WithinDuration($*_)`,
		`$pkg.InDelta($*_)`, `$pkg.InEpsilon($*_)`, `$pkg.IsType($*_)`,
		`$pkg.Implements($*_)`, `$pkg.Same($*_)`, `$pkg.NotSame($*_)`,
		`$pkg.Subset($*_)`, `$pkg.NotSubset($*_)`, `$pkg.ElementsMatch($*_)`,
		`$pkg.Eventually($*_)`, `$pkg.Never($*_)`, `$pkg.JSONEq($*_)`,
		`$pkg.YAMLEq($*_)`, `$pkg.FileExists($*_)`, `$pkg.DirExists($*_)`,
		`$pkg.Regexp($*_)`, `$pkg.NotRegexp($*_)`, `$pkg.PanicsWithValue($*_)`,
		`$pkg.PanicsWithError($*_)`, `$pkg.ErrorContains($*_)`, `$pkg.ErrorAs($*_)`,
		`$pkg.Condition($*_)`, `$pkg.InDeltaSlice($*_)`, `$pkg.InDeltaMapValues($*_)`,
		`$pkg.NoFileExists($*_)`, `$pkg.NoDirExists($*_)`).
		Where(m["pkg"].Text == "require").
		Report(`This testify assertion requires manual conversion to fun/assert - check documentation`)

	m.Match(`$pkg.HTTPSuccess($*_)`, `$pkg.HTTPRedirect($*_)`, `$pkg.HTTPError($*_)`,
		`$pkg.HTTPStatusCode($*_)`, `$pkg.HTTPBody($*_)`, `$pkg.HTTPBodyContains($*_)`,
		`$pkg.HTTPBodyNotContains($*_)`).
		Where(m["pkg"].Text == "require").
		Report(`HTTP assertions are not supported in fun/assert - implement custom HTTP checks`)

	m.Match(`"github.com/stretchr/testify/assert"`).
		Suggest("github.com/tychoish/fun/assert/check").
		Report("update import statement, for testify.assert, to fun/assert/check")

	m.Match(`"github.com/stretchr/testify/require"`).
		Suggest("github.com/tychoish/fun/assert").
		Report("update import statement, for testify.require, to fun/assert/check")
}

func convertTestifyAssertToFunCheck(m dsl.Matcher) {
	m.Import("github.com/stretchr/testify/assert")
	m.Import("github.com/tychoish/fun/assert/check")

	m.Match(`$pkg.NoError($t, $err)`, `$pkg.NoError($t, $err, $*args)`).
		Where(m["pkg"].Text == "assert").
		Suggest(`check.NotError($t, $err)`).
		Report(`Use assert.NotError from fun/assert instead of testify assert.NoError`)

	// assert.Error -> assert.Error
	m.Match(`$pkg.Error($t, $err)`, `$pkg.Error($t, $err, $*args)`).
		Where(m["pkg"].Text == "assert").
		Suggest(`check.Error($t, $err)`).
		Report(`Use assert.Error from fun/assert instead of testify assert.Error`)

	// assert.True -> assert.True
	m.Match(`$pkg.True($t, $cond)`, `$pkg.True($t, $cond, $*args)`).
		Where(m["pkg"].Text == "assert").
		Suggest(`check.True($t, $cond)`).
		Report(`Use assert.True from fun/assert instead of testify assert.True`)

	// assert.False -> assert.True(!cond)
	m.Match(`$pkg.False($t, $cond)`, `$pkg.False($t, $cond, $*args)`).
		Where(m["pkg"].Text == "assert").
		Suggest(`check.True($t, !$cond)`).
		Report(`Use assert.True from fun/assert instead of testify assert.False`)

	// assert.Equal -> assert.Equal
	m.Match(`$pkg.Equal($t, $expected, $actual)`, `$pkg.Equal($t, $expected, $actual, $*args)`).
		Where(m["pkg"].Text == "assert").
		Suggest(`check.Equal($t, $expected, $actual)`).
		Report(`Use assert.Equal from fun/assert instead of testify assert.Equal`)

	// assert.NotEqual -> use negation with assert.True
	m.Match(`$pkg.NotEqual($t, $expected, $actual)`, `$pkg.NotEqual($t, $expected, $actual, $*args)`).
		Where(m["pkg"].Text == "assert").
		Suggest(`check.True($t, $expected != $actual)`).
		Report(`Use assert.True with != from fun/assert instead of testify assert.NotEqual`)

	// assert.Nil -> assert.Zero
	m.Match(`$pkg.Nil($t, $val)`, `$pkg.Nil($t, $val, $*args)`).
		Where(m["pkg"].Text == "assert").
		Suggest(`check.Zero($t, $val)`).
		Report(`Use assert.Zero from fun/assert instead of testify assert.Nil`)

	// assert.NotNil -> use negation with assert.True
	m.Match(`$pkg.NotNil($t, $val)`, `$pkg.NotNil($t, $val, $*args)`).
		Where(m["pkg"].Text == "assert").
		Suggest(`check.True($t, $val != nil)`).
		Report(`Use assert.True with != nil from fun/assert instead of testify assert.NotNil`)

	// assert.Len -> use assert.Equal with len()
	m.Match(`$pkg.Len($t, $obj, $length)`, `$pkg.Len($t, $obj, $length, $*args)`).
		Where(m["pkg"].Text == "assert").
		Suggest(`check.Equal($t, $length, len($obj))`).
		Report(`Use assert.Equal with len() from fun/assert instead of testify assert.Len`)

	// assert.Empty -> use assert.Zero with len()
	m.Match(`$pkg.Empty($t, $obj)`, `$pkg.Empty($t, $obj, $*args)`).
		Where(m["pkg"].Text == "assert").
		Suggest(`check.Zero($t, len($obj))`).
		Report(`Use assert.Zero with len() from fun/assert instead of testify assert.Empty`)

	// assert.NotEmpty -> use assert.True with len() > 0
	m.Match(`$pkg.NotEmpty($t, $obj)`, `$pkg.NotEmpty($t, $obj, $*args)`).
		Where(m["pkg"].Text == "assert").
		Suggest(`check.True($t, len($obj) > 0)`).
		Report(`Use assert.True with len() > 0 from fun/assert instead of testify assert.NotEmpty`)

	// assert.Contains -> report manual conversion needed
	m.Match(`$pkg.Contains($t, $container, $element)`, `$pkg.Contains($t, $container, $element, $*args)`).
		Where(m["pkg"].Text == "assert").
		Report(`Manually convert Contains from testify to fun/assert - use appropriate check for container type`)

	// assert.Panics -> assert.Panic
	m.Match(`$pkg.Panics($t, $fn)`, `$pkg.Panics($t, $fn, $*args)`).
		Where(m["pkg"].Text == "assert").
		Suggest(`check.Panic($t, $fn)`).
		Report(`Use assert.Panic from fun/assert instead of testify Panics`)
	// assert.NotPanics -> check.NotPanic
	m.Match(`$pkg.NotPanics($t, $fn)`, `$pkg.NotPanics($t, $fn, $*args)`).
		Where(m["pkg"].Text == "assert").
		Suggest(`check.NotPanic($t, $fn)`).
		Report(`Use check.NotPanic from fun/assert/check instead of testify NotPanics`)
	// assert.ErrorIs -> assert.ErrorIs
	m.Match(`$pkg.ErrorIs($t, $err, $target)`, `$pkg.ErrorIs($t, $err, $target, $*args)`).
		Where(m["pkg"].Text == "assert").
		Suggest(`check.ErrorIs($t, $err, $target)`).
		Report(`Use assert.ErrorIs from fun/assert instead of testify ErrorIs`)

	// assert.Zero -> check.Zero
	m.Match(`$pkg.Zero($t, $val)`, `$pkg.Zero($t, $val, $*args)`).
		Where(m["pkg"].Text == "assert").
		Suggest(`check.Zero($t, $val)`).
		Report(`Use check.Zero from fun/assert/check instead of testify assert.Zero`)

	// Comparison operations
	m.Match(`$pkg.Greater($t, $a, $b)`, `$pkg.Greater($t, $a, $b, $*args)`).
		Where(m["pkg"].Text == "assert").
		Suggest(`check.True($t, $a > $b)`).
		Report(`Use check.True with > from fun/assert/check instead of testify assert.Greater`)

	m.Match(`$pkg.GreaterOrEqual($t, $a, $b)`, `$pkg.GreaterOrEqual($t, $a, $b, $*args)`).
		Where(m["pkg"].Text == "assert").
		Suggest(`check.True($t, $a >= $b)`).
		Report(`Use check.True with >= from fun/assert/check instead of testify assert.GreaterOrEqual`)

	m.Match(`$pkg.Less($t, $a, $b)`, `$pkg.Less($t, $a, $b, $*args)`).
		Where(m["pkg"].Text == "assert").
		Suggest(`check.True($t, $a < $b)`).
		Report(`Use check.True with < from fun/assert/check instead of testify assert.Less`)

	m.Match(`$pkg.LessOrEqual($t, $a, $b)`, `$pkg.LessOrEqual($t, $a, $b, $*args)`).
		Where(m["pkg"].Text == "assert").
		Suggest(`check.True($t, $a <= $b)`).
		Report(`Use check.True with <= from fun/assert/check instead of testify assert.LessOrEqual`)

	m.Match(`$pkg.Positive($t, $val)`, `$pkg.Positive($t, $val, $*args)`).
		Where(m["pkg"].Text == "assert").
		Suggest(`check.True($t, $val > 0)`).
		Report(`Use check.True with > 0 from fun/assert/check instead of testify assert.Positive`)

	m.Match(`$pkg.Negative($t, $val)`, `$pkg.Negative($t, $val, $*args)`).
		Where(m["pkg"].Text == "assert").
		Suggest(`check.True($t, $val < 0)`).
		Report(`Use check.True with < 0 from fun/assert/check instead of testify assert.Negative`)

	// Methods that require manual conversion - testify/assert
	m.Match(`$pkg.NotZero($*_)`, `$pkg.NotContains($*_)`, `$pkg.WithinDuration($*_)`,
		`$pkg.InDelta($*_)`, `$pkg.InEpsilon($*_)`, `$pkg.IsType($*_)`,
		`$pkg.Implements($*_)`, `$pkg.Same($*_)`, `$pkg.NotSame($*_)`,
		`$pkg.Subset($*_)`, `$pkg.NotSubset($*_)`, `$pkg.ElementsMatch($*_)`,
		`$pkg.Eventually($*_)`, `$pkg.Never($*_)`, `$pkg.JSONEq($*_)`,
		`$pkg.YAMLEq($*_)`, `$pkg.FileExists($*_)`, `$pkg.DirExists($*_)`,
		`$pkg.Regexp($*_)`, `$pkg.NotRegexp($*_)`, `$pkg.PanicsWithValue($*_)`,
		`$pkg.PanicsWithError($*_)`, `$pkg.ErrorContains($*_)`, `$pkg.ErrorAs($*_)`,
		`$pkg.Condition($*_)`, `$pkg.InDeltaSlice($*_)`, `$pkg.InDeltaMapValues($*_)`,
		`$pkg.NoFileExists($*_)`, `$pkg.NoDirExists($*_)`).
		Where(m["pkg"].Text == "assert").
		Report(`This testify assertion requires manual conversion to fun/assert/check - check documentation`)

	// HTTP assertions - testify/assert
	m.Match(`$pkg.HTTPSuccess($*_)`, `$pkg.HTTPRedirect($*_)`, `$pkg.HTTPError($*_)`,
		`$pkg.HTTPStatusCode($*_)`, `$pkg.HTTPBody($*_)`, `$pkg.HTTPBodyContains($*_)`,
		`$pkg.HTTPBodyNotContains($*_)`).
		Where(m["pkg"].Text == "assert").
		Report(`HTTP assertions are not supported in fun/assert - implement custom HTTP checks`)
}

func convertTestifyAssertToFunCheck(m dsl.Matcher) {
	m.Import("github.com/stretchr/testify/assert")
	m.Import("github.com/tychoish/fun/assert/check")

	m.Match(`$pkg.NoError($t, $err)`, `$pkg.NoError($t, $err, $*args)`).
		Where(m["pkg"].Text == "assert").
		Suggest(`check.NotError($t, $err)`).
		Report(`Use assert.NotError from fun/assert instead of testify assert.NoError`)

	// assert.Error -> assert.Error
	m.Match(`$pkg.Error($t, $err)`, `$pkg.Error($t, $err, $*args)`).
		Where(m["pkg"].Text == "assert").
		Suggest(`check.Error($t, $err)`).
		Report(`Use assert.Error from fun/assert instead of testify assert.Error`)

	// assert.True -> assert.True
	m.Match(`$pkg.True($t, $cond)`, `$pkg.True($t, $cond, $*args)`).
		Where(m["pkg"].Text == "assert").
		Suggest(`check.True($t, $cond)`).
		Report(`Use assert.True from fun/assert instead of testify assert.True`)

	// assert.False -> assert.True(!cond)
	m.Match(`$pkg.False($t, $cond)`, `$pkg.False($t, $cond, $*args)`).
		Where(m["pkg"].Text == "assert").
		Suggest(`check.True($t, !$cond)`).
		Report(`Use assert.True from fun/assert instead of testify assert.False`)

	// assert.Equal -> assert.Equal
	m.Match(`$pkg.Equal($t, $expected, $actual)`, `$pkg.Equal($t, $expected, $actual, $*args)`).
		Where(m["pkg"].Text == "assert").
		Suggest(`check.Equal($t, $expected, $actual)`).
		Report(`Use assert.Equal from fun/assert instead of testify assert.Equal`)

	// assert.NotEqual -> use negation with assert.True
	m.Match(`$pkg.NotEqual($t, $expected, $actual)`, `$pkg.NotEqual($t, $expected, $actual, $*args)`).
		Where(m["pkg"].Text == "assert").
		Suggest(`check.True($t, $expected != $actual)`).
		Report(`Use assert.True with != from fun/assert instead of testify assert.NotEqual`)

	// assert.Nil -> assert.Zero
	m.Match(`$pkg.Nil($t, $val)`, `$pkg.Nil($t, $val, $*args)`).
		Where(m["pkg"].Text == "assert").
		Suggest(`check.Zero($t, $val)`).
		Report(`Use assert.Zero from fun/assert instead of testify assert.Nil`)

	// assert.NotNil -> use negation with assert.True
	m.Match(`$pkg.NotNil($t, $val)`, `$pkg.NotNil($t, $val, $*args)`).
		Where(m["pkg"].Text == "assert").
		Suggest(`check.True($t, $val != nil)`).
		Report(`Use assert.True with != nil from fun/assert instead of testify assert.NotNil`)

	// assert.Len -> use assert.Equal with len()
	m.Match(`$pkg.Len($t, $obj, $length)`, `$pkg.Len($t, $obj, $length, $*args)`).
		Where(m["pkg"].Text == "assert").
		Suggest(`check.Equal($t, $length, len($obj))`).
		Report(`Use assert.Equal with len() from fun/assert instead of testify assert.Len`)

	// assert.Empty -> use assert.Zero with len()
	m.Match(`$pkg.Empty($t, $obj)`, `$pkg.Empty($t, $obj, $*args)`).
		Where(m["pkg"].Text == "assert").
		Suggest(`check.Zero($t, len($obj))`).
		Report(`Use assert.Zero with len() from fun/assert instead of testify assert.Empty`)

	// assert.NotEmpty -> use assert.True with len() > 0
	m.Match(`$pkg.NotEmpty($t, $obj)`, `$pkg.NotEmpty($t, $obj, $*args)`).
		Where(m["pkg"].Text == "assert").
		Suggest(`check.True($t, len($obj) > 0)`).
		Report(`Use assert.True with len() > 0 from fun/assert instead of testify assert.NotEmpty`)

	// assert.Contains -> report manual conversion needed
	m.Match(`$pkg.Contains($t, $container, $element)`, `$pkg.Contains($t, $container, $element, $*args)`).
		Where(m["pkg"].Text == "assert").
		Report(`Manually convert Contains from testify to fun/assert - use appropriate check for container type`)

	// assert.Panics -> assert.Panic
	m.Match(`$pkg.Panics($t, $fn)`, `$pkg.Panics($t, $fn, $*args)`).
		Where(m["pkg"].Text == "assert").
		Suggest(`check.Panic($t, $fn)`).
		Report(`Use assert.Panic from fun/assert instead of testify Panics`)
	// assert.NotPanics -> check.NotPanic
	m.Match(`$pkg.NotPanics($t, $fn)`, `$pkg.NotPanics($t, $fn, $*args)`).
		Where(m["pkg"].Text == "assert").
		Suggest(`check.NotPanic($t, $fn)`).
		Report(`Use check.NotPanic from fun/assert/check instead of testify NotPanics`)
	// assert.ErrorIs -> assert.ErrorIs
	m.Match(`$pkg.ErrorIs($t, $err, $target)`, `$pkg.ErrorIs($t, $err, $target, $*args)`).
		Where(m["pkg"].Text == "assert").
		Suggest(`check.ErrorIs($t, $err, $target)`).
		Report(`Use assert.ErrorIs from fun/assert instead of testify ErrorIs`)

	// assert.Zero -> check.Zero
	m.Match(`$pkg.Zero($t, $val)`, `$pkg.Zero($t, $val, $*args)`).
		Where(m["pkg"].Text == "assert").
		Suggest(`check.Zero($t, $val)`).
		Report(`Use check.Zero from fun/assert/check instead of testify assert.Zero`)

	// Comparison operations
	m.Match(`$pkg.Greater($t, $a, $b)`, `$pkg.Greater($t, $a, $b, $*args)`).
		Where(m["pkg"].Text == "assert").
		Suggest(`check.True($t, $a > $b)`).
		Report(`Use check.True with > from fun/assert/check instead of testify assert.Greater`)

	m.Match(`$pkg.GreaterOrEqual($t, $a, $b)`, `$pkg.GreaterOrEqual($t, $a, $b, $*args)`).
		Where(m["pkg"].Text == "assert").
		Suggest(`check.True($t, $a >= $b)`).
		Report(`Use check.True with >= from fun/assert/check instead of testify assert.GreaterOrEqual`)

	m.Match(`$pkg.Less($t, $a, $b)`, `$pkg.Less($t, $a, $b, $*args)`).
		Where(m["pkg"].Text == "assert").
		Suggest(`check.True($t, $a < $b)`).
		Report(`Use check.True with < from fun/assert/check instead of testify assert.Less`)

	m.Match(`$pkg.LessOrEqual($t, $a, $b)`, `$pkg.LessOrEqual($t, $a, $b, $*args)`).
		Where(m["pkg"].Text == "assert").
		Suggest(`check.True($t, $a <= $b)`).
		Report(`Use check.True with <= from fun/assert/check instead of testify assert.LessOrEqual`)

	m.Match(`$pkg.Positive($t, $val)`, `$pkg.Positive($t, $val, $*args)`).
		Where(m["pkg"].Text == "assert").
		Suggest(`check.True($t, $val > 0)`).
		Report(`Use check.True with > 0 from fun/assert/check instead of testify assert.Positive`)

	m.Match(`$pkg.Negative($t, $val)`, `$pkg.Negative($t, $val, $*args)`).
		Where(m["pkg"].Text == "assert").
		Suggest(`check.True($t, $val < 0)`).
		Report(`Use check.True with < 0 from fun/assert/check instead of testify assert.Negative`)

	// Methods that require manual conversion - testify/assert
	m.Match(`$pkg.NotZero($*_)`, `$pkg.NotContains($*_)`, `$pkg.WithinDuration($*_)`,
		`$pkg.InDelta($*_)`, `$pkg.InEpsilon($*_)`, `$pkg.IsType($*_)`,
		`$pkg.Implements($*_)`, `$pkg.Same($*_)`, `$pkg.NotSame($*_)`,
		`$pkg.Subset($*_)`, `$pkg.NotSubset($*_)`, `$pkg.ElementsMatch($*_)`,
		`$pkg.Eventually($*_)`, `$pkg.Never($*_)`, `$pkg.JSONEq($*_)`,
		`$pkg.YAMLEq($*_)`, `$pkg.FileExists($*_)`, `$pkg.DirExists($*_)`,
		`$pkg.Regexp($*_)`, `$pkg.NotRegexp($*_)`, `$pkg.PanicsWithValue($*_)`,
		`$pkg.PanicsWithError($*_)`, `$pkg.ErrorContains($*_)`, `$pkg.ErrorAs($*_)`,
		`$pkg.Condition($*_)`, `$pkg.InDeltaSlice($*_)`, `$pkg.InDeltaMapValues($*_)`,
		`$pkg.NoFileExists($*_)`, `$pkg.NoDirExists($*_)`).
		Where(m["pkg"].Text == "assert").
		Report(`This testify assertion requires manual conversion to fun/assert/check - check documentation`)

	// HTTP assertions - testify/assert
	m.Match(`$pkg.HTTPSuccess($*_)`, `$pkg.HTTPRedirect($*_)`, `$pkg.HTTPError($*_)`,
		`$pkg.HTTPStatusCode($*_)`, `$pkg.HTTPBody($*_)`, `$pkg.HTTPBodyContains($*_)`,
		`$pkg.HTTPBodyNotContains($*_)`).
		Where(m["pkg"].Text == "assert").
		Report(`HTTP assertions are not supported in fun/assert - implement custom HTTP checks`)
}
