package main

import (
	"context"
	"log"
	"os"
	"path/filepath"

	"github.com/vmkteam/pgmigrator/pkg/app"
	"github.com/vmkteam/pgmigrator/pkg/migrator"

	"github.com/BurntSushi/toml"
	"github.com/go-pg/pg/v10"
	"github.com/spf13/cobra"
)

var cfgFile string

func main() {
	rootCmd := newRootCmd()
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", app.DefaultConfigFile, "config file")
	exitOnErr(rootCmd.ParseFlags(os.Args))

	// read config
	var (
		cfg     app.Config
		dbc     *pg.DB
		rootDir = "."
		ctx     = context.Background()
	)
	_, err := os.Stat(cfgFile)
	if err != nil {
		cfg = app.Config{
			App:        (&migrator.Migrator{}).NewConfig(),
			ConfigFile: cfgFile,
		}
	} else { // config file was found
		if _, err = toml.DecodeFile(cfgFile, &cfg); err != nil {
			exitOnErr(err)
		}

		// connect to db
		dbc = pg.Connect(cfg.Database)

		// set root dir
		cfg.ConfigFile, err = filepath.Abs(cfgFile)
		exitOnErr(err)
		rootDir = filepath.Dir(cfg.ConfigFile)
	}

	// create app and run
	a := app.New(rootCmd, dbc, rootDir, cfg)
	exitOnErr(a.Run(ctx))
}

func exitOnErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func newRootCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pgmigrator",
		Short: "Applies PostgreSQL migrations",
		Long:  ``,
	}
}
