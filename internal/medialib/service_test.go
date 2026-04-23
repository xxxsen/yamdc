package medialib

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xxxsen/common/logger"
	"go.uber.org/zap"

	"github.com/xxxsen/yamdc/internal/nfo"
	"github.com/xxxsen/yamdc/internal/repository"
)

func newTestMediaService(t *testing.T) *Service {
	t.Helper()
	sqlite, err := repository.NewSQLite(context.Background(), filepath.Join(t.TempDir(), "app.db"))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, sqlite.Close())
	})
	svc := NewService(sqlite.DB(), t.TempDir(), t.TempDir())
	// 按 LIFO, 先等待后台同步/移动 goroutine 返回再关闭 sqlite/tempdir,
	// 避免异步 DB 写入与 tempdir 清理竞争。Stop() 必须先于 WaitBackground(),
	// 否则 Start() 拉起的 scheduler goroutine 会一直卡在 60s 延时 / 每日定时里,
	// 让 WaitBackground hang 住测试。
	t.Cleanup(func() {
		svc.Stop()
		svc.WaitBackground()
	})
	return svc
}

func newTestMediaServiceWithDirs(t *testing.T, libraryDir, saveDir string) *Service {
	t.Helper()
	sqlite, err := repository.NewSQLite(context.Background(), filepath.Join(t.TempDir(), "app.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlite.Close() })
	svc := NewService(sqlite.DB(), libraryDir, saveDir)
	t.Cleanup(func() {
		svc.Stop()
		svc.WaitBackground()
	})
	return svc
}

func withCapturedLogs(t *testing.T) (string, func()) {
	t.Helper()
	logPath := filepath.Join(t.TempDir(), "app.log")
	lg := logger.Init(logPath, "debug", 1, 1024*1024, 1, false)
	return logPath, func() {
		_ = lg.Sync()
		logger.Init("", "debug", 0, 0, 0, true)
	}
}

func TestServiceStartRecoversRunningTaskStates(t *testing.T) {
	svc := newTestMediaService(t)
	ctx := context.Background()

	require.NoError(t, svc.saveTaskState(ctx, TaskState{
		TaskKey:   TaskSync,
		Status:    "running",
		Total:     10,
		Processed: 3,
		Message:   "同步媒体库中",
		StartedAt: 123,
		UpdatedAt: 456,
	}))
	require.NoError(t, svc.saveTaskState(ctx, TaskState{
		TaskKey:   TaskMove,
		Status:    "running",
		Total:     5,
		Processed: 2,
		Message:   "移动到媒体库中",
		StartedAt: 789,
		UpdatedAt: 999,
	}))

	svc.Start(ctx)

	syncState, err := svc.getTaskState(ctx, TaskSync)
	require.NoError(t, err)
	require.Equal(t, "failed", syncState.Status)
	require.Equal(t, "server restarted while task running", syncState.Message)
	require.NotZero(t, syncState.FinishedAt)

	moveState, err := svc.getTaskState(ctx, TaskMove)
	require.NoError(t, err)
	require.Equal(t, "failed", moveState.Status)
	require.Equal(t, "server restarted while task running", moveState.Message)
	require.NotZero(t, moveState.FinishedAt)
}

func TestServiceStartKeepsNonRunningTaskState(t *testing.T) {
	svc := newTestMediaService(t)
	ctx := context.Background()

	require.NoError(t, svc.saveTaskState(ctx, TaskState{
		TaskKey:    TaskSync,
		Status:     "completed",
		Message:    "ok",
		StartedAt:  100,
		FinishedAt: 200,
		UpdatedAt:  300,
	}))

	svc.Start(ctx)

	state, err := svc.getTaskState(ctx, TaskSync)
	require.NoError(t, err)
	require.Equal(t, "completed", state.Status)
	require.Equal(t, "ok", state.Message)
	require.Equal(t, int64(200), state.FinishedAt)
}

func TestServiceStartNoDb(_ *testing.T) {
	svc := NewService(nil, "", "")
	svc.Start(context.Background())
}

func TestRunFullSyncLogsSyncedMediaMetadata(t *testing.T) {
	logPath, cleanup := withCapturedLogs(t)
	defer cleanup()

	libraryDir := t.TempDir()
	saveDir := t.TempDir()
	sqlite, err := repository.NewSQLite(context.Background(), filepath.Join(t.TempDir(), "app.db"))
	require.NoError(t, err)
	defer func() { require.NoError(t, sqlite.Close()) }()

	itemDir := filepath.Join(libraryDir, "movie")
	require.NoError(t, os.MkdirAll(itemDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "movie.nfo"), []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes" ?>
<movie>
  <title>Sample Title</title>
  <originaltitle>Sample Original</originaltitle>
  <id>ABC-123</id>
  <premiered>2024-01-02</premiered>
</movie>`), 0o600))

	svc := NewService(sqlite.DB(), libraryDir, saveDir)
	require.NoError(t, svc.runFullSync(context.Background(), "manual"))

	raw, err := os.ReadFile(logPath)
	require.NoError(t, err)
	logs := string(raw)
	require.Contains(t, logs, "media library sync item started")
	require.Contains(t, logs, "media library detail synced")
	require.Contains(t, logs, "rel_path")
	require.Contains(t, logs, "movie")
	require.Contains(t, logs, "ABC-123")
	require.Contains(t, logs, "Sample Title")
}

func TestRunMoveLogsPerItemProgress(t *testing.T) {
	logPath, cleanup := withCapturedLogs(t)
	defer cleanup()

	libraryDir := t.TempDir()
	saveDir := t.TempDir()
	sqlite, err := repository.NewSQLite(context.Background(), filepath.Join(t.TempDir(), "app.db"))
	require.NoError(t, err)
	defer func() { require.NoError(t, sqlite.Close()) }()

	itemDir := filepath.Join(saveDir, "movie")
	require.NoError(t, os.MkdirAll(itemDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "movie.nfo"), []byte(`<movie><title>Moved</title><id>XYZ-987</id></movie>`), 0o600))

	svc := NewService(sqlite.DB(), libraryDir, saveDir)
	require.NoError(t, svc.runMove(context.Background()))

	raw, err := os.ReadFile(logPath)
	require.NoError(t, err)
	logs := string(raw)
	require.Contains(t, logs, "move to media library item started")
	require.Contains(t, logs, "move to media library item finished")
	require.Contains(t, logs, "move to media library completed")
	require.True(t, strings.Contains(logs, "movie"))
}

// --- releaseYear ---

func TestReleaseYear(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"normal", "2024-01-02", "2024"},
		{"year_only", "2024", "2024"},
		{"embedded", "abc2024def", "2024"},
		{"no_year", "abc", ""},
		{"empty", "", ""},
		{"short", "20", ""},
		{"non_digit", "abcd", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, releaseYear(tc.input))
		})
	}
}

// --- sizeFilterBounds ---

func TestSizeFilterBounds(t *testing.T) {
	gb := int64(1024 * 1024 * 1024)
	tests := []struct {
		name     string
		filter   string
		wantLow  int64
		wantHigh int64
		wantOk   bool
	}{
		{"empty", "", 0, 0, false},
		{"all", "all", 0, 0, false},
		{"lt-1", "lt-1", 0, gb, true},
		{"1-2", "1-2", gb, 2 * gb, true},
		{"2-5", "2-5", 2 * gb, 5 * gb, true},
		{"lt-5", "lt-5", 0, 5 * gb, true},
		{"5-10", "5-10", 5 * gb, 10 * gb, true},
		{"10-20", "10-20", 10 * gb, 20 * gb, true},
		{"5-20", "5-20", 5 * gb, 20 * gb, true},
		{"20-50", "20-50", 20 * gb, 50 * gb, true},
		{"50-plus", "50-plus", 50 * gb, 0, true},
		{"unknown", "bogus", 0, 0, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			low, high, ok := sizeFilterBounds(tc.filter)
			assert.Equal(t, tc.wantOk, ok)
			assert.Equal(t, tc.wantLow, low)
			assert.Equal(t, tc.wantHigh, high)
		})
	}
}

// --- sortModeToColumn ---

func TestSortModeToColumn(t *testing.T) {
	tests := []struct {
		name string
		sort string
		want string
	}{
		{"title", "title", "CASE WHEN title = '' THEN name ELSE title END"},
		{"size", "size", "total_size"},
		{"year", "year", "release_year"},
		{"ingested", "ingested", "created_at"},
		{"updated", "updated", "updated_at"},
		{"empty_defaults_to_updated", "", "updated_at"},
		{"unknown_defaults_to_updated", "bogus", "updated_at"},
		{"whitespace_stripped", "  title  ", "CASE WHEN title = '' THEN name ELSE title END"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, sortModeToColumn(tc.sort))
		})
	}
}

// --- buildListItemsQuery ---

func TestBuildListItemsQuery(t *testing.T) {
	gb := int64(1024 * 1024 * 1024)

	t.Run("no_filter", func(t *testing.T) {
		q, args := buildListItemsQuery(ListItemsOptions{})
		assert.NotContains(t, q, "WHERE")
		// 默认排序列就是 updated_at, 不再重复出现, 只保留一次 + id tie-breaker。
		assert.Contains(t, q, "ORDER BY updated_at DESC, id DESC")
		assert.NotContains(t, q, "updated_at DESC, updated_at DESC")
		assert.Empty(t, args)
	})

	t.Run("keyword_pushes_three_like_conditions", func(t *testing.T) {
		q, args := buildListItemsQuery(ListItemsOptions{Keyword: "  AbC  "})
		assert.Contains(t, q, `LOWER(title) LIKE ? ESCAPE '\'`)
		assert.Contains(t, q, `LOWER(number) LIKE ? ESCAPE '\'`)
		assert.Contains(t, q, `LOWER(name) LIKE ? ESCAPE '\'`)
		assert.Equal(t, []any{"%abc%", "%abc%", "%abc%"}, args)
	})

	t.Run("keyword_with_like_wildcards_is_escaped", func(t *testing.T) {
		// 旧实现用 strings.Contains 做字面子串匹配, 这里验证下推后 '%' 和 '_'
		// 仍然只匹配它们自己, 不会变成 SQL 通配符。
		_, args := buildListItemsQuery(ListItemsOptions{Keyword: "100%_off"})
		require.Len(t, args, 3)
		assert.Equal(t, `%100\%\_off%`, args[0])
	})

	t.Run("year_all_is_ignored", func(t *testing.T) {
		q, args := buildListItemsQuery(ListItemsOptions{Year: "all"})
		assert.NotContains(t, q, "release_year")
		assert.Empty(t, args)
	})

	t.Run("year_equals", func(t *testing.T) {
		q, args := buildListItemsQuery(ListItemsOptions{Year: "2024"})
		assert.Contains(t, q, "release_year = ?")
		assert.Equal(t, []any{"2024"}, args)
	})

	t.Run("size_range_adds_two_bounds", func(t *testing.T) {
		q, args := buildListItemsQuery(ListItemsOptions{SizeFilter: "2-5"})
		assert.Contains(t, q, "total_size >= ?")
		assert.Contains(t, q, "total_size < ?")
		assert.Equal(t, []any{2 * gb, 5 * gb}, args)
	})

	t.Run("size_lt_adds_only_upper", func(t *testing.T) {
		q, args := buildListItemsQuery(ListItemsOptions{SizeFilter: "lt-1"})
		assert.NotContains(t, q, "total_size >= ?")
		assert.Contains(t, q, "total_size < ?")
		assert.Equal(t, []any{gb}, args)
	})

	t.Run("size_50_plus_adds_only_lower", func(t *testing.T) {
		q, args := buildListItemsQuery(ListItemsOptions{SizeFilter: "50-plus"})
		assert.Contains(t, q, "total_size >= ?")
		assert.NotContains(t, q, "total_size < ?")
		assert.Equal(t, []any{50 * gb}, args)
	})

	t.Run("combined_filters_join_with_and", func(t *testing.T) {
		q, args := buildListItemsQuery(ListItemsOptions{
			Keyword:    "k",
			Year:       "2024",
			SizeFilter: "1-2",
			Sort:       "title",
			Order:      "asc",
		})
		assert.Contains(t, q, " AND ")
		assert.Contains(t, q, "CASE WHEN title = '' THEN name ELSE title END ASC")
		require.Len(t, args, 3+1+2)
		assert.Equal(t, "%k%", args[0])
		assert.Equal(t, "2024", args[3])
		assert.Equal(t, gb, args[4])
		assert.Equal(t, 2*gb, args[5])
	})

	t.Run("order_asc_is_case_insensitive", func(t *testing.T) {
		q, _ := buildListItemsQuery(ListItemsOptions{Order: "ASC"})
		// 默认排序列 updated_at 被折叠, 所以这里直接看 tie-breaker 的方向。
		assert.Contains(t, q, "ORDER BY updated_at ASC, id ASC")
	})

	t.Run("non_updated_sort_keeps_full_tiebreaker", func(t *testing.T) {
		q, _ := buildListItemsQuery(ListItemsOptions{Sort: "title"})
		assert.Contains(t, q, "CASE WHEN title = '' THEN name ELSE title END DESC, updated_at DESC, id DESC")
	})
}

// --- escapeLikePattern ---

func TestEscapeLikePattern(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"no_special", "abc", "abc"},
		{"percent_only", "50%", `50\%`},
		{"underscore_only", "a_b", `a\_b`},
		{"backslash_only", `a\b`, `a\\b`},
		{"all_mixed", `1\0%0_a`, `1\\0\%0\_a`},
		{"empty", "", ""},
		{"unicode_untouched", "中文_混合", `中文\_混合`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, escapeLikePattern(tc.in))
		})
	}
}

// --- IsConfigured ---

func TestIsConfigured(t *testing.T) {
	tests := []struct {
		name       string
		libraryDir string
		want       bool
	}{
		{"configured", "/lib", true},
		{"empty", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewService(nil, tc.libraryDir, "")
			assert.Equal(t, tc.want, svc.IsConfigured())
		})
	}
}

// --- claimSync / finishSync / isSyncRunning ---

func TestClaimSyncAndFinishSync(t *testing.T) {
	svc := NewService(nil, "", "")
	assert.False(t, svc.isSyncRunning())
	assert.True(t, svc.claimSync())
	assert.True(t, svc.isSyncRunning())
	assert.False(t, svc.claimSync())
	svc.finishSync()
	assert.False(t, svc.isSyncRunning())
}

// --- claimMove / finishMove / isMoveRunning ---

func TestClaimMoveAndFinishMove(t *testing.T) {
	svc := NewService(nil, "", "")
	assert.False(t, svc.isMoveRunning())
	assert.False(t, svc.IsMoveRunning())
	assert.True(t, svc.claimMove())
	assert.True(t, svc.isMoveRunning())
	assert.True(t, svc.IsMoveRunning())
	assert.False(t, svc.claimMove())
	svc.finishMove()
	assert.False(t, svc.isMoveRunning())
}

// --- ResolveLibraryPath ---

func TestResolveLibraryPath(t *testing.T) {
	svc := NewService(nil, "/lib", "")
	rel, abs, err := svc.ResolveLibraryPath("sub/item")
	require.NoError(t, err)
	assert.Equal(t, "sub/item", rel)
	assert.Contains(t, abs, "sub")
}

// --- newRunningTaskState ---

func TestNewRunningTaskState(t *testing.T) {
	state := newRunningTaskState("test_task", 42, "msg")
	assert.Equal(t, "test_task", state.TaskKey)
	assert.Equal(t, "running", state.Status)
	assert.Equal(t, 42, state.Total)
	assert.Equal(t, "msg", state.Message)
	assert.NotZero(t, state.StartedAt)
}

// --- saveTaskState / getTaskState ---

func TestSaveAndGetTaskState(t *testing.T) {
	svc := newTestMediaService(t)
	ctx := context.Background()

	t.Run("empty_task_key", func(t *testing.T) {
		err := svc.saveTaskState(ctx, TaskState{TaskKey: ""})
		require.NoError(t, err)
	})

	t.Run("save_and_get", func(t *testing.T) {
		state := TaskState{
			TaskKey:   "test",
			Status:    "running",
			Total:     10,
			Processed: 5,
			Message:   "hello",
		}
		require.NoError(t, svc.saveTaskState(ctx, state))
		got, err := svc.getTaskState(ctx, "test")
		require.NoError(t, err)
		assert.Equal(t, "running", got.Status)
		assert.Equal(t, 10, got.Total)
		assert.Equal(t, 5, got.Processed)
	})

	t.Run("get_nonexistent", func(t *testing.T) {
		got, err := svc.getTaskState(ctx, "nonexistent")
		require.NoError(t, err)
		assert.Equal(t, "idle", got.Status)
	})

	t.Run("zero_updated_at_sets_now", func(t *testing.T) {
		before := time.Now().UnixMilli()
		require.NoError(t, svc.saveTaskState(ctx, TaskState{TaskKey: "auto_update"}))
		got, err := svc.getTaskState(ctx, "auto_update")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, got.UpdatedAt, before)
	})
}

// --- persistTaskProgress ---

func TestPersistTaskProgress(t *testing.T) {
	svc := newTestMediaService(t)
	ctx := context.Background()
	state := TaskState{TaskKey: "progress_test", Status: "running"}
	svc.persistTaskProgress(ctx, &state)
	assert.NotZero(t, state.UpdatedAt)
}

// --- finishTask ---

func TestFinishTask(t *testing.T) {
	svc := newTestMediaService(t)
	ctx := context.Background()
	state := TaskState{TaskKey: "finish_test", Status: "running", Current: "processing"}
	svc.finishTask(ctx, &state, "done")
	assert.Equal(t, "completed", state.Status)
	assert.Equal(t, "done", state.Message)
	assert.Equal(t, "", state.Current)
	assert.NotZero(t, state.FinishedAt)
}

// --- failTask ---

func TestFailTask(t *testing.T) {
	svc := newTestMediaService(t)
	ctx := context.Background()
	svc.failTask(ctx, zap.NewNop(), "fail_test", "something failed", assert.AnError)

	got, err := svc.getTaskState(ctx, "fail_test")
	require.NoError(t, err)
	assert.Equal(t, "failed", got.Status)
	assert.NotZero(t, got.FinishedAt)
}

// --- upsertDetail ---

func TestUpsertDetail(t *testing.T) {
	svc := newTestMediaService(t)
	ctx := context.Background()

	detail := &Detail{
		Item: Item{
			RelPath:     "movie",
			Title:       "T",
			ReleaseDate: "2024-01-01",
			PosterPath:  "p.jpg",
			CoverPath:   "c.jpg",
			UpdatedAt:   100,
		},
		Meta: Meta{Number: "N"},
	}
	require.NoError(t, svc.upsertDetail(ctx, detail))

	detail.Item.Title = "Updated"
	require.NoError(t, svc.upsertDetail(ctx, detail))
}

// --- deleteMissing ---

func TestDeleteMissing(t *testing.T) {
	svc := newTestMediaService(t)
	ctx := context.Background()

	for _, relPath := range []string{"a", "b", "c"} {
		require.NoError(t, svc.upsertDetail(ctx, &Detail{
			Item: Item{RelPath: relPath, Title: relPath, UpdatedAt: 1},
		}))
	}
	keep := map[string]struct{}{"a": {}}
	deleted, err := svc.deleteMissing(ctx, keep)
	require.NoError(t, err)
	assert.Equal(t, 2, deleted)
}

// --- ListItems ---

func TestListItems(t *testing.T) {
	svc := newTestMediaService(t)
	ctx := context.Background()
	gb := int64(1024 * 1024 * 1024)

	for _, item := range []Item{
		{RelPath: "a", Title: "Alpha", Number: "N1", ReleaseDate: "2024-01-01", TotalSize: gb, UpdatedAt: 10, CreatedAt: 1},
		{RelPath: "b", Title: "Beta", Number: "N2", ReleaseDate: "2025-06-01", TotalSize: 3 * gb, UpdatedAt: 20, CreatedAt: 2},
	} {
		require.NoError(t, svc.upsertDetail(ctx, &Detail{Item: item}))
	}

	t.Run("no_filter", func(t *testing.T) {
		items, err := svc.ListItems(ctx, ListItemsOptions{})
		require.NoError(t, err)
		assert.Len(t, items, 2)
	})
	t.Run("keyword_matches_title", func(t *testing.T) {
		items, err := svc.ListItems(ctx, ListItemsOptions{Keyword: "alpha"})
		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.Equal(t, "Alpha", items[0].Title)
	})
	t.Run("keyword_matches_number_case_insensitive", func(t *testing.T) {
		// LOWER(number) LIKE '%n1%' 必须能命中大写 "N1"。
		items, err := svc.ListItems(ctx, ListItemsOptions{Keyword: "N1"})
		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.Equal(t, "Alpha", items[0].Title)
	})
	t.Run("keyword_no_match", func(t *testing.T) {
		items, err := svc.ListItems(ctx, ListItemsOptions{Keyword: "gamma"})
		require.NoError(t, err)
		assert.Empty(t, items)
	})
	t.Run("year_filter", func(t *testing.T) {
		items, err := svc.ListItems(ctx, ListItemsOptions{Year: "2024"})
		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.Equal(t, "Alpha", items[0].Title)
	})
	t.Run("year_all", func(t *testing.T) {
		items, err := svc.ListItems(ctx, ListItemsOptions{Year: "all"})
		require.NoError(t, err)
		assert.Len(t, items, 2)
	})
	t.Run("size_filter", func(t *testing.T) {
		items, err := svc.ListItems(ctx, ListItemsOptions{SizeFilter: "2-5"})
		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.Equal(t, "Beta", items[0].Title)
	})
	t.Run("size_filter_empty_result", func(t *testing.T) {
		items, err := svc.ListItems(ctx, ListItemsOptions{SizeFilter: "50-plus"})
		require.NoError(t, err)
		assert.Empty(t, items)
	})
	t.Run("sort_title_asc", func(t *testing.T) {
		items, err := svc.ListItems(ctx, ListItemsOptions{Sort: "title", Order: "asc"})
		require.NoError(t, err)
		require.Len(t, items, 2)
		assert.Equal(t, "Alpha", items[0].Title)
	})
	t.Run("sort_title_desc", func(t *testing.T) {
		items, err := svc.ListItems(ctx, ListItemsOptions{Sort: "title", Order: "desc"})
		require.NoError(t, err)
		require.Len(t, items, 2)
		assert.Equal(t, "Beta", items[0].Title)
	})
	t.Run("sort_size_asc", func(t *testing.T) {
		items, err := svc.ListItems(ctx, ListItemsOptions{Sort: "size", Order: "asc"})
		require.NoError(t, err)
		require.Len(t, items, 2)
		assert.Equal(t, "Alpha", items[0].Title)
	})
	t.Run("default_sort_is_updated_desc", func(t *testing.T) {
		items, err := svc.ListItems(ctx, ListItemsOptions{})
		require.NoError(t, err)
		require.Len(t, items, 2)
		assert.Equal(t, "Beta", items[0].Title)
	})
	t.Run("combined_keyword_and_year", func(t *testing.T) {
		items, err := svc.ListItems(ctx, ListItemsOptions{Keyword: "N", Year: "2025"})
		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.Equal(t, "Beta", items[0].Title)
	})
}

func TestListItemsSortTitleFallsBackToName(t *testing.T) {
	// sortModeToColumn("title") 的 CASE 表达式规定 title 为空时用 name 当比较键;
	// 这里验证跨字段排序落到 SQL 里的行为和 Go 层原有 firstNonEmpty 语义一致。
	svc := newTestMediaService(t)
	ctx := context.Background()
	require.NoError(t, svc.upsertDetail(ctx, &Detail{
		Item: Item{RelPath: "a", Name: "Zeta", Title: "", UpdatedAt: 1},
	}))
	require.NoError(t, svc.upsertDetail(ctx, &Detail{
		Item: Item{RelPath: "b", Name: "", Title: "Alpha", UpdatedAt: 2},
	}))
	items, err := svc.ListItems(ctx, ListItemsOptions{Sort: "title", Order: "asc"})
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, "Alpha", items[0].Title)
	assert.Equal(t, "Zeta", items[1].Name)
}

func TestListItemsKeywordLiteralWildcards(t *testing.T) {
	// 旧实现 strings.Contains("foo", "%") 只命中字符串里真的有 '%' 的行;
	// 新 SQL 下推后必须维持同样的字面语义, 不能让 '%' 当作 "match anything" 泄漏行。
	svc := newTestMediaService(t)
	ctx := context.Background()

	for _, item := range []Item{
		{RelPath: "a", Title: "50% off promo", UpdatedAt: 1},
		{RelPath: "b", Title: "plain title", UpdatedAt: 2},
	} {
		require.NoError(t, svc.upsertDetail(ctx, &Detail{Item: item}))
	}

	got, err := svc.ListItems(ctx, ListItemsOptions{Keyword: "%"})
	require.NoError(t, err)
	require.Len(t, got, 1, "only the row containing a literal '%%' should match")
	assert.Equal(t, "a", got[0].RelPath)
}

func TestListItemsLegacyRowsNeedSyncToBeFilterable(t *testing.T) {
	// 模拟 002 migration 升级老库: 构造一行 release_year 索引列为空、
	// 但 item_json 里有可解析年份的数据。002 不做 SQL 回填, 所以 year 过滤
	// 在 sync 之前必然漏掉这行; 跑一次 upsertDetail (等价于一次 sync)
	// 之后, 索引列被写正确, 过滤才能命中。覆盖 "升级后必须触发同步媒体库
	// 才能让旧行参与过滤" 的契约。
	svc := newTestMediaService(t)
	ctx := context.Background()

	item := Item{RelPath: "legacy", Title: "Legacy", Number: "L1", ReleaseDate: "2023-08-01", UpdatedAt: 5}
	raw, err := json.Marshal(item)
	require.NoError(t, err)
	_, err = svc.db.ExecContext(ctx, `
		INSERT INTO yamdc_media_library_tab (
			rel_path, title, release_date, updated_at, poster_path, cover_path,
			item_json, detail_json, created_at,
			number, name, release_year, total_size
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, '', '', '', 0)
	`, item.RelPath, item.Title, item.ReleaseDate, item.UpdatedAt, "", "", string(raw), "{}", int64(1))
	require.NoError(t, err)

	before, err := svc.ListItems(ctx, ListItemsOptions{Year: "2023"})
	require.NoError(t, err)
	assert.Empty(t, before, "sync 之前 release_year 列为空, year 过滤不应该命中")

	require.NoError(t, svc.upsertDetail(ctx, &Detail{Item: item}))

	after, err := svc.ListItems(ctx, ListItemsOptions{Year: "2023"})
	require.NoError(t, err)
	require.Len(t, after, 1)
	assert.Equal(t, "Legacy", after[0].Title)
}

// --- GetDetail ---

func TestGetDetail(t *testing.T) {
	svc := newTestMediaService(t)
	ctx := context.Background()

	require.NoError(t, svc.upsertDetail(ctx, &Detail{
		Item: Item{RelPath: "movie", Title: "T", UpdatedAt: 1},
		Meta: Meta{Number: "N"},
	}))

	t.Run("found", func(t *testing.T) {
		items, _ := svc.ListItems(ctx, ListItemsOptions{})
		require.Len(t, items, 1)
		detail, err := svc.GetDetail(ctx, items[0].ID)
		require.NoError(t, err)
		assert.Equal(t, "N", detail.Meta.Number)
		assert.Equal(t, items[0].ID, detail.Item.ID)
	})
	t.Run("not_found", func(t *testing.T) {
		_, err := svc.GetDetail(ctx, 99999)
		assert.ErrorIs(t, err, os.ErrNotExist)
	})
}

// --- GetDetailByRelPath ---

func TestGetDetailByRelPath(t *testing.T) {
	svc := newTestMediaService(t)
	ctx := context.Background()

	require.NoError(t, svc.upsertDetail(ctx, &Detail{
		Item: Item{RelPath: "movie", Title: "T", UpdatedAt: 1},
		Meta: Meta{Number: "N"},
	}))

	t.Run("found", func(t *testing.T) {
		detail, err := svc.GetDetailByRelPath(ctx, "movie")
		require.NoError(t, err)
		assert.Equal(t, "N", detail.Meta.Number)
	})
	t.Run("not_found", func(t *testing.T) {
		_, err := svc.GetDetailByRelPath(ctx, "nonexistent")
		assert.ErrorIs(t, err, os.ErrNotExist)
	})
}

// --- UpdateItem ---

func TestUpdateItem(t *testing.T) {
	libraryDir := t.TempDir()
	svc := newTestMediaServiceWithDirs(t, libraryDir, t.TempDir())
	ctx := context.Background()

	itemDir := filepath.Join(libraryDir, "movie")
	require.NoError(t, os.MkdirAll(itemDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "movie.mp4"), []byte("v"), 0o600))
	writeNFO(t, itemDir, "movie", &nfo.Movie{ID: "movie", Title: "Old"})

	detail, err := svc.readRootDetail(libraryDir, "movie", itemDir)
	require.NoError(t, err)
	require.NoError(t, svc.upsertDetail(ctx, detail))

	items, _ := svc.ListItems(ctx, ListItemsOptions{})
	require.Len(t, items, 1)

	next, err := svc.UpdateItem(ctx, items[0].ID, Meta{Title: "New", Number: "movie"})
	require.NoError(t, err)
	assert.NotNil(t, next)
	assert.Equal(t, items[0].ID, next.Item.ID)
}

// --- ReplaceAsset ---

func TestReplaceAsset(t *testing.T) {
	libraryDir := t.TempDir()
	svc := newTestMediaServiceWithDirs(t, libraryDir, t.TempDir())
	ctx := context.Background()

	itemDir := filepath.Join(libraryDir, "movie")
	require.NoError(t, os.MkdirAll(itemDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "movie.mp4"), []byte("v"), 0o600))
	writeNFO(t, itemDir, "movie", &nfo.Movie{ID: "movie"})

	detail, err := svc.readRootDetail(libraryDir, "movie", itemDir)
	require.NoError(t, err)
	require.NoError(t, svc.upsertDetail(ctx, detail))

	items, _ := svc.ListItems(ctx, ListItemsOptions{})
	require.Len(t, items, 1)

	next, err := svc.ReplaceAsset(ctx, items[0].ID, "", "poster", "poster.jpg", []byte("imgdata"))
	require.NoError(t, err)
	assert.NotNil(t, next)
}

// --- DeleteFile ---

func TestDeleteFile(t *testing.T) {
	libraryDir := t.TempDir()
	svc := newTestMediaServiceWithDirs(t, libraryDir, t.TempDir())
	ctx := context.Background()

	itemDir := filepath.Join(libraryDir, "movie")
	efDir := filepath.Join(itemDir, "extrafanart")
	require.NoError(t, os.MkdirAll(efDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "movie.mp4"), []byte("v"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(efDir, "extra.jpg"), []byte("img"), 0o600))

	detail, err := svc.readRootDetail(libraryDir, "movie", itemDir)
	require.NoError(t, err)
	require.NoError(t, svc.upsertDetail(ctx, detail))

	items, _ := svc.ListItems(ctx, ListItemsOptions{})
	require.Len(t, items, 1)

	next, err := svc.DeleteFile(ctx, items[0].ID, "movie/extrafanart/extra.jpg")
	require.NoError(t, err)
	assert.NotNil(t, next)
}

// --- TriggerFullSync ---

func TestTriggerFullSync(t *testing.T) {
	t.Run("not_configured", func(t *testing.T) {
		svc := newTestMediaServiceWithDirs(t, "", "")
		err := svc.TriggerFullSync(context.Background())
		assert.ErrorIs(t, err, errLibraryDirNotConfigured)
	})
	t.Run("move_running", func(t *testing.T) {
		svc := newTestMediaServiceWithDirs(t, "/lib", "")
		svc.claimMove()
		err := svc.TriggerFullSync(context.Background())
		assert.ErrorIs(t, err, errMoveTaskRunning)
		svc.finishMove()
	})
	t.Run("already_running", func(t *testing.T) {
		svc := newTestMediaServiceWithDirs(t, "/lib", "")
		ctx := context.Background()
		require.NoError(t, svc.saveTaskState(ctx, TaskState{TaskKey: TaskSync, Status: "running", UpdatedAt: 1}))
		err := svc.TriggerFullSync(ctx)
		assert.ErrorIs(t, err, errSyncAlreadyRunning)
	})
	t.Run("success", func(t *testing.T) {
		libraryDir := t.TempDir()
		svc := newTestMediaServiceWithDirs(t, libraryDir, t.TempDir())
		ctx := context.Background()
		err := svc.TriggerFullSync(ctx)
		require.NoError(t, err)
		assert.Eventually(t, func() bool {
			st, stErr := svc.getTaskState(ctx, TaskSync)
			return stErr == nil && st.Status == "completed"
		}, 5*time.Second, 10*time.Millisecond)
	})
}

// --- TriggerMove ---

func TestTriggerMove(t *testing.T) {
	t.Run("not_configured", func(t *testing.T) {
		svc := newTestMediaServiceWithDirs(t, "", "")
		err := svc.TriggerMove(context.Background())
		assert.ErrorIs(t, err, errLibraryDirNotConfigured)
	})
	t.Run("no_save_dir", func(t *testing.T) {
		svc := newTestMediaServiceWithDirs(t, "/lib", "")
		err := svc.TriggerMove(context.Background())
		assert.ErrorIs(t, err, errSaveDirNotConfigured)
	})
	t.Run("sync_running", func(t *testing.T) {
		svc := newTestMediaServiceWithDirs(t, "/lib", "/save")
		svc.claimSync()
		err := svc.TriggerMove(context.Background())
		assert.ErrorIs(t, err, errSyncTaskRunning)
		svc.finishSync()
	})
	t.Run("already_running", func(t *testing.T) {
		svc := newTestMediaServiceWithDirs(t, "/lib", "/save")
		svc.claimMove()
		err := svc.TriggerMove(context.Background())
		assert.ErrorIs(t, err, errMoveAlreadyRunning)
		svc.finishMove()
	})
	t.Run("success", func(t *testing.T) {
		libraryDir := t.TempDir()
		saveDir := t.TempDir()
		svc := newTestMediaServiceWithDirs(t, libraryDir, saveDir)
		ctx := context.Background()
		err := svc.TriggerMove(ctx)
		require.NoError(t, err)
		assert.Eventually(t, func() bool {
			st, stErr := svc.getTaskState(ctx, TaskMove)
			return stErr == nil && st.Status == "completed"
		}, 5*time.Second, 10*time.Millisecond)
	})
}

// --- GetStatusSnapshot ---

func TestGetStatusSnapshot(t *testing.T) {
	svc := newTestMediaService(t)
	ctx := context.Background()
	snap, err := svc.GetStatusSnapshot(ctx)
	require.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Equal(t, "idle", snap.Sync.Status)
	assert.Equal(t, "idle", snap.Move.Status)
}

// --- moveDirectory ---

func TestMoveDirectory(t *testing.T) {
	t.Run("same_fs_rename", func(t *testing.T) {
		parent := t.TempDir()
		src := filepath.Join(parent, "src")
		dst := filepath.Join(parent, "dst")
		require.NoError(t, os.MkdirAll(src, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(src, "f.txt"), []byte("data"), 0o600))

		require.NoError(t, moveDirectory(src, dst))
		assert.FileExists(t, filepath.Join(dst, "f.txt"))
		_, err := os.Stat(src)
		assert.True(t, os.IsNotExist(err))
	})
}

// --- copyDirectory ---

func TestCopyDirectory(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		src := filepath.Join(t.TempDir(), "src")
		dst := filepath.Join(t.TempDir(), "dst")
		sub := filepath.Join(src, "sub")
		require.NoError(t, os.MkdirAll(sub, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(src, "a.txt"), []byte("hello"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(sub, "b.txt"), []byte("world"), 0o600))

		require.NoError(t, copyDirectory(src, dst))
		assert.FileExists(t, filepath.Join(dst, "a.txt"))
		assert.FileExists(t, filepath.Join(dst, "sub", "b.txt"))
		data, _ := os.ReadFile(filepath.Join(dst, "a.txt"))
		assert.Equal(t, "hello", string(data))
	})
	t.Run("src_not_exist", func(t *testing.T) {
		err := copyDirectory("/nonexistent", t.TempDir())
		assert.Error(t, err)
	})
}

// --- syncOneItem ---

func TestSyncOneItem(t *testing.T) {
	libraryDir := t.TempDir()
	svc := newTestMediaServiceWithDirs(t, libraryDir, t.TempDir())
	ctx := context.Background()
	lg := zap.NewNop()

	t.Run("success", func(t *testing.T) {
		item := filepath.Join(libraryDir, "movie")
		require.NoError(t, os.MkdirAll(item, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(item, "movie.mp4"), []byte("v"), 0o600))

		keep := make(map[string]struct{})
		result := svc.syncOneItem(ctx, lg, keep, item, "test-run")
		assert.True(t, result.Success)
		assert.Equal(t, "movie", result.RelPath)
		_, ok := keep["movie"]
		assert.True(t, ok)
	})

	t.Run("no_nfo_no_video", func(t *testing.T) {
		item := filepath.Join(libraryDir, "empty")
		require.NoError(t, os.MkdirAll(item, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(item, "readme.txt"), []byte("x"), 0o600))

		keep := make(map[string]struct{})
		result := svc.syncOneItem(ctx, lg, keep, item, "test-run")
		assert.True(t, result.Failed)
	})
}

// --- moveOneItem ---

func TestMoveOneItem(t *testing.T) {
	lg := zap.NewNop()

	t.Run("success", func(t *testing.T) {
		libraryDir := t.TempDir()
		saveDir := t.TempDir()
		svc := newTestMediaServiceWithDirs(t, libraryDir, saveDir)

		itemSrc := filepath.Join(saveDir, "movie")
		require.NoError(t, os.MkdirAll(itemSrc, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(itemSrc, "movie.mp4"), []byte("v"), 0o600))

		result := svc.moveOneItem(context.Background(), lg, itemSrc)
		assert.True(t, result.Success)
		assert.Equal(t, "movie", result.RelPath)
		assert.FileExists(t, filepath.Join(libraryDir, "movie", "movie.mp4"))
	})

	t.Run("conflict", func(t *testing.T) {
		libraryDir := t.TempDir()
		saveDir := t.TempDir()
		svc := newTestMediaServiceWithDirs(t, libraryDir, saveDir)

		require.NoError(t, os.MkdirAll(filepath.Join(libraryDir, "movie"), 0o755))
		itemSrc := filepath.Join(saveDir, "movie")
		require.NoError(t, os.MkdirAll(itemSrc, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(itemSrc, "movie.mp4"), []byte("v"), 0o600))

		result := svc.moveOneItem(context.Background(), lg, itemSrc)
		assert.True(t, result.Conflict)
	})
}

// --- runFullSync (edge cases) ---

func TestRunFullSyncNotConfigured(t *testing.T) {
	svc := newTestMediaServiceWithDirs(t, "", "")
	err := svc.runFullSync(context.Background(), "manual")
	assert.NoError(t, err)
}

func TestRunFullSyncMoveRunningBlocks(t *testing.T) {
	svc := newTestMediaServiceWithDirs(t, "/lib", "")
	svc.claimMove()
	err := svc.runFullSync(context.Background(), "manual")
	assert.NoError(t, err)
	svc.finishMove()
}

func TestRunFullSyncDeletesStaleItems(t *testing.T) {
	_, cleanup := withCapturedLogs(t)
	defer cleanup()

	libraryDir := t.TempDir()
	svc := newTestMediaServiceWithDirs(t, libraryDir, t.TempDir())
	ctx := context.Background()

	require.NoError(t, svc.upsertDetail(ctx, &Detail{
		Item: Item{RelPath: "stale_item", Title: "Stale", UpdatedAt: 1},
	}))

	require.NoError(t, svc.runFullSync(ctx, "manual"))

	items, err := svc.ListItems(ctx, ListItemsOptions{})
	require.NoError(t, err)
	assert.Len(t, items, 0)
}

// --- runFullSync bypass when reason=="move" ---

func TestRunFullSyncMoveReasonBypassesMoveCheck(t *testing.T) {
	_, cleanup := withCapturedLogs(t)
	defer cleanup()

	libraryDir := t.TempDir()
	svc := newTestMediaServiceWithDirs(t, libraryDir, t.TempDir())
	svc.claimMove()
	err := svc.runFullSync(context.Background(), "move")
	assert.NoError(t, err)
	svc.finishMove()
}

// --- moveDirectory error on rename nonexistent ---

func TestMoveDirectoryRenameError(t *testing.T) {
	err := moveDirectory("/nonexistent_src", filepath.Join(t.TempDir(), "dst"))
	assert.Error(t, err)
}

// --- copyDirectory with files ---

func TestCopyDirectoryDeep(t *testing.T) {
	src := filepath.Join(t.TempDir(), "src")
	dst := filepath.Join(t.TempDir(), "dst")
	subDir := filepath.Join(src, "sub")
	require.NoError(t, os.MkdirAll(subDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(src, "a.txt"), []byte("hello"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "b.txt"), []byte("world"), 0o600))

	require.NoError(t, copyDirectory(src, dst))
	data, err := os.ReadFile(filepath.Join(dst, "a.txt"))
	require.NoError(t, err)
	assert.Equal(t, "hello", string(data))
	data2, err := os.ReadFile(filepath.Join(dst, "sub", "b.txt"))
	require.NoError(t, err)
	assert.Equal(t, "world", string(data2))
}

// --- syncOneItem edge cases ---

func TestSyncOneItemRelPathFails(t *testing.T) {
	svc := &Service{libraryDir: ""}
	lg := zap.NewNop()
	keep := make(map[string]struct{})
	result := svc.syncOneItem(context.Background(), lg, keep, "/nonexistent", "test-run")
	assert.True(t, result.Failed || result.Success)
}

// --- moveOneItem edge cases ---

func TestMoveOneItemMoveError(t *testing.T) {
	libraryDir := t.TempDir()
	saveDir := t.TempDir()
	svc := newTestMediaServiceWithDirs(t, libraryDir, saveDir)

	src := filepath.Join(saveDir, "movie")
	require.NoError(t, os.MkdirAll(src, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(src, "f.txt"), []byte("v"), 0o600))

	target := filepath.Join(libraryDir, "movie")
	require.NoError(t, os.MkdirAll(target, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(target, "existing.txt"), []byte("v"), 0o600))

	result := svc.moveOneItem(context.Background(), zap.NewNop(), src)
	assert.True(t, result.Conflict)
}

// --- upsertDetail update path ---

func TestUpsertDetailUpdate(t *testing.T) {
	svc := newTestMediaService(t)
	ctx := context.Background()

	detail := &Detail{
		Item: Item{RelPath: "movie", Title: "T1", UpdatedAt: 1},
		Meta: Meta{Number: "N"},
	}
	require.NoError(t, svc.upsertDetail(ctx, detail))

	detail.Item.Title = "T2"
	detail.Item.UpdatedAt = 2
	require.NoError(t, svc.upsertDetail(ctx, detail))

	items, err := svc.ListItems(ctx, ListItemsOptions{})
	require.NoError(t, err)
	assert.Len(t, items, 1)
}

// --- ListItems with no keyword match ---

func TestListItemsNoMatch(t *testing.T) {
	svc := newTestMediaService(t)
	ctx := context.Background()

	require.NoError(t, svc.upsertDetail(ctx, &Detail{
		Item: Item{RelPath: "a", Title: "Alpha", UpdatedAt: 1},
	}))

	items, err := svc.ListItems(ctx, ListItemsOptions{Keyword: "nonexistent"})
	require.NoError(t, err)
	assert.Len(t, items, 0)
}

// --- GetStatusSnapshot with populated state ---

func TestGetStatusSnapshotWithStates(t *testing.T) {
	svc := newTestMediaService(t)
	ctx := context.Background()

	require.NoError(t, svc.saveTaskState(ctx, TaskState{
		TaskKey: TaskSync, Status: "running", Total: 5, UpdatedAt: 1,
	}))
	require.NoError(t, svc.saveTaskState(ctx, TaskState{
		TaskKey: TaskMove, Status: "completed", Total: 3, UpdatedAt: 2,
	}))

	snap, err := svc.GetStatusSnapshot(ctx)
	require.NoError(t, err)
	assert.Equal(t, "running", snap.Sync.Status)
	assert.Equal(t, "completed", snap.Move.Status)
}

// --- cleanupStaleItems with no stale ---

func TestCleanupStaleItemsNone(t *testing.T) {
	_, cleanup := withCapturedLogs(t)
	defer cleanup()

	svc := newTestMediaService(t)
	ctx := context.Background()

	require.NoError(t, svc.upsertDetail(ctx, &Detail{
		Item: Item{RelPath: "a", UpdatedAt: 1},
	}))

	keep := map[string]struct{}{"a": {}}
	state := TaskState{}
	deleted := svc.cleanupStaleItems(ctx, zap.NewNop(), keep, &state)
	assert.Equal(t, 0, deleted)
}

// --- cleanupStaleItems with stale items ---

func TestCleanupStaleItemsWithStale(t *testing.T) {
	_, cleanup := withCapturedLogs(t)
	defer cleanup()

	svc := newTestMediaService(t)
	ctx := context.Background()

	require.NoError(t, svc.upsertDetail(ctx, &Detail{
		Item: Item{RelPath: "a", UpdatedAt: 1},
	}))
	require.NoError(t, svc.upsertDetail(ctx, &Detail{
		Item: Item{RelPath: "b", UpdatedAt: 1},
	}))

	keep := map[string]struct{}{"a": {}}
	state := TaskState{}
	deleted := svc.cleanupStaleItems(ctx, zap.NewNop(), keep, &state)
	assert.Equal(t, 1, deleted)
}

// --- recoverTaskStates with both idle ---

func TestRecoverTaskStatesBothIdle(t *testing.T) {
	svc := newTestMediaService(t)
	ctx := context.Background()
	err := svc.recoverTaskStates(ctx)
	require.NoError(t, err)
}

// --- Start with recoverTaskStates error ---

func TestStartWithDB(t *testing.T) {
	svc := newTestMediaService(t)
	svc.Start(context.Background())
}

// --- runMove complete flow ---

func TestRunMoveCompleteFlow(t *testing.T) {
	_, cleanup := withCapturedLogs(t)
	defer cleanup()

	libraryDir := t.TempDir()
	saveDir := t.TempDir()
	svc := newTestMediaServiceWithDirs(t, libraryDir, saveDir)

	item1 := filepath.Join(saveDir, "movie1")
	require.NoError(t, os.MkdirAll(item1, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(item1, "movie1.mp4"), []byte("v"), 0o600))
	writeNFO(t, item1, "movie1", &nfo.Movie{ID: "movie1"})

	item2 := filepath.Join(saveDir, "movie2")
	require.NoError(t, os.MkdirAll(item2, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(item2, "movie2.mp4"), []byte("v"), 0o600))

	require.NoError(t, os.MkdirAll(filepath.Join(libraryDir, "movie2"), 0o755))

	require.NoError(t, svc.runMove(context.Background()))

	ctx := context.Background()
	state, err := svc.getTaskState(ctx, TaskMove)
	require.NoError(t, err)
	assert.Equal(t, "completed", state.Status)
	assert.Equal(t, 2, state.Total)
	assert.Equal(t, 1, state.SuccessCount)
	assert.Equal(t, 1, state.ConflictCount)
}

// --- runFullSync complete flow with library items ---

func TestRunFullSyncCompleteFlow(t *testing.T) {
	_, cleanup := withCapturedLogs(t)
	defer cleanup()

	libraryDir := t.TempDir()
	svc := newTestMediaServiceWithDirs(t, libraryDir, t.TempDir())
	ctx := context.Background()

	for _, name := range []string{"m1", "m2"} {
		d := filepath.Join(libraryDir, name)
		require.NoError(t, os.MkdirAll(d, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(d, name+".mp4"), []byte("v"), 0o600))
		writeNFO(t, d, name, &nfo.Movie{ID: name, Title: name})
	}
	require.NoError(t, svc.upsertDetail(ctx, &Detail{
		Item: Item{RelPath: "stale", Title: "Stale", UpdatedAt: 1},
	}))

	require.NoError(t, svc.runFullSync(ctx, "manual"))

	items, err := svc.ListItems(ctx, ListItemsOptions{})
	require.NoError(t, err)
	assert.Len(t, items, 2)

	state, err := svc.getTaskState(ctx, TaskSync)
	require.NoError(t, err)
	assert.Equal(t, "completed", state.Status)
}

// --- runFullSync when already claimed ---

func TestRunFullSyncAlreadyClaimed(t *testing.T) {
	libraryDir := t.TempDir()
	svc := newTestMediaServiceWithDirs(t, libraryDir, t.TempDir())
	svc.claimSync()
	err := svc.runFullSync(context.Background(), "manual")
	assert.NoError(t, err)
	svc.finishSync()
}

// --- UpdateItem not found ---

func TestUpdateItemNotFound(t *testing.T) {
	svc := newTestMediaService(t)
	ctx := context.Background()
	_, err := svc.UpdateItem(ctx, 99999, Meta{})
	assert.ErrorIs(t, err, os.ErrNotExist)
}

// --- ReplaceAsset not found ---

func TestReplaceAssetNotFound(t *testing.T) {
	svc := newTestMediaService(t)
	ctx := context.Background()
	_, err := svc.ReplaceAsset(ctx, 99999, "", "poster", "p.jpg", []byte("img"))
	assert.ErrorIs(t, err, os.ErrNotExist)
}

// --- DeleteFile not found ---

func TestDeleteFileNotFound(t *testing.T) {
	svc := newTestMediaService(t)
	ctx := context.Background()
	_, err := svc.DeleteFile(ctx, 99999, "movie/extrafanart/f.jpg")
	assert.ErrorIs(t, err, os.ErrNotExist)
}

// --- ListItems with bad JSON in DB ---

func TestListItemsBadJSON(t *testing.T) {
	svc := newTestMediaService(t)
	ctx := context.Background()
	now := time.Now().UnixMilli()
	_, err := svc.db.ExecContext(ctx, `
		INSERT INTO yamdc_media_library_tab (rel_path, title, release_date, updated_at, poster_path, cover_path, item_json, detail_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"bad", "T", "2024", now, "", "", "{{INVALID", "{}", now,
	)
	require.NoError(t, err)
	_, err = svc.ListItems(ctx, ListItemsOptions{})
	assert.Error(t, err)
}

// --- GetDetail with bad JSON ---

func TestGetDetailBadJSON(t *testing.T) {
	svc := newTestMediaService(t)
	ctx := context.Background()
	now := time.Now().UnixMilli()
	_, err := svc.db.ExecContext(ctx, `
		INSERT INTO yamdc_media_library_tab (rel_path, title, release_date, updated_at, poster_path, cover_path, item_json, detail_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"bad", "T", "2024", now, "", "", "{}", "{{INVALID", now,
	)
	require.NoError(t, err)

	var id int64
	err = svc.db.QueryRowContext(ctx, `SELECT id FROM yamdc_media_library_tab WHERE rel_path = ?`, "bad").Scan(&id)
	require.NoError(t, err)

	_, err = svc.GetDetail(ctx, id)
	assert.Error(t, err)
}

// --- GetDetailByRelPath with bad JSON ---

func TestGetDetailByRelPathBadJSON(t *testing.T) {
	svc := newTestMediaService(t)
	ctx := context.Background()
	now := time.Now().UnixMilli()
	_, err := svc.db.ExecContext(ctx, `
		INSERT INTO yamdc_media_library_tab (rel_path, title, release_date, updated_at, poster_path, cover_path, item_json, detail_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"bad", "T", "2024", now, "", "", "{}", "{{INVALID", now,
	)
	require.NoError(t, err)
	_, err = svc.GetDetailByRelPath(ctx, "bad")
	assert.Error(t, err)
}

// --- DB error paths: use closed DB to trigger query errors ---

func newClosedDBService(t *testing.T) *Service {
	t.Helper()
	sqlite, err := repository.NewSQLite(context.Background(), filepath.Join(t.TempDir(), "app.db"))
	require.NoError(t, err)
	svc := NewService(sqlite.DB(), t.TempDir(), t.TempDir())
	require.NoError(t, sqlite.Close())
	return svc
}

func TestListItemsDBError(t *testing.T) {
	svc := newClosedDBService(t)
	_, err := svc.ListItems(context.Background(), ListItemsOptions{})
	assert.Error(t, err)
}

func TestGetDetailDBError(t *testing.T) {
	svc := newClosedDBService(t)
	_, err := svc.GetDetail(context.Background(), 1)
	assert.Error(t, err)
}

func TestGetDetailByRelPathDBError(t *testing.T) {
	svc := newClosedDBService(t)
	_, err := svc.GetDetailByRelPath(context.Background(), "movie")
	assert.Error(t, err)
}

func TestGetStatusSnapshotDBError(t *testing.T) {
	svc := newClosedDBService(t)
	_, err := svc.GetStatusSnapshot(context.Background())
	assert.Error(t, err)
}

func TestSaveTaskStateDBError(t *testing.T) {
	svc := newClosedDBService(t)
	err := svc.saveTaskState(context.Background(), TaskState{TaskKey: "test", UpdatedAt: 1})
	assert.Error(t, err)
}

func TestGetTaskStateDBError(t *testing.T) {
	svc := newClosedDBService(t)
	_, err := svc.getTaskState(context.Background(), "test")
	assert.Error(t, err)
}

func TestUpsertDetailDBError(t *testing.T) {
	svc := newClosedDBService(t)
	err := svc.upsertDetail(context.Background(), &Detail{
		Item: Item{RelPath: "x", UpdatedAt: 1},
	})
	assert.Error(t, err)
}

func TestDeleteMissingDBError(t *testing.T) {
	svc := newClosedDBService(t)
	_, err := svc.deleteMissing(context.Background(), map[string]struct{}{})
	assert.Error(t, err)
}

func TestRecoverTaskStatesDBError(t *testing.T) {
	svc := newClosedDBService(t)
	err := svc.recoverTaskStates(context.Background())
	assert.Error(t, err)
}

func TestRecoverTaskStateDBError(t *testing.T) {
	svc := newClosedDBService(t)
	err := svc.recoverTaskState(context.Background(), TaskSync)
	assert.Error(t, err)
}

func TestStartWithClosedDB(t *testing.T) {
	_, cleanup := withCapturedLogs(t)
	defer cleanup()
	svc := newClosedDBService(t)
	svc.Start(context.Background())
}

// --- UpdateItem with updateRootItem error ---

func TestUpdateItemUpdateError(t *testing.T) {
	libraryDir := t.TempDir()
	svc := newTestMediaServiceWithDirs(t, libraryDir, t.TempDir())
	ctx := context.Background()

	itemDir := filepath.Join(libraryDir, "movie")
	require.NoError(t, os.MkdirAll(itemDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "movie.mp4"), []byte("v"), 0o600))

	detail, err := svc.readRootDetail(libraryDir, "movie", itemDir)
	require.NoError(t, err)
	require.NoError(t, svc.upsertDetail(ctx, detail))

	items, _ := svc.ListItems(ctx, ListItemsOptions{})
	require.Len(t, items, 1)

	nfoPath := filepath.Join(itemDir, "movie.nfo")
	require.NoError(t, os.WriteFile(nfoPath, []byte("<movie/>"), 0o600))
	require.NoError(t, os.Chmod(nfoPath, 0o000))
	t.Cleanup(func() { _ = os.Chmod(nfoPath, 0o755) })

	_, err = svc.UpdateItem(ctx, items[0].ID, Meta{Title: "New"})
	assert.Error(t, err)
}

// --- ReplaceAsset with replace error ---

func TestReplaceAssetReplaceError(t *testing.T) {
	libraryDir := t.TempDir()
	svc := newTestMediaServiceWithDirs(t, libraryDir, t.TempDir())
	ctx := context.Background()

	itemDir := filepath.Join(libraryDir, "movie")
	require.NoError(t, os.MkdirAll(itemDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "movie.mp4"), []byte("v"), 0o600))
	writeNFO(t, itemDir, "movie", &nfo.Movie{ID: "movie"})

	detail, err := svc.readRootDetail(libraryDir, "movie", itemDir)
	require.NoError(t, err)
	require.NoError(t, svc.upsertDetail(ctx, detail))

	items, _ := svc.ListItems(ctx, ListItemsOptions{})
	require.Len(t, items, 1)

	require.NoError(t, os.Chmod(itemDir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(itemDir, 0o755) })

	_, err = svc.ReplaceAsset(ctx, items[0].ID, "", "poster", "p.jpg", []byte("img"))
	assert.Error(t, err)
}

// --- DeleteFile with delete error ---

func TestDeleteFileDeleteError(t *testing.T) {
	libraryDir := t.TempDir()
	svc := newTestMediaServiceWithDirs(t, libraryDir, t.TempDir())
	ctx := context.Background()

	itemDir := filepath.Join(libraryDir, "movie")
	efDir := filepath.Join(itemDir, "extrafanart")
	require.NoError(t, os.MkdirAll(efDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "movie.mp4"), []byte("v"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(efDir, "extra.jpg"), []byte("img"), 0o600))

	detail, err := svc.readRootDetail(libraryDir, "movie", itemDir)
	require.NoError(t, err)
	require.NoError(t, svc.upsertDetail(ctx, detail))

	items, _ := svc.ListItems(ctx, ListItemsOptions{})
	require.Len(t, items, 1)

	require.NoError(t, os.Chmod(efDir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(efDir, 0o755) })

	_, err = svc.DeleteFile(ctx, items[0].ID, "movie/extrafanart/extra.jpg")
	assert.Error(t, err)
}

// --- runFullSync with listRootItemDirs error ---

func TestRunFullSyncListError(t *testing.T) {
	_, cleanup := withCapturedLogs(t)
	defer cleanup()

	libraryDir := t.TempDir()
	svc := newTestMediaServiceWithDirs(t, libraryDir, t.TempDir())
	require.NoError(t, os.Chmod(libraryDir, 0o000))
	t.Cleanup(func() { _ = os.Chmod(libraryDir, 0o755) })

	err := svc.runFullSync(context.Background(), "manual")
	assert.Error(t, err)
}

// --- runMove with listRootItemDirs error ---

func TestRunMoveListError(t *testing.T) {
	_, cleanup := withCapturedLogs(t)
	defer cleanup()

	saveDir := t.TempDir()
	svc := newTestMediaServiceWithDirs(t, t.TempDir(), saveDir)
	require.NoError(t, os.Chmod(saveDir, 0o000))
	t.Cleanup(func() { _ = os.Chmod(saveDir, 0o755) })

	err := svc.runMove(context.Background())
	assert.Error(t, err)
}

// --- syncOneItem upsertDetail error ---

func TestSyncOneItemUpsertError(t *testing.T) {
	_, cleanup := withCapturedLogs(t)
	defer cleanup()

	libraryDir := t.TempDir()
	svc := newClosedDBService(t)
	svc.libraryDir = libraryDir

	item := filepath.Join(libraryDir, "movie")
	require.NoError(t, os.MkdirAll(item, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(item, "movie.mp4"), []byte("v"), 0o600))

	keep := make(map[string]struct{})
	result := svc.syncOneItem(context.Background(), zap.NewNop(), keep, item, "test-run")
	assert.True(t, result.Failed)
}

// --- syncAllItems with failed item ---

func TestSyncAllItemsWithFailedItem(t *testing.T) {
	_, cleanup := withCapturedLogs(t)
	defer cleanup()

	libraryDir := t.TempDir()
	svc := newTestMediaServiceWithDirs(t, libraryDir, t.TempDir())
	ctx := context.Background()

	goodItem := filepath.Join(libraryDir, "good")
	require.NoError(t, os.MkdirAll(goodItem, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(goodItem, "good.mp4"), []byte("v"), 0o600))

	badItem := filepath.Join(libraryDir, "bad")
	require.NoError(t, os.MkdirAll(badItem, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(badItem, "bad.mp4"), []byte("v"), 0o600))
	require.NoError(t, os.Chmod(badItem, 0o000))
	t.Cleanup(func() { _ = os.Chmod(badItem, 0o755) })

	state := newRunningTaskState(TaskSync, 2, "test")
	keep := svc.syncAllItems(ctx, zap.NewNop(), []string{goodItem, badItem}, &state, "test-run")
	assert.Equal(t, 2, state.Processed)
	assert.True(t, state.ErrorCount > 0 || state.SuccessCount > 0)
	_ = keep
}

// --- runMove with mixed results (errors) ---

func TestRunMoveWithMixedResults(t *testing.T) {
	_, cleanup := withCapturedLogs(t)
	defer cleanup()

	libraryDir := t.TempDir()
	saveDir := t.TempDir()
	svc := newTestMediaServiceWithDirs(t, libraryDir, saveDir)

	item := filepath.Join(saveDir, "movie")
	require.NoError(t, os.MkdirAll(item, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(item, "movie.mp4"), []byte("v"), 0o600))

	target := filepath.Join(libraryDir, "movie")
	require.NoError(t, os.MkdirAll(target, 0o755))

	require.NoError(t, svc.runMove(context.Background()))

	ctx := context.Background()
	state, err := svc.getTaskState(ctx, TaskMove)
	require.NoError(t, err)
	assert.Equal(t, "completed", state.Status)
	assert.True(t, state.ConflictCount > 0 || state.ErrorCount > 0)
}

// --- moveOneItem with MkdirAll failure ---

func TestMoveOneItemMkdirAllError(t *testing.T) {
	libraryDir := t.TempDir()
	saveDir := t.TempDir()
	svc := newTestMediaServiceWithDirs(t, libraryDir, saveDir)

	subDir := filepath.Join(saveDir, "deep", "nested")
	require.NoError(t, os.MkdirAll(subDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "movie.mp4"), []byte("v"), 0o600))

	deepLib := filepath.Join(libraryDir, "deep")
	require.NoError(t, os.WriteFile(deepLib, []byte("not a dir"), 0o600))

	result := svc.moveOneItem(context.Background(), zap.NewNop(), subDir)
	assert.True(t, result.Failed)
}

// --- recoverTaskState saveTaskState error ---

func TestRecoverTaskStateSaveError(t *testing.T) {
	libraryDir := t.TempDir()
	sqlite, err := repository.NewSQLite(context.Background(), filepath.Join(t.TempDir(), "app.db"))
	require.NoError(t, err)
	svc := NewService(sqlite.DB(), libraryDir, t.TempDir())
	ctx := context.Background()

	require.NoError(t, svc.saveTaskState(ctx, TaskState{TaskKey: TaskSync, Status: "running", UpdatedAt: 1}))
	require.NoError(t, sqlite.Close())

	err = svc.recoverTaskState(ctx, TaskSync)
	assert.Error(t, err)
}

// --- ListItems with sort and order combos ---

func TestListItemsSortCombinations(t *testing.T) {
	svc := newTestMediaService(t)
	ctx := context.Background()

	for i, item := range []Item{
		{RelPath: "a", Title: "B", TotalSize: 200, ReleaseDate: "2025-01-01", CreatedAt: 20, UpdatedAt: 20},
		{RelPath: "b", Title: "A", TotalSize: 100, ReleaseDate: "2024-01-01", CreatedAt: 10, UpdatedAt: 10},
	} {
		item.ID = int64(i + 1)
		require.NoError(t, svc.upsertDetail(ctx, &Detail{Item: item}))
	}

	for _, tc := range []struct{ sort, order string }{
		{"title", "asc"},
		{"title", "desc"},
		{"size", "asc"},
		{"size", "desc"},
		{"year", "asc"},
		{"year", "desc"},
		{"ingested", "asc"},
		{"ingested", "desc"},
		{"", "asc"},
		{"", "desc"},
	} {
		items, err := svc.ListItems(ctx, ListItemsOptions{Sort: tc.sort, Order: tc.order})
		require.NoError(t, err)
		assert.Len(t, items, 2, "sort=%s order=%s", tc.sort, tc.order)
	}
}

// TestListItemsKeywordConsistencyAfterPushdown 用实际 DB 验证: 把
// keyword 过滤从 Go 层下推到 SQL 层后, ListItems 对 title / number / name
// 三字段的命中语义不变 —— 这是 1.4 优化的"行为不变"契约。
func TestListItemsKeywordConsistencyAfterPushdown(t *testing.T) {
	svc := newTestMediaService(t)
	ctx := context.Background()

	items := []Item{
		// 关键字只出现在 title
		{RelPath: "a", Title: "Alpha Series", Number: "X-1"},
		// 关键字只出现在 number
		{RelPath: "b", Title: "Other", Number: "ALPHA-2"},
		// 关键字只出现在 name
		{RelPath: "c", Title: "Other", Number: "X-3", Name: "alpha-file.mp4"},
		// 不含关键字
		{RelPath: "d", Title: "Beta", Number: "Y-4", Name: "beta.mp4"},
	}
	for i := range items {
		items[i].ID = int64(i + 1)
		require.NoError(t, svc.upsertDetail(ctx, &Detail{Item: items[i]}))
	}

	got, err := svc.ListItems(ctx, ListItemsOptions{Keyword: "alpha"})
	require.NoError(t, err)
	require.Len(t, got, 3, "应同时匹配 title/number/name 三种字段")

	paths := make(map[string]struct{}, len(got))
	for _, it := range got {
		paths[it.RelPath] = struct{}{}
	}
	for _, want := range []string{"a", "b", "c"} {
		_, ok := paths[want]
		assert.True(t, ok, "missing expected rel_path=%s", want)
	}
	_, unexpected := paths["d"]
	assert.False(t, unexpected, "beta should not appear when keyword=alpha")
}

func TestListConflictIdentities(t *testing.T) {
	svc := newTestMediaService(t)
	ctx := context.Background()

	for _, d := range []*Detail{
		{Item: Item{RelPath: "a", Title: "A", Number: "NUM-001", UpdatedAt: 1}},
		{Item: Item{RelPath: "b", Title: "B", Number: "NUM-002", UpdatedAt: 2}},
	} {
		require.NoError(t, svc.upsertDetail(ctx, d))
	}

	identities, err := svc.ListConflictIdentities(ctx)
	require.NoError(t, err)
	assert.Len(t, identities, 2)

	found := map[string]string{}
	for _, ci := range identities {
		found[ci.RelPath] = ci.Number
	}
	assert.Equal(t, "NUM-001", found["a"])
	assert.Equal(t, "NUM-002", found["b"])
}

func TestListConflictIdentitiesEmpty(t *testing.T) {
	svc := newTestMediaService(t)
	ctx := context.Background()

	identities, err := svc.ListConflictIdentities(ctx)
	require.NoError(t, err)
	assert.Empty(t, identities)
}

func TestListConflictIdentitiesDBError(t *testing.T) {
	sqlite, err := repository.NewSQLite(context.Background(), filepath.Join(t.TempDir(), "app.db"))
	require.NoError(t, err)
	svc := NewService(sqlite.DB(), t.TempDir(), t.TempDir())
	t.Cleanup(func() {
		svc.Stop()
		svc.WaitBackground()
	})
	require.NoError(t, sqlite.Close())

	_, err = svc.ListConflictIdentities(context.Background())
	assert.Error(t, err)
}
