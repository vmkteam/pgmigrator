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
* Database: PostgreSQL
* Migrations filename mask: `YYYYY-MM-DDD-<description>.sql` / `YYYYY-MM-DD-<description>-NONTR.sql`
* Migration types: UP only
* Algorithm: applies sorted migrations files that are in the folder and fit the file mask, except for what is already applied in the database
* The program works only with the configuration file located in the folder with migrations.


FAQ
--
Q: Why only up migrations?<br>
A: In development, we almost never write down migrations because it's useless in 99% of cases. If something goes wrong, we just don't roll up or decide manually what to do.

Q: Why only PostgreSQL? <br>
A: There is a goal to create a highly specialized simple tool for migrations.

Q: Why such a specific file mask rather than the generally recognized one `V<Version>_<description>.sql`?<br>
A: This is historically the case. The date in the file gives more transparency than the version number. Less development conflicts in the branch.<br>
_It is possible to override this parameter through the configuration file._

Q: Why not as a library? Why migrations specifically as files on disk?
A: The goal is a simple utility that works with files. Alternatives in the form of libraries have already been written here https://awesome-go.com/#database-schema-migration

Migrations
--
Migration files are located in a folder. Subfolders are not counted. Files are sorted by name.
All recorded migrations are written to a table in database (by default `public.pgMigrations`)
Default file mask: `YYYYY-MM-DDD-<description>.sql`.

All migrations are started in a separate transaction with a specific StatementTimeout in the configuration file. If not specified, it is not used.
Non-transactional migrations have the following file mask: `YYYYY-MM-DD-<description>-NONTR.sql` (e.g. for create index concurrently).

You can override the file mask through the configuration file. If not specified, the default one is used.
If there is `MANUAL` at the end of the file name, this migration will be ignored.

        2020 // folder, not counted
        pgmigrator.toml // mandatory configuration file
        2021-04-12-create-table-commentTranslations.sql // runs inside transaction
        2021-06-02-make-person-alias-not-null-NONTR.sql  // runs outside transaction
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
	Password = "tesdb"
	Database = "testdb"
	PoolSize = 1
	ApplicationName = "pgmigrator"

Run
--
    Command-line tool for PostgreSQL migrations
    
    Usage:
    pgmigrator [command]
    
    Available Commands:
    completion  Generate the autocompletion script for the specified shell
    dryrun      Tries to apply migrations. Runs migrations inside single transaction and always rollbacks it
    help        Help about any command
    init        Initialize default configuration file in current directory
    last        Shows recent applied migrations from db
    plan        Shows migration files which can be applied
    redo        Rerun last applied migration from db
    run         Applies all new migrations
    skip        Marks migrations done without actually running them.
    verify      Checks and shows invalid migrations
    
    Flags:
    -c, --config string   configuration file (default "pgmigrator.toml")
    -h, --help            help for pgmigrator
    -v, --version         version for pgmigrator
    
    Use "pgmigrator [command] --help" for more information about a command.

Any command supports an argument in the form of a number. For `last` it is the number of last migrations (default is 5). For all others - the number of the file from `plan` to which to apply migrations. If no argument is passed, there are no restrictions (or default ones are used).

The base directory for migrations is the one where the configuration file is located.
That is, you can call `pgmigrator --config docs/patches/pgmigrator.toml plan` and it will take all migrations from the `docs/patches` folder.

## Commands

### Plan

**Algorithm**

* get a list of files sorted by name
* connect to the database
    - check if there is a table
    - get the list of migrations from the database
* display the list of files to be applied  (TODO: highlight DROP?)

**Output**

	Planning to apply Х migrations:
		1 - 2022-07-18-movieComments.sql
		2 - 2022-07-28-jwlinks.sql
		3 - 2022-07-30-compilations-fix.sql 


### Run

**Algorithm**

* create a table, if necessary
* make a plan
  - for each file
     - if the migration is regular
        - begin
          - perform migration
          - add a record of the completed migration
        - commit
     - if migration is non-transactional
        - add migration record
        - perform migration
        - update migration record
     - if something goes wrong - rollback and exit (except for nontr, since it is not clear what exactly happened).

**Output**

	Planning to apply Х migrations:
		1 - 2022-07-18-movieComments.sql
		2 - 2022-07-28-jwlinks.sql
		3 - 2022-07-30-compilations-fix.sql 

	Applying:
		1 - 2022-07-18-movieComments.sql ... done in 2s 
		2 - 2022-07-28-jwlinks.sql ... done in 3m
		3 - 2022-07-30-compilations-fix.sql ... 		
	ERROR: <error text>

### DryRun

* Like `Run`, but open one big transaction and use ROLLBACK.
* If there is a NONTR - do not let dryrun run (only up to a certain filename)
* `StatementTimeout` setting is ignored

### Skip

Like `Run`, but without actually running sql migration, only adding migration success record

### Last

Shows the latest database migrations from a table.

**Output**

    Showing last migrations in public.pgMigrations:
    34 - 2022-08-30 22:25:03 (ERR) 	 > 2022-07-30-compilations-NONTR.sql
    33 - 2022-08-30 22:25:03 (3s)  	 > 2022-07-30-compilations-fix.sql
    32 - 2022-08-30 22:25:34 (1s)    > 2022-07-28-jwlinks.sql
    31 - 2022-08-30 22:23:12 (5m 4s) > 2022-07-18-movieComments.sql

### Verify

Checks patch integrity in the database and locally by md5 hash.

### Init

Initializes a new configuration file with default settings.

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

* id (pk) – serial
* filename - unique migration file name
* startedAt - timestamp of starting migration
* finishedAt - timestamp of finishing migration
* transactional - transactional flag (false для NONTR migrations)
* md5sum - md5 hash of migration file 

### Docker images
- [Docker Hub](https://hub.docker.com/vmkteam/pgmigrator)
- Packages Tab in this repo
