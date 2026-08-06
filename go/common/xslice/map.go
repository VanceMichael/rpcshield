package xslice

import (
	"fmt"
)

// Map applies mapper function `func(From) To` to each element of a slice of type From
// and returns it as a slice of type To. Map should be called when
// the mapper function can't return any error and always succeeds.
//
// Calls to Map shouldn't be chained like `Map(Map(Map(a1, f1), f2), f3)`
// due to possible performance degradation. For such a case consider using for loops instead.
//
// This function is nil safe.
//
// Example: map a slice of ints to a slice of strings
//
//	ints := []int{1, 2, 3}
//	strings := Map(ints, strconv.Itoa)
func Map[From any, To any](a []From, mapper func(From) To) []To {
	if a == nil {
		return nil
	}
	result := make([]To, len(a))
	for i, e := range a {
		result[i] = mapper(e)
	}
	return result
}

// TryMap applies mapper function `func(From) (To, error)` to each element of a slice of type From
// and returns it as a slice of type To. TryMap should be called when
// the mapper function can return an error.
//
// If the mapper func returns an error, then TryMap will return `nil, err`. No partial mapping result
// will be returned.
//
// Calls to TryMap shouldn't be chained like `TryMap(TryMap(TryMap(a1, f1), f2), f3)`
// due to possible performance degradation. For such a case consider using for loops instead.
//
// This function is nil safe.
//
// Example: map a slice of protobuf UUIDs to a slice of gofrs UUIDs
//
//	 pbUUIDtoUUID := func(pbUUID pb.UUID) (gofrsuuid.UUID, error) {
//		  return gofrsuuid.FromString(pbUUID.String())
//	 }
//
//	 uuids, err := TryMap(pbMessage.GetUUIDS(), pbUUIDtoUUID)
//	 if err != nil {
//	 }
func TryMap[From any, To any](a []From, mapper func(From) (To, error)) ([]To, error) {
	if a == nil {
		return nil, nil
	}
	result := make([]To, len(a))
	for i, e := range a {
		mapped, err := mapper(e)
		if err != nil {
			return nil, fmt.Errorf("trymap: element at '%d', error: %w", i, err)
		}
		result[i] = mapped
	}
	return result, nil
}

// ToMapFunc converts a slice to a map using the provided mapper function `func(E) (K, V)` to get the key and value for
// every element of the provided slice.
//
// WARNING: if multiple elements map to the same key, later elements with a given key would override earlier elements. As
// a result, the returned map may have fewer entries than the provided slice.
func ToMapFunc[S ~[]E, E any, K comparable, V any](s S, mapFunc func(E) (K, V)) map[K]V {
	result := make(map[K]V, len(s))
	for _, v := range s {
		mapKey, mapVal := mapFunc(v)
		result[mapKey] = mapVal
	}
	return result
}
