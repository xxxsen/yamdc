package repository

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestApplyMigrationsBumpsUserVersion 覆盖正常 case: 首次 open DB 后,
// PRAGMA user_version 必须等于最后一个 migration 文件的序号。
func TestApplyMigrationsBumpsUserVersion(t *testing.T) {
	ctx := context.Background()
	sqlite, err := NewSQLite(ctx, filepath.Join(t.TempDir(), "app.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlite.Close() })

	migs, err := loadMigrations()
	require.NoError(t, err)
	require.NotEmpty(t, migs)
	wantVersion := migs[len(migs)-1].version

	got, err := readUserVersion(ctx, sqlite.DB())
	require.NoError(t, err)
	assert.Equal(t, wantVersion, got)
}

// TestApplyMigrationsIsIdempotent 覆盖正常 case: 同一个 DB 文件被反复 open
// 多次 (模拟进程重启), user_version 已经到达最新时, applyMigrations 必须是 no-op,
// 不会因为 ALTER TABLE 重放而 fail。
func TestApplyMigrationsIsIdempotent(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "app.db")

	sqlite, err := NewSQLite(ctx, path)
	require.NoError(t, err)
	require.NoError(t, sqlite.Close())

	sqlite2, err := NewSQLite(ctx, path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlite2.Close() })

	migs, err := loadMigrations()
	require.NoError(t, err)
	got, err := readUserVersion(ctx, sqlite2.DB())
	require.NoError(t, err)
	assert.Equal(t, migs[len(migs)-1].version, got)
}

// TestApplyMigrationsRunsPendingOnUpgrade 覆盖 "老库升级" 路径 (边缘 case):
// 把 user_version 手动回退到 0, 代表老版本只跑过 001_init 且没有 user_version 记录,
// 重新打开 DB 后 applyMigrations 必须把 002 重放、把 user_version 顶上去。
//
// 002 migration 里面用的是 ALTER TABLE ADD COLUMN (非幂等) +
// CREATE INDEX IF NOT EXISTS (幂等)。回退 user_version 时要同步把 002 加的列
// 扔掉, 不然 ALTER TABLE 会直接报 duplicate column, 测试等效于模拟了升级。
func TestApplyMigrationsRunsPendingOnUpgrade(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "app.db")

	sqlite, err := NewSQLite(ctx, path)
	require.NoError(t, err)
	db := sqlite.DB()

	// 002 加的 3 个 index 覆盖了 3 列, 先 drop 掉, 不然下面 DROP COLUMN
	// 会被 "column still referenced by index" 挡住。
	for _, idx := range []string{
		"idx_yamdc_media_library_number",
		"idx_yamdc_media_library_release_year",
		"idx_yamdc_media_library_total_size",
	} {
		_, err = db.ExecContext(ctx, "DROP INDEX IF EXISTS "+idx)
		require.NoError(t, err)
	}
	// 002 之前的表结构是没有这几列的, 模拟老库需要把它们重建成 "还没升级" 的样子。
	// SQLite 支持 DROP COLUMN (3.35+), glebarez/go-sqlite 用的 CGo-free 端口已经内置。
	for _, col := range []string{"number", "name", "release_year", "total_size"} {
		_, err = db.ExecContext(ctx, "ALTER TABLE yamdc_media_library_tab DROP COLUMN "+col)
		require.NoError(t, err)
	}
	_, err = db.ExecContext(ctx, "PRAGMA user_version = 1")
	require.NoError(t, err)
	require.NoError(t, sqlite.Close())

	sqlite2, err := NewSQLite(ctx, path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlite2.Close() })

	migs, err := loadMigrations()
	require.NoError(t, err)
	got, err := readUserVersion(ctx, sqlite2.DB())
	require.NoError(t, err)
	assert.Equal(t, migs[len(migs)-1].version, got, "user_version should be bumped after re-open")

	var count int
	err = sqlite2.DB().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM pragma_table_info('yamdc_media_library_tab')
		WHERE name IN ('number','name','release_year','total_size')
	`).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 4, count, "002 migration columns should be present after upgrade")
}

// TestParseMigrationVersion 覆盖正常 case + 异常 case + 边缘 case: 正确的
// NNN_ 前缀、没有下划线的名字、前缀不是数字、前缀是 0 / 负数。
func TestParseMigrationVersion(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{"valid_single_digit", "1_init.sql", 1, false},
		{"valid_multi_digit", "042_feature.sql", 42, false},
		{"missing_underscore", "001init.sql", 0, true},
		{"leading_underscore", "_001.sql", 0, true},
		{"non_numeric_prefix", "abc_init.sql", 0, true},
		{"zero_prefix", "000_noop.sql", 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseMigrationVersion(tc.input)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestLoadMigrationsSorted 验证 loadMigrations 按版本号升序返回, 即使文件名
// 不是字典序排好的 (比如 10 排在 9 前面时字典序会错序)。覆盖边缘 case。
func TestLoadMigrationsSorted(t *testing.T) {
	migs, err := loadMigrations()
	require.NoError(t, err)
	for i := 1; i < len(migs); i++ {
		assert.Less(t, migs[i-1].version, migs[i].version,
			"loadMigrations must return versions in ascending order")
	}
}

// TestReadUserVersionOnFreshDB 覆盖边缘 case: 一个没跑过 applyMigrations 的
// 裸 sqlite DB, user_version 应该是 0。
func TestReadUserVersionOnFreshDB(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "bare.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	got, err := readUserVersion(ctx, db)
	require.NoError(t, err)
	assert.Equal(t, 0, got)
}

// TestSplitSQLStatementsStripsLineComments 覆盖注释里含 ";" 的正常路径:
// SQL 行注释 (--) 里的分号不应该被误认成语句分隔符, 两条 CREATE 依然按
// 期望各自独立。历史上 002 / 003 迁移都因为中文注释里带分号踩过同一个坑,
// 这个测试确保修好之后不会回退。
func TestSplitSQLStatementsStripsLineComments(t *testing.T) {
	input := `-- 注释里有分号; 和中文标点也没关系;
CREATE TABLE foo (id INTEGER);
-- 另一段注释; 继续带分号;
CREATE TABLE bar (id INTEGER);`
	got := splitSQLStatements(input)
	require.Len(t, got, 2)
	assert.Contains(t, got[0], "CREATE TABLE foo")
	assert.Contains(t, got[1], "CREATE TABLE bar")
}

// TestSplitSQLStatementsSkipsBlank 覆盖边缘 case: 连续分号 / 纯注释行 / 纯空白
// 不应该产生空语句 (否则 ExecContext 会因为 "" 报语法错)。
func TestSplitSQLStatementsSkipsBlank(t *testing.T) {
	input := `;;
-- only a comment;
   ;
CREATE TABLE foo (id INTEGER);`
	got := splitSQLStatements(input)
	require.Len(t, got, 1)
	assert.Contains(t, got[0], "CREATE TABLE foo")
}

// TestSplitSQLStatementsEmpty 覆盖异常 case: 空输入不应该 panic 也不应该
// 返回 [""].
func TestSplitSQLStatementsEmpty(t *testing.T) {
	assert.Empty(t, splitSQLStatements(""))
	assert.Empty(t, splitSQLStatements("-- just a comment\n"))
}
