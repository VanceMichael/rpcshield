package constant

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// rpcshield is the name of the tool
const rpcshield = "rpcshield"

// FailCloseAnnotation is the annotation for fail close
const FailCloseAnnotation = "rpcshield:failclose"

// FailOpenAnnotation is the annotation for fail open
const FailOpenAnnotation = "rpcshield:failopen"

// FailCloseSuppressAnnotation is the annotation to suppress fail-close warnings
const FailCloseSuppressAnnotation = "rpcshield:suppress-fail-close"

// LanguageGo is the constant for Go
const LanguageGo = "go"

// LanguageJava is the constant for Java
const LanguageJava = "java"

// SourceForCSV is the source for the CSV report
const SourceForCSV = "go_static"

// MaxPathsToStoreForQuadruple is the maximum number of paths to store for a quadruple
const MaxPathsToStoreForQuadruple = 20

// GitSHAPlaceholderString is the placeholder for the git SHA in a Sourcegraph URL template
const GitSHAPlaceholderString = "{GitSHA}"

// FilePathPlaceholderString is the placeholder for the filepath in a Sourcegraph URL template
const FilePathPlaceholderString = "{filepath}"

// LinePlaceholderString is the placeholder for the line number in a Sourcegraph URL template
const LinePlaceholderString = "{line}"

// _serviceNamePlaceholderString is the placeholder for the service name in a Terrablob URL template
const _serviceNamePlaceholderString = "{service_name}"

// _timestampPlaceholderString is the placeholder for the timestamp in a Terrablob URL template
const _timestampPlaceholderString = "{timestamp}"

// _repoPlaceholderString is the placeholder for the repo in a Terrablob URL template
const _repoPlaceholderString = "{repo}"

// CFFGenSuffix is the suffix for CFFGen in filepaths.
// E.g., src/example.internal/rider/presentation/cx-presentation/controller/homescreen/homescreen_cffgen.go
const CFFGenSuffix = "_cffgen"

// BuildTBFileURL fills in the repo, service name, and timestamp placeholders of a Terrablob file
// URL template (see config.Config.TerrablobFileURLTemplate). An empty template returns "".
func BuildTBFileURL(urlTemplate, repo, serviceName, timestamp string) string {
	if urlTemplate == "" {
		return ""
	}

	tbURL := strings.Replace(urlTemplate, _repoPlaceholderString, repo, 1)
	tbURL = strings.Replace(tbURL, _serviceNamePlaceholderString, serviceName, 1)

	timestampStr := url.PathEscape(timestamp)
	timestampStr = strings.ReplaceAll(timestampStr, ":", "%3A")

	tbURL = strings.Replace(tbURL, _timestampPlaceholderString, timestampStr, 1)
	return tbURL
}

// Set of reasons for fail-ness status of a function. TODO: Use for diagnostics.
const (
	FailOpenNoErrReturningFunc   = "Fail open since the function is not error returning"
	FailOpenInvalidFunc          = "Fail open since the function is not a valid candidate for error propagation analysis"
	FailOpenCallSiteNoReturn     = "Fail open since the function does not return an error influenced by the callsite"
	FailOpenCallSiteAllNilReturn = "Fail open since the function returns a nil error for all paths influenced by the callsite"
	FailCloseCallSiteReturn      = "Fail close since at least one path is found in the function that returns an error influenced by the callsite"
	FailUnknownCallSiteReturn    = "Fail status unknown since the function returns an unknown error value influenced by the callsite"
)

// Failness is a custom type for failness status from analysis.
type Failness string

const (
	// FailUnknown is a status for unknown
	FailUnknown Failness = "FailUnknown"
	// FailUnknownIntended is a status for unknown intended. For example, if the function returns another function `return foo()`.
	FailUnknownIntended Failness = "FailUnknownIntended"
	// FailUnknownWithError is a status for unknown with error.
	FailUnknownWithError Failness = "FailUnknownWithError"
	// FailOpen is a status for fail open
	FailOpen Failness = "FailOpen"
	// FailClose is a status for fail close
	FailClose Failness = "FailClose"
	// FailCloseSuppressed is a status for fail close but suppressed by user annotations.
	FailCloseSuppressed Failness = "FailCloseSuppressed"
)

// OrderMapFailClosePriority is a map to determine the priority of fail-ness status with the precedence order:
// FailClose > FailUnknownIntended > FailUnknownWithError > FailUnknown > FailOpen > FailCloseSuppressed.
var OrderMapFailClosePriority = map[Failness]int{
	FailClose:            0,
	FailUnknownIntended:  1,
	FailUnknownWithError: 2,
	FailUnknown:          3,
	FailOpen:             4,
	FailCloseSuppressed:  5,
}

// OrderMapFailOpenPriority is a map to determine the priority of fail-ness status with the precedence order:
// FailCloseSuppressed > FailOpen > FailUnknown > FailUnknownWithError > FailUnknownIntended > FailClose.
var OrderMapFailOpenPriority = map[Failness]int{
	FailCloseSuppressed:  0,
	FailOpen:             1,
	FailUnknown:          2,
	FailUnknownWithError: 3,
	FailUnknownIntended:  4,
	FailClose:            5,
}

// ValidateFailness checks if the failness value is a valid one.
func ValidateFailness(f Failness) error {
	if f != FailUnknown && f != FailOpen && f != FailClose && f != FailUnknownWithError && f != FailUnknownIntended && f != FailCloseSuppressed {
		return fmt.Errorf("invalid failness value: %q", f)
	}
	return nil
}

// UnmarshalJSON unmarshals the JSON data into the Failness type with validation.
func (f *Failness) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, (*string)(f)); err != nil {
		return err
	}
	return ValidateFailness(*f)
}

// AnalyzerKind is a custom type to determine the kind of analyzer.
type AnalyzerKind string

const (
	// Unknown is a default status code
	Unknown AnalyzerKind = "Unknown"
	// AST is a status code for AST-based analysis
	AST AnalyzerKind = "AST"
	// SSA is a status code for SSA-based analysis
	SSA AnalyzerKind = "SSA"
	// SCCP is a status code for SCCP-based (formal) SSA analysis
	SCCP AnalyzerKind = "SCCP"
	// LLM is a status code for LLM-based analysis
	LLM AnalyzerKind = "LLM"
	// Annotation is a status code for annotation-based analysis
	Annotation AnalyzerKind = "Annotation"
	// Predefined is a status code for analysis that uses the predefined rules
	Predefined AnalyzerKind = "Predefined"
	// CacheAnalyzerKind is a status code for cache analyzer
	CacheAnalyzerKind AnalyzerKind = "Cache"
)

// UnmarshalJSON unmarshals the JSON data into the AnalyzerKind type with validation.
func (a *AnalyzerKind) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, (*string)(a)); err != nil {
		return err
	}

	// Special casing for supporting backward compatibility
	if *a == "CacheAnalyzer" {
		*a = CacheAnalyzerKind
	}
	if *a == "" {
		*a = Unknown
	}

	switch *a {
	case Unknown, AST, SSA, SCCP, LLM, Annotation, Predefined, CacheAnalyzerKind:
		return nil
	default:
		return fmt.Errorf("invalid analyzer kind: %q", *a)
	}
}

// ErrorValue represents the possible values of an error
type ErrorValue uint8

const (
	// ErrorUnknown is the default error value before analysis
	ErrorUnknown ErrorValue = 1 << iota // 0b00001
	// ErrorNonnil represents a determined non-nil error value
	ErrorNonnil // 0b00010
	// ErrorNil represents a determined nil error value
	ErrorNil // 0b00100
	// ErrorUndetermined represents that the error value cannot be determined after analysis
	ErrorUndetermined // 0b01000
	// ErrorUndeterminedForCallExpr represents that the error value cannot be determined for a CallExpr (e.g., return foo())
	ErrorUndeterminedForCallExpr // 0b10000
)

// FuncValidity represents the possible validity values of a function
type FuncValidity uint8

const (
	// FuncValidityUnknown is the default function validity value when it is not known or cannot be determined
	FuncValidityUnknown FuncValidity = iota
	// FuncValid represents a valid function for error propagation analysis
	FuncValid
	// FuncInvalidAllRetNilAndOutboundFound represents an invalid function where all return statements return nil and an outbound sink site is found
	FuncInvalidAllRetNilAndOutboundFound
	// FuncInvalidOutboundNotFound represents an invalid function where an outbound sink site is not found
	FuncInvalidOutboundNotFound
	// FuncInvalidOtherReason represents an invalid function for some other reason
	FuncInvalidOtherReason
)

// String returns the string representation of a FuncValidity
func (f FuncValidity) String() string {
	switch f {
	case FuncValid:
		return "FuncValid"
	case FuncInvalidAllRetNilAndOutboundFound:
		return "FuncInvalidAllRetNilAndOutboundFound"
	case FuncInvalidOutboundNotFound:
		return "FuncInvalidOutboundNotFound"
	case FuncInvalidOtherReason:
		return "FuncInvalidOtherReason"
	}
	panic(fmt.Sprintf("unknown validity value: %d", f))
}

// GetFuncValidityFromString returns the FuncValidity corresponding to the given string
func GetFuncValidityFromString(s string) FuncValidity {
	switch strings.ToLower(s) {
	case strings.ToLower("FuncValid"):
		return FuncValid
	case strings.ToLower("FuncInvalidAllRetNilAndOutboundFound"):
		return FuncInvalidAllRetNilAndOutboundFound
	case strings.ToLower("FuncInvalidOutboundNotFound"):
		return FuncInvalidOutboundNotFound
	case strings.ToLower("FuncInvalidOtherReason"):
		return FuncInvalidOtherReason
	}
	panic(fmt.Sprintf("unknown validity value: %s", s))
}

const (
	// GogoprotoPkgPrefix is the prefix for gogoproto package.
	GogoprotoPkgPrefix = "gogoproto"
	// ThriftrwPkgPrefix is the prefix for thriftrw package.
	ThriftrwPkgPrefix = "thriftrw"
	// CadenceClientPkg is the package path for cadence client.
	CadenceClientPkg = "go.uber.org/cadence/client"
	// CadenceServiceName is the service name for cadence.
	CadenceServiceName = "cadence"
)

// ThriftSubpackageSuffixes is the list of suffixes for thrift subpackages.
var ThriftSubpackageSuffixes = []string{"client", "server", "fx"}

// IsThriftSubpackage returns true if the given package path is a thrift subpackage.
func IsThriftSubpackage(pkgPath string) bool {
	if !strings.HasPrefix(pkgPath, ThriftrwPkgPrefix) {
		return false
	}
	for _, suffix := range ThriftSubpackageSuffixes {
		if strings.HasSuffix(pkgPath, suffix) {
			return true
		}
	}
	return false
}

// IsRPCGeneratedPkg returns true if the given package path indicates it is a generated package by gogoproto or thriftrw.
func IsRPCGeneratedPkg(pkgPath string) bool {
	return strings.HasPrefix(pkgPath, GogoprotoPkgPrefix+"/") || strings.HasPrefix(pkgPath, ThriftrwPkgPrefix+"/")
}
