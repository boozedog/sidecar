# Fix ANSI Truncation Memory Leak

## Problem Summary

Active memory leak detected: 51GB allocated in 8 minutes (106 MB/second throughput) caused by excessive `ansi.Truncate()` calls in hot rendering paths.

**Root Cause:** `ansi.Truncate()` is called ~1,120 times per second (500ms polling × ~560 ops per render) without any caching, causing massive string allocation churn.

**Hot Paths:**
- `worktree/view_preview.go:truncateAllLines()` - Processes entire 500-line buffer every render
- `worktree/view_preview.go:renderOutputContent()` - Truncates ~30 visible lines 2x each per render
- `worktree/view_diff.go` - Similar pattern for diff content
- `gitstatus/diff_renderer.go` - Side-by-side diff rendering

## Solution: Line-Level Truncation Cache

Add caching for truncated line results, following existing patterns:
- `sync.Pool` pattern from `adapter/codex/adapter.go:17-30`
- Render cache pattern from `conversations/plugin.go:169-171`

## Implementation Plan

### Phase 1: Create Truncation Cache

**Create:** `internal/ui/truncate_cache.go`

Implement cache with:
- Cache key: `{content: string, width: int, offset: int}`
- Thread-safe with `sync.RWMutex` (read-heavy workload)
- Methods: `Truncate()`, `TruncateLeft()`, `Clear()`
- Max size limit: 1000 entries for safety

### Phase 2: Integrate into Worktree Plugin (Highest Impact)

**Modify:** `internal/plugins/worktree/plugin.go`
- Add `truncateCache *ui.TruncateCache` field to Plugin struct
- Initialize in `New()` function

**Modify:** `internal/plugins/worktree/view_preview.go`
- Update `truncateAllLines()` (lines 107-131) to use cached truncation
- Update `renderOutputContent()` (lines 207, 211) to use cached truncation
- Add early return if width unchanged since last render

**Modify:** `internal/plugins/worktree/agent.go`
- Add cache invalidation at line 576 when `OutputBuf.Update(output)` returns `true`

**Modify:** `internal/plugins/worktree/view_diff.go`
- Update lines 115, 118, 135 to use cached truncation
- Share cache instance with view_preview (same plugin)

### Phase 3: Extend to Gitstatus Plugin

**Modify:** `internal/plugins/gitstatus/plugin.go`
- Add `truncateCache *ui.TruncateCache` field to Plugin struct
- Initialize in plugin creation

**Modify:** `internal/plugins/gitstatus/diff_renderer.go`
- Update lines 225, 240, 335 to use cached truncation
- Clear cache when diff content changes

**Modify:** `internal/plugins/gitstatus/sidebar_view.go`
- Update line 875 to use cached truncation

### Cache Invalidation Strategy

Clear cache when:
1. Content changes (OutputBuffer.Update returns true)
2. Window resizes (width changes)
3. Horizontal scroll offset changes

Conservative approach: Clear entire cache on any change to avoid stale content.

## Critical Files

- `internal/ui/truncate_cache.go` (new)
- `internal/plugins/worktree/plugin.go`
- `internal/plugins/worktree/view_preview.go`
- `internal/plugins/worktree/view_diff.go`
- `internal/plugins/worktree/agent.go`
- `internal/plugins/gitstatus/plugin.go`
- `internal/plugins/gitstatus/diff_renderer.go`
- `internal/plugins/gitstatus/sidebar_view.go`

## Expected Impact

- **Phase 2 complete:** 80-90% reduction in allocations for worktree plugin
- **Phase 3 complete:** 90-95% reduction in total ansi-related allocations
- **Target:** Allocation rate drops from 106 MB/s to <20 MB/s

## Verification

### Before Implementation
1. Capture baseline: `pprof` memory profile showing 51GB/8min allocation rate
2. Identify exact allocation sites with `-memprofilerate=1`

### After Phase 2
1. Re-run profiler with worktree changes only
2. Verify 80%+ reduction in `ansi.Truncate` allocations
3. Visual test: Agent output updates correctly, no artifacts

### After Phase 3
1. Full profiling with all plugins active
2. Verify 90%+ total reduction in ansi allocations
3. Visual test: Diff rendering, git status rendering work correctly
4. Race detector: `go test -race ./...`

### Success Criteria
- Memory allocation rate: <20 MB/s (was 106 MB/s)
- No performance regression in render times
- No visual artifacts or stale content
- Cache hit rate >90% in steady state

## Risks & Mitigations

**Risk:** Stale cache causing outdated content display
- **Mitigation:** Conservative invalidation (clear on any content/size change)
- **Validation:** Visual testing with rapidly changing agent output

**Risk:** Cache memory growth
- **Mitigation:** Natural bound (cleared on content change) + 1000 entry limit
- **Monitoring:** Cache size during active agent sessions

**Risk:** Concurrency issues
- **Mitigation:** Use `sync.RWMutex` for thread-safety
- **Validation:** Run with `-race` detector

## Notes

- Cache is per-plugin instance, not global (better locality)
- Follows existing codebase patterns (conversations render cache, scanner pool)
- Drop-in replacement: transparent to rendering logic
- Memory cost: ~50-200KB per active plugin (acceptable trade-off)
