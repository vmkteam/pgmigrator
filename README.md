# pgmigrator: command-line tool for PostgreSQL migrations.

[![Release](https://img.shields.io/github/release/vmkteam/pgmigrator.svg)](https://github.com/vmkteam/pgmigrator/releases/latest)
[![Build Status](https://github.com/vmkteam/pgmigrator/actions/workflows/go.yml/badge.svg?branch=master)](https://github.com/vmkteam/pgmigrator/actions)
[![Linter Status](https://github.com/vmkteam/pgmigrator/actions/workflows/golangci-lint.yml/badge.svg?branch=master)](https://github.com/vmkteam/pgmigrator/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/vmkteam/pgmigrator)](https://goreportcard.com/report/github.com/vmkteam/pgmigrator)
[![codecov](https://codecov.io/gh/vmkteam/pgmigrator/branch/master/graph/badge.svg)](https://codecov.io/gh/vmkteam/pgmigrator)

pgmigrator is a very simple utility designed to roll up incremental (up only) migrations for PostgreSQL only.

Goal: to make an understandable tool for local development and stage environments.

Limitations
--
* Database: PostgeSQL
* File format: `YYYYY-MM-DDD-<description>.sql` / `YYYYY-MM-DD-<description>-NONTR.sql`
* Migration types: UP only
* Algorithm: run sorted files that are in the folder and fit the file format, except for what is already in the database
* The program works only with the configuration file located in the folder with migrations.


FAQ
--
Q: Why only up migrations?<br>
A: In development, we almost never write down migrations because it's useless in 99% of cases. If something goes wrong, we just don't roll up or decide manually what to do.

Q: Why only PostgreSQL? <br>
A: There is a goal to create a highly specialized simple tool for migrations.

Q: Why such a specific file format rather than the generally recognized one `V<Version>_<description>.sql`?<br>
A: This is historically the case. The date in the file gives more transparency than the version number. Less development conflicts in the branch.<br>
_It is possible to override this parameter through the config._

Q: Why not as a library? Why migrations specifically as files on disk?
A: The goal is a simple utility that works with files. Alternatives in the form of libraries have already been written here https://awesome-go.com/#database-schema-migration

Migrations
--
Migrations are located in a folder. Subfolders are not counted. Files are sorted by name.
All recorded migrations are written to a table (by default `public.pgMigrations`)
File format is `YYYYY-MM-DDD-<description>.sql`.

All migrations are started in a separate transaction with a specific StatementTimeout in the config. If not specified, it is not used.
Non-transactional migrations have the following format `YYYYY-MM-DD-<description>-NONTR.sql` (e.g. for create index concurrently).

You can override the file mask through the config. If not specified, the default one is used.
If there is `MANUAL` at the end of the file name, such migration is ignored.

        2020 // folder, not counted
        pgmigrator.toml // mandatory config file
        2021-04-12-create-table-commentTranslations.sql
        2021-06-02-make-person-alias-not-null-NONTR.sql
        2021-06-03-make-person-alias-not-null-MANUAL.sql // ignored


Configuration file
--
	[App]
	Table = "public.pgMigrations"
	StatementTimeout = "5s" 
	Filemask = "\d{4}-\d{2}-\d{2}-\S+.sql"
	
	[Database]
	Addr     = "localhost:5432"
	User     = "postgres"
	Database = "testdb"
	Password = "tesdb"
	PoolSize = 1
	ApplicationName = "pgmigrator"

Run
--
	Applies PostgreSQL migrations
	
	Usage:
	pgmigrator [command]
	
	Available Commands:
	completion  Generate the autocompletion script for the specified shell
	create      Creates default config file pgmigrator.toml in current dir
	dryrun      Tries to apply migrations. Runs migrations inside single transaction and always rollbacks it
	help        Help about any command
	last        Shows recent migrations from db
	plan        Shows migration files which can be applied
	redo        Rerun last migration
	run         Run to apply migrations
	verify      Checks and shows invalid migrations
	
	Flags:
	--config string   config file (default "pgmigrator.toml")
	-h, --help            help for pgmigrator
	
	Use "pgmigrator [command] --help" for more information about a command.

Any command supports an argument in the form of a number. For `last` it is the number of last migrations (default is 5). For all others - the number of the file from `plan` to which to apply migrations. If no argument is passed, there are no restrictions (or default ones are used).

The base directory for migrations is the one where the config file is located.
That is, you can call `pgmigrator --config docs/patches/pgmigrator.toml plan` and it will take all migrations from the `docs/patches` folder.

## Commands

### Plan

Algorithm:

* get a list of files sorted by name
* connect to the database
    - check if there is a table
    - get the list of migrations from the database
* display the list of files to be applied


### Run

    create a table, if necessary
    make a plan
    for each file.
        if the migration is regular
        begin
            perform migration
            add a record of the completed migration
        commit
    
        if migration is non-transactional
            add migration record
            perform migration
            update migration record
    
        if something goes wrong - rollback and exit (except for nontr, since it is not clear what exactly happened).
### DryRun

* Like Run, but open one big transaction and use ROLLBACK.
* If there is a NONTR - do not let dryrun run (only up to a certain filename)
* Ignore StatementTimeout

### Last

Shows the latest database migrations from a table.

### Verify

Checks patch integrity in the database and locally by md5 hash.

### Create

Creates a new configuration file.

### Redo

Perform again the last migration that is recorded in the table.

Database model
-- 
Default: table `pgMigrations`, scheme `public`.
It is recommended to have a different migrations table for each schema.

    create table if not exists "pgMigrations"
    (
        id            serial                    not null,
        filename      text                      not null,
        "startedAt"   timestamptz default now() not null,
        "finishedAt"  timestamptz,
        transactional bool        default true  not null,
        md5sum        varchar(32)               not null,
        primary key ("id"),
        unique ("filename")
    );