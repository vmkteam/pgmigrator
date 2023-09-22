package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/fatih/color"
	"github.com/go-pg/pg/v10"
	"github.com/rodaine/table"
	"github.com/spf13/cobra"

	"github.com/vmkteam/pgmigrator/pkg/migrator"
)

const (
	DefaultCount      = 5
	DefaultConfigFile = "pgmigrator.toml"
	DateFormat        = "2006-01-02 15:04:05"
)

type Config struct {
	Database   *pg.Options
	App        migrator.Config
	ConfigFile string `toml:"-"`
}

type App struct {
	rootCmd *cobra.Command
	mg      *migrator.Migrator
	dbc     *pg.DB
	cfg     Config
}

func New(rootCmd *cobra.Command, dbc *pg.DB, rootDir string, cfg Config) App {
	return App{
		rootCmd: rootCmd,
		mg:      migrator.NewMigrator(dbc, cfg.App, rootDir),
		cfg:     cfg,
		dbc:     dbc,
	}
}

func (a App) Run(ctx context.Context) error {
	a.rootCmd.AddCommand(a.createCmd(), a.dryRunCmd(ctx), a.lastCmd(ctx), a.planCmd(ctx), a.redoCmd(ctx), a.runCmd(ctx), a.verifyCmd(ctx))

	return a.rootCmd.Execute()
}

// createCmd represents the create command.
func (a App) createCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create",
		Short: "Creates default config file " + DefaultConfigFile + " in current dir",
		Long:  ``,
		Run: func(cmd *cobra.Command, args []string) {
			cfg := Config{App: a.mg.NewConfig()}
			var buf bytes.Buffer

			enc := toml.NewEncoder(&buf)
			if err := enc.Encode(cfg); err != nil {
				log.Fatalf("Failed to create file: %v", err)
				return
			}

			// write default DB config
			buf.WriteString(`
[Database]
  Addr     = "localhost:5432"
  User     = "postgres"
  Database = "pgmigrator"
  Password = ""
  PoolSize = 1
  ApplicationName = "pgmigrator"`)
			if err := os.WriteFile(a.cfg.ConfigFile, buf.Bytes(), os.ModePerm); err != nil {
				log.Fatalf("Failed to write file %s: %v", a.cfg.ConfigFile, err)
				return
			}

			log.Printf("File %v was successfully created.", a.cfg.ConfigFile)
		},
	}
}

// lastCmd represents the last command.
func (a App) lastCmd(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "last",
		Short: "Shows recent migrations from db",
		Long:  ``,
		Run: func(cmd *cobra.Command, args []string) {
			if err := a.hasDB(ctx); err != nil {
				log.Fatalf("DB connection error: %v", err)
			}

			// calculate count
			cnt, err := count(args)
			if err != nil {
				log.Fatal("invalid argument")
			}

			mm, err := a.mg.Last(ctx, cnt)
			if err != nil {
				log.Fatal("Execute command error: %w", err)
			}

			// print table
			tbl := table.New("ID", "StartedAt", "FinishedAt", "Duration", "Filename")
			for _, m := range mm {
				if m.FinishedAt != nil {
					tbl.AddRow(m.ID, m.StartedAt.Format(DateFormat), m.FinishedAt.Format(DateFormat), m.FinishedAt.Sub(m.StartedAt), m.Filename)
				} else { // err
					tbl.AddRow(m.ID, m.StartedAt.Format(DateFormat), "error while applying", "", m.Filename)
				}
			}

			fmt.Printf("Showing last %d migrations in %s:\n", cnt, a.cfg.App.Table)
			prepareTable(tbl).Print()
		},
	}
}

// planCmd shows migration files which can be applied.
func (a App) planCmd(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "plan",
		Short: "Shows migration files which can be applied",
		Long:  ``,
		Run: func(cmd *cobra.Command, args []string) {
			if err := a.hasDB(ctx); err != nil {
				log.Fatalf("DB connection error: %v", err)
			}

			mm, err := a.mg.Plan(ctx)
			if err != nil {
				log.Fatalf("Execute command failed: %v", err)
				return
			} else if len(mm) == 0 {
				fmt.Println("No new migrations were found.")
				return
			}

			// print table
			fmt.Printf("Planning to apply %d migrations:\n", len(mm))
			tbl := table.New("ID", "Filename")
			for i, m := range mm {
				tbl.AddRow(i+1, m)
			}
			prepareTable(tbl).Print()
		},
	}
}

// verifyCmd shows invalid migrations.
func (a App) verifyCmd(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "verify",
		Short: "Checks and shows invalid migrations",
		Long:  ``,
		Run: func(cmd *cobra.Command, args []string) {
			if err := a.hasDB(ctx); err != nil {
				log.Fatalf("DB connection error: %v", err)
			}

			mm, err := a.mg.Verify(ctx)
			if err != nil {
				log.Fatalf("Execute command error: %v", err)
			} else if len(mm) == 0 {
				fmt.Println("All applied migrations are correct!")
				return
			}

			// print table
			fmt.Printf("Found %d invalid applied migrations:\n", len(mm))
			tbl := table.New("ID", "StartedAt", "Filename", "MD5sum (applied)", "MD5sum (local)")
			for _, m := range mm {
				tbl.AddRow(m.ID, m.StartedAt.Format(DateFormat), m.Filename, m.Md5sum, m.Md5sumLocal)
			}

			prepareTable(tbl).Print()
		},
	}
}

// runCmd run to migrations.
func (a App) runCmd(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "run",
		Short: "Run to apply migrations",
		Long:  ``,
		Run: func(cmd *cobra.Command, args []string) {
			if err := a.hasDB(ctx); err != nil {
				log.Fatalf("DB connection error: %v", err)
			}

			// plan to apply
			mm, err := a.mg.Plan(ctx)
			if err != nil {
				log.Fatalf("Execute command failed: %v\n", err)
			} else if len(mm) == 0 {
				fmt.Println("No new migrations were found.")
				return
			}

			// calculate count
			cnt, err := count(args)
			if err != nil {
				log.Fatal("invalid argument")
			} else if cnt > len(mm) {
				cnt = len(mm)
			}

			fmt.Println("Running live migrations:")
			// apply migrations
			ch := make(chan string)
			wg := &sync.WaitGroup{}
			go readCh(ch, wg)
			if err = a.mg.Run(ctx, mm[:cnt], ch); err != nil {
				fmt.Printf("Apply migration error: %v", err)
				return
			}
			wg.Wait()
		},
	}
}

// dryRunCmd tries to apply migrations. Runs migrations inside single transaction and always rolllbacks it
func (a App) dryRunCmd(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "dryrun",
		Short: "Tries to apply migrations. Runs migrations inside single transaction and always rollbacks it",
		Long:  ``,
		Run: func(cmd *cobra.Command, args []string) {
			if err := a.hasDB(ctx); err != nil {
				log.Fatalf("DB connection error: %v", err)
			}

			if err := a.hasDB(ctx); err != nil {
				log.Fatalf("DB connection error: %v", err)
			}

			// plan to apply
			mm, err := a.mg.Plan(ctx)
			if err != nil {
				log.Fatalf("Execute command failed: %v\n", err)
			} else if len(mm) == 0 {
				fmt.Println("No new migrations were found.")
				return
			}

			// calculate count
			cnt, err := count(args)
			if err != nil {
				log.Fatal("invalid argument")
			} else if cnt > len(mm) {
				cnt = len(mm)
			}

			fmt.Println("BEGIN")
			// apply migrations
			ch := make(chan string)
			wg := &sync.WaitGroup{}
			go readCh(ch, wg)
			if err = a.mg.DryRun(ctx, mm[:cnt], ch); err != nil {
				log.Fatalf("Apply migration error: %v", err)
				return
			}
			wg.Wait()
			fmt.Println("ROLLBACK")
		},
	}
}

// redoCmd rerun last migration
func (a App) redoCmd(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "redo",
		Short: "Rerun last migration",
		Long:  ``,
		Run: func(cmd *cobra.Command, args []string) {
			if err := a.hasDB(ctx); err != nil {
				log.Fatalf("DB connection error: %v", err)
			}

			fmt.Println("Redo last migration:")
			ch := make(chan string)
			wg := &sync.WaitGroup{}
			go readCh(ch, wg)
			_, err := a.mg.Redo(ctx, ch)
			if err != nil {
				log.Fatalf("Apply migration error: %v", err)
				return
			}
			wg.Wait()
		},
	}
}

func (a App) hasDB(ctx context.Context) error {
	if a.dbc == nil {
		return errors.New("no db connection specified in config file. Run `pgmigrator create` for new config file")
	}

	return a.dbc.Ping(ctx)
}

func prepareTable(tbl table.Table) table.Table {
	headerFmt := color.New(color.FgGreen, color.Underline).SprintfFunc()
	columnFmt := color.New(color.FgYellow).SprintfFunc()
	tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)

	return tbl
}

func count(args []string) (int, error) {
	if len(args) == 0 {
		return DefaultCount, nil
	}

	return strconv.Atoi(args[0])
}

func readCh(ch chan string, wg *sync.WaitGroup) {
	wg.Add(1)
	var lastTime time.Time
	for x := range ch {
		// first run
		if !lastTime.IsZero() {
			fmt.Printf("done in %v\n", time.Since(lastTime))
		}

		fmt.Printf("  - %s \t...", x)
		lastTime = time.Now()
	}

	if !lastTime.IsZero() {
		fmt.Printf("done in %v\n", time.Since(lastTime))
	}

	wg.Done()
}
