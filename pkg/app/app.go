package app

import (
	"bytes"
	"context"
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
	cfg     Config
}

func New(rootCmd *cobra.Command, mg *migrator.Migrator, cfg Config) App {
	return App{
		rootCmd: rootCmd,
		mg:      mg,
		cfg:     cfg,
	}
}

func (a App) Run(ctx context.Context) error {
	a.rootCmd.AddCommand(a.initCmd(), a.dryRunCmd(ctx), a.lastCmd(ctx), a.planCmd(ctx), a.redoCmd(ctx), a.runCmd(ctx), a.verifyCmd(ctx), a.skipCmd(ctx))
	a.rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if cmd.Name() == "init" || cmd.Name() == "help" {
			return
		}

		if a.mg == nil {
			log.Fatal("Configuration file was not found. Please create new via `pgmigrator init`")
		}
	}

	return a.rootCmd.Execute()
}

// initCmd represents the init command.
func (a App) initCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize default configuration file in current directory",
		Long:  `Initialize default configuration file in current directory. If -c flag passed, initialize file with this name`,
		Run: func(cmd *cobra.Command, args []string) {
			var buf bytes.Buffer

			enc := toml.NewEncoder(&buf)
			if err := enc.Encode(migrator.NewDefaultConfig()); err != nil {
				log.Fatalf("Failed to create file: %v", err)
				return
			}

			// write default DB config
			buf.WriteString(`
[Database]
  Addr     = "localhost:5432"
  User     = "postgres"
  Password = ""
  Database = "pgmigrator"
  PoolSize = 1
  ApplicationName = "pgmigrator"`)
			if err := os.WriteFile(a.cfg.ConfigFile, buf.Bytes(), 0644); err != nil {
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
		Use:   "last [<count>]",
		Short: "Shows recent applied migrations from db",
		Long: `Shows recent applied migrations from db.
If <count> applied, shows recent <count> applied migrations. By default: 5`,
		Run: func(cmd *cobra.Command, args []string) {
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
		Use:   "run [<count>]",
		Short: "Applies all new migrations",
		Long: `Applies all new migrations.
If <count> applied, applies only <count> migrations from plan. By default: 5`,
		Run: func(cmd *cobra.Command, args []string) {
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
		Use:   "dryrun [<count>]",
		Short: "Tries to apply migrations. Runs migrations inside single transaction and always rollbacks it",
		Long: `Tries to apply migrations. Runs migrations inside single transaction and always rollbacks it.
If <count> applied, runs only <count> migrations. By default: 5`,
		Run: func(cmd *cobra.Command, args []string) {
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

// skipCmd marks migrations done without actually running them
func (a App) skipCmd(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "skip [<count>]",
		Short: "Marks migrations done without actually running them",
		Long: `Marks migrations done without actually running them.
If <count> applied, marks only first <count> migrations displayed in plan. Default <count> = 5.`,
		Run: func(cmd *cobra.Command, args []string) {
			// get list of migrations
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

			// skip migrations
			ch := make(chan string)
			wg := &sync.WaitGroup{}
			go readCh(ch, wg)
			fmt.Println("Skipping migrations...")
			if err = a.mg.Skip(ctx, mm[:cnt], ch); err != nil {
				log.Fatalf("Skip migration error: %v", err)
				return
			}
			wg.Wait()
			fmt.Println("Done")
		},
	}
}

// redoCmd rerun last migration
func (a App) redoCmd(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "redo",
		Short: "Rerun last applied migration from db",
		Long:  ``,
		Run: func(cmd *cobra.Command, args []string) {
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
