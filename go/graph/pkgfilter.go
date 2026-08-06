package graph

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/tools/go/ssa"
)

// DefaultExcludePackagePatterns is the default value of Config.ExcludePackagePatterns, used
// whenever that field is left nil. It covers common open-source RPC/tracing/metrics/DI/storage
// libraries that typically appear as framework plumbing between an inbound handler and the
// business logic rpcshield's fail-close analysis cares about, regardless of which organization's
// codebase is being analyzed.
//
// A company-internal fork with its own additional framework/glue packages to exclude (e.g.
// a monorepo fork, via config.Config.ExtraExcludePackagePatterns) should extend this list rather
// than replace it -- see Config.ExcludePackagePatterns's doc comment. Since this variable is
// exported, treat changes to its contents as a compatibility-affecting change, not an
// implementation detail.
//
// Standard-library packages are not listed here: every top-level directory under GOROOT/src
// (other than "sync"-prefixed ones) is excluded automatically regardless of this list -- see
// stdlibExclusionRegexes.
var DefaultExcludePackagePatterns = []string{
	"^go.uber.org/yarpc(/.*)?$",
	"^github.com/yarpc/yarpc-go(/.*)?$",
	"^github.com/uber/tchannel-go(/.*)?$",
	"^go.uber.org/zap(/.*)?$",
	"^go.uber.org/fx(/.*)?$",
	"^go.uber.org/multierr(/.*)?$",
	"^github.com/uber-go/tally(/.*)?$",
	"^github.com/opentracing(/.*)?$",
	"^github.com/uber/jaeger-client-go(/.*)?$",
	"^github.com/jaegertracing(/.*)?$",
	"^go.opentelemetry.io(/.*)?$",
	"^github.com/apache/thrift(/.*)?$",
	"^github.com/go-redis/redis(/.*)?$",
	"^github.com/mattn/go-sqlite3(/.*)?$",
	"^github.com/spiffe/go-spiffe(/.*)?$",
}

/*
For reference: here is a list of SDK packages we get when we list all files in GOROOT/src/.
milind-F5PJHX6JKQ:pprof_parser milind$ ls /usr/local/go/src/
Make.dist	arena		clean.bash	compress	embed		go		index		make.bat	os		regexp		sort		testing
README.vendor	bootstrap.bash	clean.bat	container	encoding	go.mod		internal	make.rc		path		run.bash	strconv		text
all.bash	bufio		clean.rc	context		errors		go.sum		io		maps		plugin		run.bat		strings		time
all.bat		buildall.bash	cmd		crypto		expvar		hash		iter		math		race.bash	run.rc		sync		unicode
all.rc		builtin		cmp		database	flag		html		log		mime		race.bat	runtime		syscall		unsafe
archive		bytes		cmp.bash	debug		fmt		image		make.bash	net		reflect		slices		testdata	vendor
*/

// stdlibExclusionRegexes returns one compiled regexp per top-level GOROOT/src directory (other
// than "sync"-prefixed ones and empty names), each matching that package and all of its
// subpackages. This is what makes standard-library packages excluded automatically without
// needing to be listed in DefaultExcludePackagePatterns.
func (g *builderImpl) stdlibExclusionRegexes() ([]*regexp.Regexp, error) {
	goRoot := os.Getenv("GOROOT")
	if goRoot == "" {
		return nil, fmt.Errorf("GOROOT is not set")
	}

	g.logger.Debugf("stdlibExclusionRegexes GOROOT: %s\n", goRoot)

	entries, err := os.ReadDir(filepath.Join(goRoot, "src"))
	if err != nil {
		return nil, fmt.Errorf("error reading GOROOT/src: %w", err)
	}

	regexes := make([]*regexp.Regexp, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pkg := entry.Name()
		// Exclude "sync" prefix -- sync/atomic and friends are small and frequently relevant to
		// fail-close analysis of concurrent error handling.
		if strings.HasPrefix(pkg, "sync") {
			continue
		}
		if pkg == "" {
			continue
		}
		g.logger.Debugf("Added: %s", pkg)

		re, err := regexp.Compile("^" + regexp.QuoteMeta(pkg) + "(/.*)?$")
		if err != nil {
			return nil, fmt.Errorf("compile stdlib exclusion pattern for %q: %w", pkg, err)
		}
		regexes = append(regexes, re)
	}

	g.logger.Debugf("stdlibExclusionRegexes size: %d\n", len(regexes))
	return regexes, nil
}

// excludePackageRegexes returns the fully compiled, merged set of package-exclusion regexes
// (g.config.ExcludePackagePatterns or DefaultExcludePackagePatterns, plus the automatic GOROOT
// stdlib scan), compiling and caching them on first use so that repeated getBlockListPackages
// calls within this builder's lifetime don't recompile the pattern list or re-scan GOROOT/src.
func (g *builderImpl) excludePackageRegexes() ([]*regexp.Regexp, error) {
	g.excludePkgRegexOnce.Do(func() {
		patterns := g.config.ExcludePackagePatterns
		if patterns == nil {
			patterns = DefaultExcludePackagePatterns
		}

		regexes := make([]*regexp.Regexp, 0, len(patterns))
		for _, pattern := range patterns {
			re, err := regexp.Compile(pattern)
			if err != nil {
				g.excludePkgRegexErr = fmt.Errorf("compile exclude package pattern %q: %w", pattern, err)
				return
			}
			regexes = append(regexes, re)
		}

		stdlibRegexes, err := g.stdlibExclusionRegexes()
		if err != nil {
			g.excludePkgRegexErr = err
			return
		}

		g.excludePkgRegex = append(regexes, stdlibRegexes...)
	})
	return g.excludePkgRegex, g.excludePkgRegexErr
}

func (g *builderImpl) getBlockListPackages(pkgs []*ssa.Package) (map[*ssa.Package]bool, error) {
	regexes, err := g.excludePackageRegexes()
	if err != nil {
		return nil, err
	}
	result := make(map[*ssa.Package]bool, len(pkgs))
	for _, pkg := range pkgs {
		// Empty package drop for sure
		if pkg == nil || pkg.Pkg == nil || pkg.Pkg.Path() == "" {
			result[pkg] = true
			continue
		}
		curPath := pkg.Pkg.Path()
		g.logger.Debugf("getBlockListPackages checking: %s", curPath)

		found := false
		for _, re := range regexes {
			if re.MatchString(curPath) {
				g.logger.Debugf("getBlockListPackages blocking: %s", curPath)
				result[pkg] = true
				found = true
				break
			}
		}
		if !found {
			g.logger.Debugf("getBlockListPackages not blocking: %s", curPath)
		}

		// Default is false. If the package is not in the exclusion set, then it is not blocked
	}
	return result, nil
}
