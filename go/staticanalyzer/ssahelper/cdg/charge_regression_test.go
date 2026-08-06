package cdg

// charge_regression_test.go — Regression tests for CDG path explosion.
//
// These tests reproduce the exponential path growth observed in the Charge
// function from service-0001 (example.internal/part1/service-0001). That function
// has 240+ basic blocks with deeply nested conditionals and loops, causing
// inverse CDG path enumeration to blow up without a path limit.
//
// Two test strategies are used:
//   1. Synthetic CFG from hardcoded edges (TestChargeCFG_CDGPathGrowth):
//      Uses the exact CFG edge structure extracted from the real function.
//      Fast and deterministic — no parsing or compilation needed.
//   2. Rewritten source (TestChargeCFG_FromRewrittenSource):
//      Reads the actual Charge function source from testdata/, rewrites
//      it via the AST rewriter (cfg_rewriter_test.go), compiles to SSA,
//      and builds CDG from the real CFG. End-to-end validation.

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/ssa"
)

// chargeCFGEdges encodes the CFG of the service-0001 Charge function.
// Each entry is: {from, []to}. This CFG causes exponential path growth
// in inverse CDG enumeration.
var chargeCFGEdges = []struct {
	from  int
	succs []int
}{
	{0, []int{1, 2}},
	{1, []int{3, 4}},
	{2, []int{20, 19}},
	{3, nil},
	{4, []int{5, 6}},
	{5, []int{6}},
	{6, []int{7, 8}},
	{7, []int{8}},
	{8, []int{10, 11}},
	{9, []int{12, 13}},
	{10, []int{15, 17}},
	{11, []int{9, 10}},
	{12, nil},
	{13, []int{14, 10}},
	{14, []int{10}},
	{15, []int{16}},
	{16, []int{2}},
	{17, []int{16}},
	{18, []int{21, 19}},
	{19, []int{22, 23}},
	{20, []int{18, 19}},
	{21, nil},
	{22, []int{23}},
	{23, []int{24, 25}},
	{24, []int{26, 27}},
	{25, []int{47, 48}},
	{26, nil},
	{27, []int{28, 29}},
	{28, nil},
	{29, []int{30, 33}},
	{30, nil},
	{31, []int{38, 35}},
	{32, []int{30, 31}},
	{33, []int{30, 32}},
	{34, []int{39, 35}},
	{35, []int{42, 41}},
	{36, []int{35, 34}},
	{37, []int{36, 35}},
	{38, []int{37, 35}},
	{39, nil},
	{40, []int{41}},
	{41, []int{43, 44}},
	{42, []int{40, 41}},
	{43, nil},
	{44, []int{45, 46}},
	{45, nil},
	{46, []int{25}},
	{47, []int{49, 50}},
	{48, []int{53, 54}},
	{49, nil},
	{50, []int{51, 52}},
	{51, nil},
	{52, nil},
	{53, []int{55, 56}},
	{54, []int{60, 59}},
	{55, []int{54, 57}},
	{56, nil},
	{57, []int{54}},
	{58, []int{61, 62}},
	{59, []int{73, 74}},
	{60, []int{59, 58}},
	{61, nil},
	{62, []int{63, 64}},
	{63, []int{65, 66}},
	{64, []int{72, 59}},
	{65, nil},
	{66, []int{67, 68}},
	{67, []int{69, 64}},
	{68, []int{70, 64}},
	{69, []int{64}},
	{70, []int{64}},
	{71, []int{59}},
	{72, []int{71, 59}},
	{73, nil},
	{74, []int{75, 76}},
	{75, []int{77, 79}},
	{76, []int{92, 93}},
	{77, []int{80, 81}},
	{78, []int{91, 90}},
	{79, []int{88, 78}},
	{80, []int{81}},
	{81, []int{82, 83}},
	{82, []int{83}},
	{83, []int{84, 85}},
	{84, []int{85}},
	{85, []int{86, 78}},
	{86, []int{87, 78}},
	{87, nil},
	{88, nil},
	{89, []int{90}},
	{90, nil},
	{91, []int{89, 90}},
	{92, []int{94, 95}},
	{93, []int{96, 97}},
	{94, nil},
	{95, []int{93}},
	{96, []int{98, 99}},
	{97, []int{100, 101}},
	{98, nil},
	{99, []int{97}},
	{100, nil},
	{101, []int{105, 104}},
	{102, []int{106, 107}},
	{103, []int{109, 110}},
	{104, []int{108, 103}},
	{105, []int{102, 104}},
	{106, nil},
	{107, []int{103}},
	{108, nil},
	{109, []int{111, 112}},
	{110, []int{113, 114}},
	{111, nil},
	{112, []int{110}},
	{113, nil},
	{114, []int{115, 116}},
	{115, nil},
	{116, []int{117, 118}},
	{117, []int{119, 120}},
	{118, []int{127, 126}},
	{119, nil},
	{120, []int{121, 118}},
	{121, []int{122, 123}},
	{122, nil},
	{123, []int{124, 118}},
	{124, []int{118}},
	{125, nil},
	{126, []int{128, 129}},
	{127, []int{126, 125}},
	{128, nil},
	{129, []int{130, 131}},
	{130, []int{132, 131}},
	{131, []int{133, 134}},
	{132, []int{131}},
	{133, []int{134}},
	{134, []int{135, 136}},
	{135, []int{137, 138}},
	{136, []int{158, 157}},
	{137, []int{139, 138}},
	{138, []int{142, 141}},
	{139, nil},
	{140, []int{144, 141}},
	{141, []int{145, 146}},
	{142, []int{140, 141}},
	{143, nil},
	{144, []int{143, 141}},
	{145, nil},
	{146, []int{149, 136}},
	{147, []int{150}},
	{148, []int{147, 136}},
	{149, []int{148, 136}},
	{150, []int{151, 152}},
	{151, []int{153, 154}},
	{152, []int{136}},
	{153, nil},
	{154, []int{150}},
	{155, []int{159, 160}},
	{156, []int{172, 170}},
	{157, []int{162, 164}},
	{158, []int{155, 157}},
	{159, []int{160}},
	{160, []int{161, 156}},
	{161, []int{156}},
	{162, []int{165, 166}},
	{163, []int{167, 156}},
	{164, []int{162, 163}},
	{165, nil},
	{166, []int{163}},
	{167, []int{168, 169}},
	{168, nil},
	{169, []int{156}},
	{170, []int{173, 171}},
	{171, []int{183, 184}},
	{172, []int{174, 175}},
	{173, nil},
	{174, []int{175}},
	{175, []int{178, 177}},
	{176, []int{177}},
	{177, []int{180, 171}},
	{178, []int{176, 177}},
	{179, []int{182, 171}},
	{180, []int{179, 181}},
	{181, []int{179, 171}},
	{182, []int{171}},
	{183, []int{185, 186}},
	{184, []int{192, 190}},
	{185, []int{186}},
	{186, []int{187, 188}},
	{187, nil},
	{188, []int{184}},
	{189, []int{190}},
	{190, []int{195, 194}},
	{191, []int{189, 190}},
	{192, []int{191, 190}},
	{193, []int{194}},
	{194, []int{196, 197}},
	{195, []int{193, 194}},
	{196, []int{197}},
	{197, []int{200, 199}},
	{198, []int{201, 203}},
	{199, []int{210, 212}},
	{200, []int{198, 199}},
	{201, []int{202}},
	{202, []int{204, 205}},
	{203, []int{202}},
	{204, []int{206, 199}},
	{205, []int{207, 208}},
	{206, nil},
	{207, []int{199}},
	{208, []int{209, 199}},
	{209, nil},
	{210, []int{213, 214}},
	{211, []int{216, 217}},
	{212, []int{210, 211}},
	{213, []int{214}},
	{214, []int{215, 211}},
	{215, []int{211}},
	{216, []int{217}},
	{217, []int{218, 219}},
	{218, []int{219}},
	{219, []int{220, 221}},
	{220, nil},
	{221, []int{222, 224}},
	{222, []int{225, 226}},
	{223, []int{232, 231}},
	{224, []int{228, 223}},
	{225, []int{226}},
	{226, []int{227, 223}},
	{227, nil},
	{228, nil},
	{229, []int{230}},
	{230, []int{233, 234}},
	{231, []int{229, 230}},
	{232, []int{229, 231}},
	{233, []int{237, 234}},
	{234, []int{245, 246}},
	{235, []int{238, 240}},
	{236, []int{235, 234}},
	{237, []int{236, 234}},
	{238, []int{244, 243}},
	{239, []int{234}},
	{240, []int{241, 239}},
	{241, []int{238, 239}},
	{242, nil},
	{243, []int{234}},
	{244, []int{242, 243}},
	{245, []int{246}},
	{246, nil},
}

// TestChargeCFG_CDGPathGrowth is a regression test that verifies the CDG path
// limit prevents exponential blowup on the service-0001 Charge function's CFG.
//
// The test builds a synthetic CFG from the hardcoded edge table above,
// constructs the CDG, then scans all blocks to find the one with the most
// paths. It asserts that the limit is actually hit (confirming exponential
// growth exists) and that increasing the limit produces more paths.
func TestChargeCFG_CDGPathGrowth(t *testing.T) {
	// Derive block count from the edge table rather than hardcoding.
	numBlocks := 0
	for _, edge := range chargeCFGEdges {
		if edge.from+1 > numBlocks {
			numBlocks = edge.from + 1
		}
		for _, to := range edge.succs {
			if to+1 > numBlocks {
				numBlocks = to + 1
			}
		}
	}

	f, b := newFunctionWithBlocks(numBlocks)
	for _, edge := range chargeCFGEdges {
		for _, to := range edge.succs {
			connect(b[edge.from], b[to])
		}
	}

	// Build CDG.
	builder := NewCDGBuilder()
	forwardCDG := builder.BuildCDG(f)
	inverseCDG := NewInverseCDG(forwardCDG)

	// Scan all blocks to find the one with the most CDG paths. Use a moderate
	// limit for the scan to avoid blowing up across all blocks.
	const scanLimit = 10_000
	var deepBlock *ssa.BasicBlock
	maxPaths := 0
	for _, block := range b {
		paths, _ := inverseCDG.GetControllerPathsWithLimit(block, scanLimit)
		if len(paths) > maxPaths {
			maxPaths = len(paths)
			deepBlock = block
		}
	}
	require.NotNil(t, deepBlock, "should find at least one block with CDG paths")
	t.Logf("Deepest block: block %d with %d paths (scan limit=%d)", deepBlock.Index, maxPaths, scanLimit)

	// The deepest block must hit the scan limit, confirming exponential growth.
	require.GreaterOrEqual(t, maxPaths, scanLimit,
		"deepest block should hit the scan limit, confirming exponential path growth")

	// Verify that path counts grow with limit — this is the key property.
	const smallLimit = 1000
	const largeLimit = 5000
	pathsSmall, limitHitSmall := inverseCDG.GetControllerPathsWithLimit(deepBlock, smallLimit)
	pathsLarge, limitHitLarge := inverseCDG.GetControllerPathsWithLimit(deepBlock, largeLimit)

	require.True(t, limitHitSmall, "block %d should hit the %d path limit", deepBlock.Index, smallLimit)
	require.True(t, limitHitLarge, "block %d should hit the %d path limit", deepBlock.Index, largeLimit)
	require.GreaterOrEqual(t, len(pathsSmall), smallLimit)
	require.GreaterOrEqual(t, len(pathsLarge), largeLimit)

	t.Logf("Confirmed exponential growth: limit=%d → %d paths, limit=%d → %d paths",
		smallLimit, len(pathsSmall), largeLimit, len(pathsLarge))

	// Verify that an early/leaf block does NOT hit the limit.
	earlyBlock := b[3] // leaf block (no successors)
	earlyPaths, earlyLimitHit := inverseCDG.GetControllerPathsWithLimit(earlyBlock, scanLimit)
	require.False(t, earlyLimitHit, "early leaf block should not hit the path limit")
	t.Logf("Early block 3: %d paths (limit not hit)", len(earlyPaths))
}

// TestChargeCFG_FromRewrittenSource is an end-to-end regression test.
// It reads the actual Charge function source from testdata/charge_func.go,
// rewrites it via the AST rewriter, compiles to SSA, builds CDG, and
// verifies that exponential path growth is present and limited.
func TestChargeCFG_FromRewrittenSource(t *testing.T) {
	fn := buildCFGFromGoSource(t, "testdata/charge_func.go", "Charge")
	t.Logf("Rewritten Charge function has %d blocks", len(fn.Blocks))
	require.Greater(t, len(fn.Blocks), 100, "expected 100+ blocks from Charge function")

	// Build CDG.
	cdgBuilder := NewCDGBuilder()
	forwardCDG := cdgBuilder.BuildCDG(fn)
	inverseCDG := NewInverseCDG(forwardCDG)

	// Scan all blocks to find the one with the most CDG paths.
	const scanLimit = 10_000
	var deepBlock *ssa.BasicBlock
	maxPaths := 0
	for _, block := range fn.Blocks {
		paths, _ := inverseCDG.GetControllerPathsWithLimit(block, scanLimit)
		if len(paths) > maxPaths {
			maxPaths = len(paths)
			deepBlock = block
		}
	}
	require.NotNil(t, deepBlock)
	t.Logf("Deepest block: block %d with %d paths (scan limit=%d)", deepBlock.Index, maxPaths, scanLimit)

	// The deepest block must hit the scan limit.
	require.GreaterOrEqual(t, maxPaths, scanLimit,
		"deepest block should hit the scan limit, confirming exponential path growth")

	// Verify exponential growth by checking two different limits.
	const smallLimit = 1000
	const largeLimit = 5000
	pathsSmall, limitHitSmall := inverseCDG.GetControllerPathsWithLimit(deepBlock, smallLimit)
	pathsLarge, limitHitLarge := inverseCDG.GetControllerPathsWithLimit(deepBlock, largeLimit)

	require.True(t, limitHitSmall, "block %d should hit the %d path limit", deepBlock.Index, smallLimit)
	require.True(t, limitHitLarge, "block %d should hit the %d path limit", deepBlock.Index, largeLimit)
	require.GreaterOrEqual(t, len(pathsSmall), smallLimit)
	require.GreaterOrEqual(t, len(pathsLarge), largeLimit)

	// Verify memory stays bounded with the limit.
	runtime.GC()
	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	paths, _ := inverseCDG.GetControllerPathsWithLimit(deepBlock, 10_000)

	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)
	allocMB := float64(memAfter.TotalAlloc-memBefore.TotalAlloc) / (1024 * 1024)

	t.Logf("limit=10000  block=%d  paths=%d  alloc=%.1f MB",
		deepBlock.Index, len(paths), allocMB)
}
