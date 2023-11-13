package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/BurntSushi/toml"
	"github.com/go-pg/pg/v10"
	"github.com/spf13/cobra"
	"github.com/vmkteam/pgmigrator/pkg/app"
	"github.com/vmkteam/pgmigrator/pkg/migrator"
)

var cfgFile string

func main() {
	log.SetFlags(0)
	rootCmd := newRootCmd()
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", app.DefaultConfigFile, "configuration file")
	rootCmd.InitDefaultVersionFlag()
	rootCmd.InitDefaultHelpFlag()
	exitOnErr(rootCmd.ParseFlags(os.Args))

	// read config
	cfg := app.Config{
		App:        migrator.NewDefaultConfig(),
		ConfigFile: cfgFile,
	}

	var mg *migrator.Migrator

	// check for configuration file
	_, err := os.Stat(cfgFile)
	if err == nil {
		_, err = toml.DecodeFile(cfgFile, &cfg)
		exitOnErr(err)

		cfg.ConfigFile, err = filepath.Abs(cfgFile)
		exitOnErr(err)

		mg = migrator.NewMigrator(pg.Connect(cfg.Database), cfg.App, filepath.Dir(cfg.ConfigFile))
	}

	// create app and run
	a := app.New(rootCmd, mg, cfg)
	exitOnErr(a.Run(context.Background()))
}

func exitOnErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func newRootCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "pgmigrator",
		Short:   "Command-line tool for PostgreSQL migrations",
		Long:    ``,
		Version: appVersion(),
	}
}

func appVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}

	return info.Main.Version
}
