package migrator

import (
	"context"
	"os"
	"testing"

	"github.com/go-pg/pg/v10"
	. "github.com/smartystreets/goconvey/convey"
)

var (
	testDb       *pg.DB
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
	testDb = NewTestDB()
	testConfig = NewDefaultConfig()
	testMigrator = NewMigrator(testDb, testConfig, "testdata")
	os.Exit(m.Run())
}

func TestMigrator_Plan(t *testing.T) {
	ctx := context.Background()

	Convey("TestMigrator_Plan", t, func() {
		err := recreateSchema()
		So(err, ShouldBeNil)

		want := []string{
			"2022-12-12-01-create-table-statuses.sql",
			"2022-12-12-02-create-table-news.sql",
			"2022-12-12-03-add-comments-news-NONTR.sql",
			"2022-12-13-01-create-categories-table.sql",
			"2022-12-13-02-create-tags-table.sql",
		}
		got, err := testMigrator.Plan(ctx)
		So(err, ShouldBeNil)
		So(got, ShouldResemble, want)
	})
}

func TestMigrator_readFiles(t *testing.T) {
	Convey("TestMigrator_readFiles", t, func() {
		want := []string{
			"2022-12-12-01-create-table-statuses.sql",
			"2022-12-12-02-create-table-news.sql",
			"2022-12-12-03-add-comments-news-NONTR.sql",
			"2022-12-13-01-create-categories-table.sql",
			"2022-12-13-02-create-tags-table.sql",
		}

		filenames, err := testMigrator.readAllFiles()
		So(err, ShouldBeNil)
		So(filenames, ShouldResemble, want)
	})
}

func TestMigrator_compareFilenames(t *testing.T) {
	Convey("TestMigrator_compareFilenames", t, func() {
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
		So(res, ShouldResemble, want)
	})
}

func TestNewMigration(t *testing.T) {
	Convey("TestMigrator_compareFilenames", t, func() {
		res, err := NewMigration(testMigrator.rootDir, "2022-12-12-01-create-table-statuses.sql")
		So(err, ShouldBeNil)
		So(res, ShouldResemble, Migration{
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
		})
	})

}

func TestMigrator_prepareMigrationsToRun(t *testing.T) {
	Convey("TestMigrator_prepareMigrationsToRun", t, func() {
		dirFiles := []string{
			"2022-12-12-01-create-table-statuses.sql",
			"2022-12-12-02-create-table-news.sql",
			"2022-12-12-03-add-comments-news-NONTR.sql",
			"2022-12-13-01-create-categories-table.sql",
			"2022-12-13-02-create-tags-table.sql",
		}
		want := []Migration{
			{
				Filename:      "2022-12-12-01-create-table-statuses.sql",
				Transactional: true,
			},
			{
				Filename:      "2022-12-12-02-create-table-news.sql",
				Transactional: true,
			},
			{
				Filename:      "2022-12-12-03-add-comments-news-NONTR.sql",
				Transactional: false,
			},
			{
				Filename:      "2022-12-13-01-create-categories-table.sql",
				Transactional: true,
			},
			{
				Filename:      "2022-12-13-02-create-tags-table.sql",
				Transactional: true,
			},
		}

		res, err := testMigrator.newMigrations(dirFiles)
		So(err, ShouldBeNil)
		So(res, ShouldHaveLength, len(want))

		for i, m := range res {
			So(m.Filename, ShouldEqual, want[i].Filename)
			So(m.Transactional, ShouldEqual, want[i].Transactional)
		}
	})
}

func TestMigrator_applyNonTransactionalMigration(t *testing.T) {
	t.Skip()
	ctx := context.Background()
	Convey("TestMigrator_applyNonTransactionalMigration", t, func() {
		err := recreateSchema()
		So(err, ShouldBeNil)

		_, err = testMigrator.db.ExecContext(ctx, `create table news (id text);`)
		So(err, ShouldBeNil)
		mg, err := NewMigration(testMigrator.rootDir, "2022-12-12-03-add-comments-news-NONTR.sql")
		So(err, ShouldBeNil)

		err = testMigrator.applyNonTransactionalMigration(ctx, mg)
		So(err, ShouldBeNil)

		var pm PgMigration
		err = testMigrator.db.ModelContext(ctx, &pm).Where(`"filename" = ?`, mg.Filename).Select()
		So(err, ShouldBeNil)
		So(pm, ShouldNotBeNil)
		So(pm.FinishedAt, ShouldNotBeEmpty)
	})
}

func TestMigrator_applyMigration(t *testing.T) {
	t.Skip()
	ctx := context.Background()
	Convey("TestMigrator_applyMigration", t, func() {
		mg, err := NewMigration(testMigrator.rootDir, "2022-12-12-01-create-table-statuses.sql")
		So(err, ShouldBeNil)

		err = testMigrator.applyMigration(ctx, mg)
		So(err, ShouldBeNil)

		var pm PgMigration
		err = testMigrator.db.ModelContext(ctx, &pm).Where(`"filename" = ?`, mg.Filename).Select()
		So(err, ShouldBeNil)
		So(pm, ShouldNotBeNil)
		So(pm.FinishedAt, ShouldNotBeEmpty)
	})
}

func TestMigrator_Run(t *testing.T) {
	ctx := context.Background()
	Convey("TestMigrator_Run", t, func() {
		err := recreateSchema()
		So(err, ShouldBeNil)

		err = execRun(ctx, t)
		So(err, ShouldBeNil)
	})
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
	Convey("TestMigrator_Last", t, func() {
		Convey("check empty list", func() {
			err := recreateSchema()
			So(err, ShouldBeNil)

			list, err := testMigrator.Last(ctx, 5)
			So(err, ShouldBeNil)
			So(list, ShouldHaveLength, 0)
		})
		Convey("check all applied list", func() {
			err := recreateSchema()
			So(err, ShouldBeNil)
			err = execRun(ctx, t)
			So(err, ShouldBeNil)

			list, err := testMigrator.Last(ctx, 5)
			So(err, ShouldBeNil)
			So(list, ShouldHaveLength, 5)
		})
		Convey("check 3 last migrations", func() {
			list, err := testMigrator.Last(ctx, 3)
			So(err, ShouldBeNil)
			So(list, ShouldHaveLength, 3)
		})
	})
}

func TestMigrator_Redo(t *testing.T) {
	ctx := context.Background()
	Convey("TestMigrator_Redo", t, func() {
		Convey("check if migrations wasn't applied", func() {
			err := recreateSchema()
			So(err, ShouldBeNil)

			ch := make(chan string)
			go readFromCh(ch, t)

			pm, err := testMigrator.Redo(ctx, ch)
			So(err.Error(), ShouldEqual, "applied migrations were not found")
			So(pm, ShouldBeNil)
		})
		Convey("redo last applied", func() {
			err := recreateSchema()
			So(err, ShouldBeNil)

			err = execRun(ctx, t)
			So(err, ShouldBeNil)

			_, err = testDb.Exec("DROP TABLE tags CASCADE;")
			So(err, ShouldBeNil)

			ch := make(chan string)
			go readFromCh(ch, t)

			pm, err := testMigrator.Redo(ctx, ch)
			So(err, ShouldBeNil)
			So(pm, ShouldResemble, &PgMigration{
				ID:            pm.ID,
				Filename:      "2022-12-13-02-create-tags-table.sql",
				StartedAt:     pm.StartedAt,
				FinishedAt:    pm.FinishedAt,
				Transactional: true,
				Md5sum:        "d10bca7f78e847d3d4e71003b31a54a6",
			})
		})
	})
}

func TestMigrator_dryRunMigrations(t *testing.T) {
	ctx := context.Background()
	Convey("TestMigrator_prepareMigrationsToDryRun", t, func() {
		err := recreateSchema()
		So(err, ShouldBeNil)
		err = testMigrator.createMigratorTable(ctx)
		So(err, ShouldBeNil)

		dirFiles := []string{
			"2022-12-12-01-create-table-statuses.sql",
			"2022-12-12-02-create-table-news.sql",
		}
		mm, err := testMigrator.newMigrations(dirFiles)
		So(err, ShouldBeNil)

		ch := make(chan string)
		go readFromCh(ch, t)
		err = testMigrator.dryRunMigrations(ctx, mm, ch)
		So(err, ShouldBeNil)
	})
}

func TestMigrator_DryRun(t *testing.T) {
	ctx := context.Background()
	Convey("TestMigrator_DryRun", t, func() {
		Convey("check transactional migrations to dry run", func() {
			err := recreateSchema()
			So(err, ShouldBeNil)

			dirFiles := []string{
				"2022-12-12-01-create-table-statuses.sql",
				"2022-12-12-02-create-table-news.sql",
			}

			ch := make(chan string)
			go readFromCh(ch, t)

			err = testMigrator.DryRun(ctx, dirFiles, ch)
			So(err, ShouldBeNil)
		})

		Convey("check with non transactional migrations to dry run", func() {
			err := recreateSchema()
			So(err, ShouldBeNil)
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
			So(err.Error(), ShouldEqual, `non transactional migration found "2022-12-12-03-add-comments-news-NONTR.sql", run all migrations before it, please`)
		})
	})
}

func TestMigrator_skipMigrations(t *testing.T) {
	ctx := context.Background()
	Convey("TestMigrator_skipMigrations", t, func() {
		err := recreateSchema()
		So(err, ShouldBeNil)
		err = testMigrator.createMigratorTable(ctx)
		So(err, ShouldBeNil)

		dirFiles := []string{
			"2022-12-12-01-create-table-statuses.sql",
			"2022-12-12-02-create-table-news.sql",
		}
		mm, err := testMigrator.newMigrations(dirFiles)
		So(err, ShouldBeNil)

		ch := make(chan string)
		go readFromCh(ch, t)
		err = testMigrator.skipMigrations(ctx, mm, ch)
		So(err, ShouldBeNil)

		for _, mg := range mm {
			var pm PgMigration
			err = testMigrator.db.ModelContext(ctx, &pm).Where(`"filename" = ?`, mg.Filename).Select()
			So(err, ShouldBeNil)
			So(pm, ShouldNotBeNil)
			So(pm.FinishedAt, ShouldNotBeEmpty)
		}
	})
}

func TestMigrator_Skip(t *testing.T) {
	ctx := context.Background()
	Convey("TestMigrator_Skip", t, func() {
		err := recreateSchema()
		So(err, ShouldBeNil)
		filenames, err := testMigrator.Plan(ctx)
		So(err, ShouldBeNil)
		ch := make(chan string)
		go readFromCh(ch, t)
		err = testMigrator.Skip(ctx, filenames, ch)
		So(err, ShouldBeNil)
	})
}

func TestMigrator_compareMD5Sum(t *testing.T) {
	Convey("TestMigrator_compareMD5Sum", t, func() {
		Convey("check correct", func() {
			filenames := []string{
				"2022-12-12-01-create-table-statuses.sql",
				"2022-12-12-02-create-table-news.sql",
			}
			mm, err := testMigrator.newMigrations(filenames)
			So(err, ShouldBeNil)
			fileMigrations := mm.ToDB()
			dbMigrations := []PgMigration{
				{
					Filename: "2022-12-12-01-create-table-statuses.sql",
					Md5sum:   "463fe73a85e13dd55fe210904ec19d7c",
				},
				{
					Filename: "2022-12-12-02-create-table-news.sql",
					Md5sum:   "6158555b3ceb1a216b0cb365cb97fc71",
				},
			}

			invalid := testMigrator.compareMD5Sum(fileMigrations, dbMigrations)
			So(invalid, ShouldHaveLength, 0)
		})
		Convey("check invalid", func() {
			filenames := []string{
				"2022-12-12-01-create-table-statuses.sql",
				"2022-12-12-02-create-table-news.sql",
			}
			mm, err := testMigrator.newMigrations(filenames)
			So(err, ShouldBeNil)
			fileMigrations := mm.ToDB()

			dbMigrations := []PgMigration{
				{
					Filename: "2022-12-12-01-create-table-statuses.sql",
					Md5sum:   "463fe73a85e13dd55fe210904ec19d7c",
				},
				{
					Filename: "2022-12-12-02-create-table-news.sql",
					Md5sum:   "invalid!!!",
				},
			}

			invalid := testMigrator.compareMD5Sum(fileMigrations, dbMigrations)
			So(invalid, ShouldHaveLength, 1)
			So(invalid[0].Filename, ShouldEqual, "2022-12-12-02-create-table-news.sql")
		})
	})
}

func TestMigrator_Verify(t *testing.T) {
	ctx := context.Background()
	Convey("TestMigrator_Verify", t, func() {
		Convey("check from empty table", func() {
			err := recreateSchema()
			So(err, ShouldBeNil)

			invalid, err := testMigrator.Verify(ctx)
			So(err, ShouldBeNil)
			So(invalid, ShouldHaveLength, 0)
		})
		Convey("check from correct migrations", func() {
			err := recreateSchema()
			So(err, ShouldBeNil)

			err = execRun(ctx, t)
			So(err, ShouldBeNil)

			invalid, err := testMigrator.Verify(ctx)
			So(err, ShouldBeNil)
			So(invalid, ShouldHaveLength, 0)
		})
		Convey("check from invalid migrations", func() {
			err := recreateSchema()
			So(err, ShouldBeNil)

			err = execRun(ctx, t)
			So(err, ShouldBeNil)

			invalidFilename := "2022-12-13-01-create-categories-table.sql"
			pm := PgMigration{Md5sum: "invalid!!!"}
			_, err = testMigrator.db.ModelContext(ctx, &pm).Column("md5sum").Where(`"filename" = ?`, invalidFilename).Update()
			So(err, ShouldBeNil)

			invalid, err := testMigrator.Verify(ctx)
			So(err, ShouldBeNil)
			So(invalid, ShouldHaveLength, 1)
			So(invalid[0].Filename, ShouldEqual, invalidFilename)
		})
	})
}

func readFromCh(ch chan string, t *testing.T) {
	for x := range ch {
		t.Log(x)
	}
}

func recreateSchema() error {
	_, err := testDb.Exec("DROP SCHEMA public CASCADE; CREATE SCHEMA PUBLIC;")
	return err
}
