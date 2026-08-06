package cacheanalyzer

import (
	"fmt"
	"sync"

	"github.com/uber-research/rpcshield/go/constant"
	"golang.org/x/tools/go/ssa"
)

// CacheAnalyzer is an interface for analyzing and caching the failness of a function at an instruction.
type CacheAnalyzer interface {
	// AnalyzeCall returns the failness of a function at an instruction if it is found in the cache.
	AnalyzeCall(f *ssa.Function, call ssa.Instruction) (OriginalFailnessInfo, error)
	// InsertToCache inserts the failness of a function at an instruction into the cache.
	InsertToCache(f *ssa.Function, call ssa.Instruction, failness constant.Failness, source constant.AnalyzerKind)
	// One day when we need to delete from the cache, we can add this method.
}

// OriginalFailnessInfo is a struct that contains the failness statuc and the source of the failness.
type OriginalFailnessInfo struct {
	Status constant.Failness
	Source constant.AnalyzerKind
}

type cacheAnalyzer struct {
	*sync.RWMutex
	cache map[funcInsPair]OriginalFailnessInfo
}

type funcInsPair struct {
	f *ssa.Function
	c ssa.Instruction
}

// NewCacheAnalyzer creates a new CacheAnalyzer.
func NewCacheAnalyzer() CacheAnalyzer {
	return &cacheAnalyzer{
		RWMutex: &sync.RWMutex{},
		cache:   make(map[funcInsPair]OriginalFailnessInfo),
	}
}

// AnalyzeCall returns the failness of a function at an instruction if it is found in the cache.
func (c *cacheAnalyzer) AnalyzeCall(f *ssa.Function, call ssa.Instruction) (OriginalFailnessInfo, error) {
	key := funcInsPair{f: f, c: call}
	defer c.RUnlock()
	c.RLock()
	if failness, ok := c.cache[key]; ok {
		return failness, nil
	}
	return OriginalFailnessInfo{Status: constant.FailUnknown, Source: constant.Unknown}, fmt.Errorf("failness not found in cache")
}

// InsertToCache inserts the failness of a function at an instruction into the cache.
func (c *cacheAnalyzer) InsertToCache(f *ssa.Function, call ssa.Instruction, failness constant.Failness, source constant.AnalyzerKind) {
	key := funcInsPair{f: f, c: call}
	defer c.Unlock()
	c.Lock()
	c.cache[key] = OriginalFailnessInfo{Status: failness, Source: source}
}
