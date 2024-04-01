# pgmigrator: command-line tool for PostgreSQL migrations.

[![Release](https://img.shields.io/github/release/vmkteam/pgmigrator.svg)](https://github.com/vmkteam/pgmigrator/releases/latest)
[![Build Status](https://github.com/vmkteam/pgmigrator/actions/workflows/go.yml/badge.svg?branch=master)](https://github.com/vmkteam/pgmigrator/actions)
[![Linter Status](https://github.com/vmkteam/pgmigrator/actions/workflows/golangci-lint.yml/badge.svg?branch=master)](https://github.com/vmkteam/pgmigrator/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/vmkteam/pgmigrator)](https://goreportcard.com/report/github.com/vmkteam/pgmigrator)
[![codecov](https://codecov.io/gh/vmkteam/pgmigrator/branch/master/graph/badge.svg)](https://codecov.io/gh/vmkteam/pgmigrator)

pgmigrator очень простая утилита, предназначенная для накатывания инкрементальных (up only) миграций только для PostgreSQL.

* Понятный инструмент для локальной разработки и стейджа, на проде - AS IS.
* Поддержка Ctrl+C - ROLLBACK текущей миграции, выход.

Ограничения
--
* База: PostgreSQL
* Маска файлов миграций: `YYYY-MM-DDD-<description>.sql` / `YYYY-MM-DD-<description>-NONTR.sql` 
* Типы миграций: только UP
* Алгоритм: применяем к базе данных отсортированные файлы с миграциями, которые есть в папке и подходят по маске файла, кроме тех, которые уже применены к базе
* Программа работает только с конфигурационным файлом, расположенным в папке с миграциями.


FAQ
--
Q: Почему только up миграции?<br>
A: В разработке мы почти никогда не пишем down миграции, потому что это бесполезно в 99% случаях. Если что-то пошло не так, то мы просто не накатываем миграцию или решаем вручную, что делать.

Q: Почему только PostgreSQL?<br>
A: Есть цель создать узкоспециализированный простой инструмент для миграций.

Q: Почему такая специфическая маска файла, а не общепризнанный `V<Version>_<description>.sql`?<br>
A: Так исторически сложилось. Дата в файле дает больше прозрачности, чем номер версии. Меньше конфликтов при разработке в ветке.<br>
_Есть возможность переопределить этот параметр через файл конфигурации._

Q: Почему не в виде библиотеки? Почему миграции именно в виде файлов на диске?<br>
A: Цель - простая утилита, которая работает с файлами. Альтернативы в виде библиотек уже написаны https://awesome-go.com/#database-schema-migration



Миграции
--
Файлы с миграциями расположены в папке. Подпапки не учитываются. Файлы отсортированы по имени.
Все занесенные миграции записываются в таблицу (по умолчанию `public.pgMigrations`)
Маска файла по умолчанию: `YYYY-MM-DDD-<description>.sql`

Все миграции запускаются в отдельной транзакции с определенным StatementTimeout, определенном в файле конфигурации.
Нетранзакционные миграции имеют следующую маску файла `YYYY-MM-DD-<description>-NONTR.sql` (например, для create index concurrently).

Можно переопределить маску файла через файл конфигурации. Если маска не указана - используется маска по умолчанию.
Если в имени файла есть `MANUAL`, то такая миграция игнорируется.

	2020 // папка, не учитывается
	pgmigrator.toml // обязательный файл конфигурации
	2021-04-12-create-table-commentTranslations.sql // запускается внутри транзакции
	2021-06-02-make-person-alias-not-null-NONTR.sql // запускается вне транзакции
	2021-06-03-make-person-alias-not-null-MANUAL.sql // игнорируется

Файл конфигурации
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

Запуск
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

Любая команда поддерживает аргумент в виде числа. Для `last` - это количество последних миграций (по умолчанию 5). Для всех остальных – номер файла из `plan`, до которого применять миграции. Если аргумент не передан, то ограничений нет (или используется значение по умолчанию).

Базовая директория для миграций - та, в которой расположен файл конфигурации.
То есть можно вызвать `pgmigrator --config docs/patches/pgmigrator.toml plan` - и он возьмет все миграции из папки `docs/patches`.  

### Plan

**Алгоритм**
* получить список файлов миграций, отсортированный по имени
* подключиться к бд
	- проверить, есть ли таблица
	- получить список миграций из базы
* отобразить список файлов миграций, которые надо применить (TODO: подсветить DROP?)

**Вывод** 

	Planning to apply Х migrations:
		1 - 2022-07-18-movieComments.sql
		2 - 2022-07-28-jwlinks.sql
		3 - 2022-07-30-compilations-fix.sql 

### Run

**Алгоритм**

* создаем таблицу, если нужно
* строим план
* для каждого файла миграции
  - если миграция обычная:
    - begin
      - выполняем миграцию
      - добавляем запись в бд о выполненной миграции
    - commit
  - если миграция non transactional:
    - добавляем запись о миграции
    - выполняем миграцию
    - обновляем запись о миграции
  - если что-то идет не так - то транзакция откатывается (ROLLBACK) и программа завершается (кроме nontr, так как не понятно, что именно произошло)

**Вывод** 

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

* Как пункт `Run`, только открываем одну большую транзакцию и используем ROLLBACK.
* если в имени файла миграции есть суффикс NONTR – не даем запустить dryrun (только до определенного имени файла)
* Настройка `StatementTimeout` игнорируется 

**Вывод**

Как в `Run`, только в конце выводим сообщение о ROLLBACK.

### Skip

Как и `Run`, но без выполнения sql миграции. Только добавление записи о том, что миграция применена 

### Last

Показываем последние транзакции.

**Вывод**

		Showing last migrations in public.pgMigrations:
		34 - 2022-08-30 22:25:03 (ERR) 	 > 2022-07-30-compilations-NONTR.sql
		33 - 2022-08-30 22:25:03 (3s)  	 > 2022-07-30-compilations-fix.sql
		32 - 2022-08-30 22:25:34 (1s)    > 2022-07-28-jwlinks.sql
		31 - 2022-08-30 22:23:12 (5m 4s) > 2022-07-18-movieComments.sql

### Verify

Проверяет целостность файлов миграций в базе данных и локально по md5 хешу.

### Init

Инициализирует новый файл конфигурации с параметрами по умолчанию.

### Redo

Выполняет еще раз последнюю миграцию, записанную в таблице миграций в бд.

Модель базы
--
По умолчанию: список примененных миграций хранится в таблице `pgMigrations`, схема `public`.<br>
Рекомендуется иметь для каждой схемы свою таблицу миграций.

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
* filename - уникальное имя файла миграции
* startedAt - дата запуска миграции
* finishedAt - дата завершения миграции
* transactional - флаг транзакционности (false для NONTR)
* md5sum - хеш сумма файла миграции


Процесс внедрения
--

1. Переносим все старые файлы миграций в отдельную папку (или подпапку).
2. Оставляем только новые патчи.
3. Создаем файл конфигурации (команда `pgmigrator init`) и модифицируем если это необходимо.
4. Применяем патчи через новый инструмент.

У каждого проекта своя схема, в ней своя таблица с pgMigrations.

При обновлении из гита можно вызвать `pgmigrator plan` из `docs/patches` и посмотреть новые патчи.
Внедрять можно на любой стадии проекта.

### Docker образы
- [Docker Hub](https://hub.docker.com/vmkteam/pgmigrator)
- Вкладка Packages в этом репозитории
