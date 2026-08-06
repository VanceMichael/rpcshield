package servicemapping

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/uber-research/rpcshield/go/constant"
	"github.com/uber-research/rpcshield/go/grail"
)

var _withVersionSuffixRegex = regexp.MustCompile("(.*)v[0-9]$")

// nameMatch infers the service name from the client package path based on heuristics (
// edit distances from the parts of package path to the service name). Nil is returned if the
// inference failed.
func (m *Mapper) nameMatch(clientPkgPath string) *grail.Service {
	original := clientPkgPath
	parts := strings.Split(clientPkgPath, "/")
	if parts[0] == constant.ThriftrwPkgPrefix {
		parts = parts[:len(parts)-1] // Trim the client package name for thriftrw
	}
	parts = parts[1:] // Trim "thriftrw"/"gogoproto"
	if m.codebasePkgPrefix != "" && len(parts) > 0 && parts[0] == m.codebasePkgPrefix {
		parts = parts[1:] // Trim the codebase package prefix, e.g. "example.internal"
	}
	m.logger.Debugw("Distilled client package path", "parts", parts, "original", original)

	// We try to infer the service name by taking 4, 3, 2 or 1 parts (sliding window) of the
	// package path and concatenating them to form candidate service names. Note that the longer
	// names are given preference to the shorter ones (if they both matched) since they are more
	// likely to be the service names. Moreover, we also start from the end of the package path
	// since the most specific service name is more likely to be the correct service name,
	// e.g., "example.internal/infra/progsys/cffdemo" would be tried as:
	// "infraprogsyscffdemo"
	// "progsyscffdemo"
	// "infraprogsys"
	// "cffdemo"
	// "progsys"
	var candidates []string
	for _, numParts := range []int{4, 3, 2, 1} {
		var toAdds []string
		// When we are trying to add candidate only for one part, we should ignore the first
		// top-level folder since the name is usually very generic, like "infra", "fares" etc.
		// that is highly likely to be matched to an incorrect generic service name.
		start := 0
		if numParts == 1 {
			start = 1
		}
		// We will compare the names without hyphens at all (a conservative fuzzy match), so we can
		// simply join the parts without any separator.
		for i := len(parts) - numParts; i >= start; i-- {
			toAdds = append(toAdds, strings.Join(parts[i:i+numParts], ""))
		}

		// Somtimes service names are suffixed with "-go", so we also try that (only if such
		// suffix does not already exist).
		for _, t := range toAdds {
			if !strings.HasSuffix(t, "-go") {
				toAdds = append(toAdds, t+"-go")
			}
		}

		// Sometimes package paths and service names contain version suffixes (e.g., "v1", "v2").
		// We try to add (or remove) such suffixes to the candidate service names to try to match.
		// Note that these candidates are added after the original names to give them lower priority.
		//
		// For example:
		// 1. We get package path `foo-v2` -> add `foo` as a candidate, since if we can’t find an
		//    exact match of `foo-v2`, it's most likely that the service simply does not have a
		//    versioning naming scheme, and we just try to match the original name.
		// 2. We get package path `foo` -> add `foo-v1`, `foo-v2`... `foo-v9` as candidates: if we
		//    cannot find an exact match of foo, and there are foo-v1 and foo-v2 , we would always
		//    match `foo-v1` due to the order. However, since `foo-v1` and `foo-v2` are probably of
		//    the same tier, and even if we made a small mistake there, everything else in the
		//    rpcshield's analysis would still be correct.
		for _, t := range toAdds {
			matches := _withVersionSuffixRegex.FindStringSubmatch(t)
			if len(matches) != 0 {
				// The name with version suffix trimmed (captured group 1).
				toAdds = append(toAdds, matches[1])
			} else {
				// The version suffix does not exist, try to add possible version suffixes.
				for v := range 10 {
					toAdds = append(toAdds, t+"v"+strconv.Itoa(v))
				}
			}
		}
		candidates = append(candidates, toAdds...)
	}

	m.logger.Debugw("Inferred service candidates", "candidates", candidates)

	// Note that we conservatively do semi-exact matches (modulo hyphens) to reduce false positives.
	// This would lead to false negatives in some cases. However, this would significantly reduce
	// the number of false positives.
	for _, candidate := range candidates {
		for _, service := range m.serviceMap {
			if normalizeServiceName(candidate) == normalizeServiceName(service.Name) {
				m.logger.Debugw("Inferred service", "client_pkg_path", clientPkgPath, "service", service.Name)
				return &service
			}
		}
	}

	m.logger.Warnw("Failed to infer service", "client_pkg_path", clientPkgPath)
	return nil
}

func normalizeServiceName(serviceName string) string {
	return strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(serviceName), "-", ""), "_", "")
}
