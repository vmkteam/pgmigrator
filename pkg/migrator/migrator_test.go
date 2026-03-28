package migrator

import (
	"context"
	"os"
	"testing"

	"github.com/go-pg/pg/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	testDB       *pg.DB
	testConfig   Config
	testMigrator *Migrator
	dbConn       = env("DB_CONN", "postgres://postgres:postgres@localhost:5432/pgmigrator?sslmode=disable")
)

func env(v, def string) string {
	if r := os.Getenv(v); r != "" {
		return r
	}

	return def
}

func NewTestDB() *pg.DB {
	ops, err := pg.ParseURL(dbConn)
	if err != nil {
		panic(err)
	}
	return pg.Connect(ops)
}

func TestMain(m *testing.M) {
	testDB = NewTestDB()
	testConfig = NewDefaultConfig()
	testMigrator = NewMigrator(testDB, testConfig, "testdata")
	os.Exit(m.Run())
}

func TestMigrator_Plan(t *testing.T) {
	ctx := context.Background()

	err := recreateSchema()
	require.NoError(t, err)

	want := []string{
		"2022-12-12-01-create-table-statuses.sql",
		"2022-12-12-02-create-table-news.sql",
		"2022-12-12-03-add-comments-news-NONTR.sql",
		"2022-12-13-01-create-categories-table.sql",
		"2022-12-13-02-create-tags-table.sql",
	}
	got, err := testMigrator.Plan(ctx)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestMigrator_readFiles(t *testing.T) {
	want := []string{
		"2022-12-12-01-create-table-statuses.sql",
		"2022-12-12-02-create-table-news.sql",
		"2022-12-12-03-add-comments-news-NONTR.sql",
		"2022-12-13-01-create-categories-table.sql",
		"2022-12-13-02-create-tags-table.sql",
	}

	filenames, err := testMigrator.readAllFiles()
	require.NoError(t, err)
	assert.Equal(t, want, filenames)
}

func TestMigrator_compareFilenames(t *testing.T) {
	dirFiles := []string{
		"2022-12-12-01-create-table-statuses.sql",
		"2022-12-12-02-create-table-news.sql",
		"2022-12-12-03-add-comments-news-NONTR.sql",
		"2022-12-13-01-create-categories-table.sql",
		"2022-12-13-02-create-tags-table.sql",
	}
	completedFiles := []string{"2022-12-12-01-create-table-statuses.sql"}
	want := []string{
		"2022-12-12-02-create-table-news.sql",
		"2022-12-12-03-add-comments-news-NONTR.sql",
		"2022-12-13-01-create-categories-table.sql",
		"2022-12-13-02-create-tags-table.sql",
	}
	res := testMigrator.removeCompleted(dirFiles, completedFiles)
	assert.Equal(t, want, res)
}

func TestNewMigration(t *testing.T) {
	res, err := NewMigration(testMigrator.rootDir, "2022-12-12-01-create-table-statuses.sql")
	require.NoError(t, err)
	assert.Equal(t, Migration{
		Filename: "2022-12-12-01-create-table-statuses.sql",
		Data: []byte(`CREATE TABLE "statuses"
(
    "statusId" SERIAL       NOT NULL,
    "title"    varchar(255) NOT NULL,
    "alias"    varchar(64)  NOT NULL,
    CONSTRAINT "statuses_pkey" PRIMARY KEY ("statusId"),
    CONSTRAINT "statuses_alias_key" UNIQUE ("alias")
);
`),
		Md5Sum:        "463fe73a85e13dd55fe210904ec19d7c",
		Transactional: true,
	}, res)
}

func TestMigrator_prepareMigrationsToRun(t *testing.T) {
	dirFiles := []string{
		"2022-12-12-01-create-table-statuses.sql",
		"2022-12-12-02-create-table-news.sql",
		"2022-12-12-03-add-comments-news-NONTR.sql",
		"2022-12-13-01-create-categories-table.sql",
		"2022-12-13-02-create-tags-table.sql",
	}
	want := []Migration{
		{Filename: "2022-12-12-01-create-table-statuses.sql", Transactional: true},
		{Filename: "2022-12-12-02-create-table-news.sql", Transactional: true},
		{Filename: "2022-12-12-03-add-comments-news-NONTR.sql", Transactional: false},
		{Filename: "2022-12-13-01-create-categories-table.sql", Transactional: true},
		{Filename: "2022-12-13-02-create-tags-table.sql", Transactional: true},
	}

	res, err := testMigrator.newMigrations(dirFiles)
	require.NoError(t, err)
	require.Len(t, res, len(want))

	for i, m := range res {
		assert.Equal(t, want[i].Filename, m.Filename)
		assert.Equal(t, want[i].Transactional, m.Transactional)
	}
}

func TestMigrator_applyNonTransactionalMigration(t *testing.T) {
	t.Skip()
	ctx := context.Background()

	err := recreateSchema()
	require.NoError(t, err)

	_, err = testMigrator.db.ExecContext(ctx, `create table news (id text);`)
	require.NoError(t, err)

	mg, err := NewMigration(testMigrator.rootDir, "2022-12-12-03-add-comments-news-NONTR.sql")
	require.NoError(t, err)

	err = testMigrator.applyNonTransactionalMigration(ctx, mg)
	require.NoError(t, err)

	var pm PgMigration
	err = testMigrator.db.ModelContext(ctx, &pm).Where(`"filename" = ?`, mg.Filename).Select()
	require.NoError(t, err)
	assert.NotEmpty(t, pm.FinishedAt)
}

func TestMigrator_applyMigration(t *testing.T) {
	t.Skip()
	ctx := context.Background()

	mg, err := NewMigration(testMigrator.rootDir, "2022-12-12-01-create-table-statuses.sql")
	require.NoError(t, err)

	err = testMigrator.applyMigration(ctx, mg)
	require.NoError(t, err)

	var pm PgMigration
	err = testMigrator.db.ModelContext(ctx, &pm).Where(`"filename" = ?`, mg.Filename).Select()
	require.NoError(t, err)
	assert.NotEmpty(t, pm.FinishedAt)
}

func TestMigrator_Run(t *testing.T) {
	ctx := context.Background()

	err := recreateSchema()
	require.NoError(t, err)

	err = execRun(ctx, t)
	require.NoError(t, err)
}

func execRun(ctx context.Context, t *testing.T) error {
	filenames, err := testMigrator.Plan(ctx)
	if err != nil {
		return err
	}

	ch := make(chan string)
	go readFromCh(ch, t)

	return testMigrator.Run(ctx, filenames, ch)
}

func TestMigrator_Last(t *testing.T) {
	ctx := context.Background()

	t.Run("empty list", func(t *testing.T) {
		err := recreateSchema()
		require.NoError(t, err)

		list, err := testMigrator.Last(ctx, 5)
		require.NoError(t, err)
		assert.Empty(t, list)
	})

	t.Run("all applied", func(t *testing.T) {
		err := recreateSchema()
		require.NoError(t, err)
		err = execRun(ctx, t)
		require.NoError(t, err)

		list, err := testMigrator.Last(ctx, 5)
		require.NoError(t, err)
		assert.Len(t, list, 5)
	})

	t.Run("3 last migrations", func(t *testing.T) {
		list, err := testMigrator.Last(ctx, 3)
		require.NoError(t, err)
		assert.Len(t, list, 3)
	})
}

func TestMigrator_Redo(t *testing.T) {
	ctx := context.Background()

	t.Run("no applied migrations", func(t *testing.T) {
		err := recreateSchema()
		require.NoError(t, err)

		ch := make(chan string)
		go readFromCh(ch, t)

		pm, err := testMigrator.Redo(ctx, ch)
		require.EqualError(t, err, "applied migrations were not found")
		assert.Nil(t, pm)
	})

	t.Run("redo last applied", func(t *testing.T) {
		err := recreateSchema()
		require.NoError(t, err)

		err = execRun(ctx, t)
		require.NoError(t, err)

		_, err = testDB.Exec("DROP TABLE tags CASCADE;")
		require.NoError(t, err)

		ch := make(chan string)
		go readFromCh(ch, t)

		pm, err := testMigrator.Redo(ctx, ch)
		require.NoError(t, err)
		assert.Equal(t, &PgMigration{
			ID:            pm.ID,
			Filename:      "2022-12-13-02-create-tags-table.sql",
			StartedAt:     pm.StartedAt,
			FinishedAt:    pm.FinishedAt,
			Transactional: true,
			Md5sum:        "d10bca7f78e847d3d4e71003b31a54a6",
		}, pm)
	})
}

func TestMigrator_dryRunMigrations(t *testing.T) {
	ctx := context.Background()

	err := recreateSchema()
	require.NoError(t, err)
	err = testMigrator.createMigratorTable(ctx)
	require.NoError(t, err)

	dirFiles := []string{
		"2022-12-12-01-create-table-statuses.sql",
		"2022-12-12-02-create-table-news.sql",
	}
	mm, err := testMigrator.newMigrations(dirFiles)
	require.NoError(t, err)

	ch := make(chan string)
	go readFromCh(ch, t)
	err = testMigrator.dryRunMigrations(ctx, mm, ch)
	require.NoError(t, err)
}

func TestMigrator_DryRun(t *testing.T) {
	ctx := context.Background()

	t.Run("transactional migrations", func(t *testing.T) {
		err := recreateSchema()
		require.NoError(t, err)

		dirFiles := []string{
			"2022-12-12-01-create-table-statuses.sql",
			"2022-12-12-02-create-table-news.sql",
		}

		ch := make(chan string)
		go readFromCh(ch, t)

		err = testMigrator.DryRun(ctx, dirFiles, ch)
		require.NoError(t, err)
	})

	t.Run("with non transactional migrations", func(t *testing.T) {
		err := recreateSchema()
		require.NoError(t, err)

		dirFiles := []string{
			"2022-12-12-01-create-table-statuses.sql",
			"2022-12-12-02-create-table-news.sql",
			"2022-12-12-03-add-comments-news-NONTR.sql",
			"2022-12-13-01-create-categories-table.sql",
			"2022-12-13-02-create-tags-table.sql",
		}

		ch := make(chan string)
		go readFromCh(ch, t)

		err = testMigrator.DryRun(ctx, dirFiles, ch)
		assert.EqualError(t, err, `non transactional migration found "2022-12-12-03-add-comments-news-NONTR.sql", run all migrations before it, please`)
	})
}

func TestMigrator_skipMigrations(t *testing.T) {
	ctx := context.Background()

	err := recreateSchema()
	require.NoError(t, err)
	err = testMigrator.createMigratorTable(ctx)
	require.NoError(t, err)

	dirFiles := []string{
		"2022-12-12-01-create-table-statuses.sql",
		"2022-12-12-02-create-table-news.sql",
	}
	mm, err := testMigrator.newMigrations(dirFiles)
	require.NoError(t, err)

	ch := make(chan string)
	go readFromCh(ch, t)
	err = testMigrator.skipMigrations(ctx, mm, ch)
	require.NoError(t, err)

	for _, mg := range mm {
		var pm PgMigration
		err = testMigrator.db.ModelContext(ctx, &pm).Where(`"filename" = ?`, mg.Filename).Select()
		require.NoError(t, err)
		assert.NotEmpty(t, pm.FinishedAt)
	}
}

func TestMigrator_Skip(t *testing.T) {
	ctx := context.Background()

	err := recreateSchema()
	require.NoError(t, err)

	filenames, err := testMigrator.Plan(ctx)
	require.NoError(t, err)

	ch := make(chan string)
	go readFromCh(ch, t)
	err = testMigrator.Skip(ctx, filenames, ch)
	require.NoError(t, err)
}

func TestMigrator_compareMD5Sum(t *testing.T) {
	t.Run("correct checksums", func(t *testing.T) {
		filenames := []string{
			"2022-12-12-01-create-table-statuses.sql",
			"2022-12-12-02-create-table-news.sql",
		}
		mm, err := testMigrator.newMigrations(filenames)
		require.NoError(t, err)

		fileMigrations := mm.ToDB()
		dbMigrations := []PgMigration{
			{Filename: "2022-12-12-01-create-table-statuses.sql", Md5sum: "463fe73a85e13dd55fe210904ec19d7c"},
			{Filename: "2022-12-12-02-create-table-news.sql", Md5sum: "6158555b3ceb1a216b0cb365cb97fc71"},
		}

		invalid := testMigrator.compareMD5Sum(fileMigrations, dbMigrations)
		assert.Empty(t, invalid)
	})

	t.Run("invalid checksum", func(t *testing.T) {
		filenames := []string{
			"2022-12-12-01-create-table-statuses.sql",
			"2022-12-12-02-create-table-news.sql",
		}
		mm, err := testMigrator.newMigrations(filenames)
		require.NoError(t, err)

		fileMigrations := mm.ToDB()
		dbMigrations := []PgMigration{
			{Filename: "2022-12-12-01-create-table-statuses.sql", Md5sum: "463fe73a85e13dd55fe210904ec19d7c"},
			{Filename: "2022-12-12-02-create-table-news.sql", Md5sum: "invalid!!!"},
		}

		invalid := testMigrator.compareMD5Sum(fileMigrations, dbMigrations)
		require.Len(t, invalid, 1)
		assert.Equal(t, "2022-12-12-02-create-table-news.sql", invalid[0].Filename)
	})
}

func TestMigrator_Verify(t *testing.T) {
	ctx := context.Background()

	t.Run("empty table", func(t *testing.T) {
		err := recreateSchema()
		require.NoError(t, err)

		invalid, err := testMigrator.Verify(ctx)
		require.NoError(t, err)
		assert.Empty(t, invalid)
	})

	t.Run("correct migrations", func(t *testing.T) {
		err := recreateSchema()
		require.NoError(t, err)

		err = execRun(ctx, t)
		require.NoError(t, err)

		invalid, err := testMigrator.Verify(ctx)
		require.NoError(t, err)
		assert.Empty(t, invalid)
	})

	t.Run("invalid migrations", func(t *testing.T) {
		err := recreateSchema()
		require.NoError(t, err)

		err = execRun(ctx, t)
		require.NoError(t, err)

		invalidFilename := "2022-12-13-01-create-categories-table.sql"
		pm := PgMigration{Md5sum: "invalid!!!"}
		_, err = testMigrator.db.ModelContext(ctx, &pm).Column("md5sum").Where(`"filename" = ?`, invalidFilename).Update()
		require.NoError(t, err)

		invalid, err := testMigrator.Verify(ctx)
		require.NoError(t, err)
		require.Len(t, invalid, 1)
		assert.Equal(t, invalidFilename, invalid[0].Filename)
	})
}

func readFromCh(ch chan string, t *testing.T) {
	for x := range ch {
		t.Log(x)
	}
}

func recreateSchema() error {
	_, err := testDB.Exec("DROP SCHEMA public CASCADE; CREATE SCHEMA PUBLIC;")
	return err
}
