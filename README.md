# pgmigrator


pgmigrator очень простая утилита, предназначенная для накатывания инкрементальных (up only) миграций только для PostgreSQL.

* Понятный инструмент для локальной разработки и стейджа, на проде - AS IS.
* Поддержка Ctrl+C - ROLLBACK текущией миграции, выход.

Ограничения
--
* База: PostgeSQL
* Формат файла: `YYYY-MM-DDD-<description>.sql` / `YYYY-MM-DD-<description>-NONTR.sql` 
* Типы миграций: только UP
* Алгоритм: накатываем отсортированные файлы, которые есть в папке и подходят по формату файла, кроме того, что есть уже в базе
* Программа работает только с конфигурационным файлом, расположенным в папке с миграциями.


FAQ
--
Q: Почему только up миграции?<br>
A: В разработке мы почти никогда не пишем down миграции, потому что это бесполезно в 99% случаях. Если что-то пошло не так, то мы просто не накатываем или решаем вручную, что делать.

Q: Почему только PostgreSQL?<br>
A: Есть цель создать узкоспециализированный простой инструмент для миграций.

Q: Почему такой ретроградный формат файла, а не общепризнанный `V<Version>_<description>.sql`?<br>
A: Так исторически сложилось. Дата в файле дает больше прозрачности, чем номер версии. Меньше конфликтов при разработке в ветке.<br>
Есть возможность переопределить этот параметр через конфиг._


Q: Почему нет в виде библиотеки? Почему миграции именно в виде файлов на диске?<br>
A: Цель - простая утилита, которая работает с файлами. Альтернативы в виде библиотек уже написаны https://awesome-go.com/#database-schema-migration



Миграции
--
Миграции расположены в папке. Подпапки не учитываются. Файлы отсортированы по имени.
Все занесенные миграции записываются в таблицу (по умолчанию `public.pgMigrations`)
Формат файла `YYYY-MM-DDD-<description>.sql`

Все миграции запускаются в отедльной транзакции с определенным StatementTimeout в конфиге.
Нетранзакционные миграции имеют следующий формат `YYYY-MM-DD-<description>-NONTR.sql` (например, для create index concurrently).

Можно переопределить маску файла через конфиг. Если не указана - используется дефолтная.
Если в названии файла есть `MANUAL`, то такая миграция игнорируется.

	2020 // папка, не учитывается
	pgmigrator.toml // обязательный конфиг файл
	2021-04-12-create-table-commentTranslations.sql
	2021-06-02-make-person-alias-not-null-NONTR.sql
	2021-06-03-make-person-alias-not-null-MANUAL.sql // игнорируется

Конфиг файл
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

Запуск
--
	pgmigrator OPTIONS COMMNAD [args]
		OPTIONS:
			-c config file (default pgmigrator.toml)
			-v show sql

		COMMAND:
			plan   - show pending migrations
			run    - run in db
			dryrun - (begin rollback, check nontr)
			last   - show last transactions, default number is 5
			create - create config file

		Args: 
			[max file number from plan] for plan/run/dryrun
			[top X] from last

Любая команда поддерживает аргумент в виде числа. Для `last` - это количество последних миграций (по умолчанию 5). Для всех остальных – номер файла из `plan`, до которого применять миграции. Если аргумент не передан, то ограничений нет (или используются дефолтные).

Базовая директория для миграций - та, в которой расположен конфиг файл.
То есть можно вызвать `pgmigrator -c docs/patches/pgmigrator.toml plan` - и он возьмет все миграции из папки `docs/patches`.  

### Plan

Алгоритм:

* получить список файлов, отсортированный по имени
* законнектится к бд
	- проверить, есть ли таблица
	- получить список миграций из базы
* отобразить список файлов, которые надо применить (подсветить DROP?)

Вывод 

	Planning to apply Х migrations:
		1 - 2022-07-18-movieComments.sql
		2 - 2022-07-28-jwlinks.sql
		3 - 2022-07-30-compilations-fix.sql 
	
### Run

	создаем таблицу, если нужно
	строим план
	для каждого файла
		если миграция обычная
		begin
			выполняем миграцию
			добавляем запись о выполненой миграции
		commit

		если миграция non transactional
			добавляем запись о миграции
			выполняем миграцию
			обновляем запись о миграции

		если что-то идет не так - то rollback и выход (кроме nontr, так как не понятно, что именно произошло.)

Вывод 

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

* Как пункт Run, только открываем одну большую транзакцию и используем ROLLBACK.
* если есть NONTR – не даем запустить dryrun (только до определнного имени файла)
* Игнорируем StatementTimeout

Вывод: как в Run, только в конце выводим сообщение о ROLLBACK.

### Last

Показываем последние транзакции.

Вывод

		Showing last migrations in public.pgMigrations:
		34 - 2022-08-30 22:25:03 (ERR) 	 > 2022-07-30-compilations-NONTR.sql
		33 - 2022-08-30 22:25:03 (3s)  	 > 2022-07-30-compilations-fix.sql
		32 - 2022-08-30 22:25:34 (1s)    > 2022-07-28-jwlinks.sql
		31 - 2022-08-30 22:23:12 (5m 4s) > 2022-07-18-movieComments.sql


Модель базы
-- 
По умолчанию: таблица `pgMigrations`, схема `public`. 
Рекомендуется иметь для каждой схемы свою таблицу миграций.

* id (pk) – serial 
* filename (unique) - имя файла
* startedAt - дата запуска транзакции
* finishedAt - дата завершения транзакции
* transactional bool - флаг транзакционнности (false для NONTR) 
* md5sum - хеш сумма файла


Процесс внедрения
--

1. Переносим все старые файлы в папке `docs/patches` в `2022`.
2. Оставляем только новые патчи.
3. Создаем конфиг файл, применяем патч через новый инструмент.

У каждого проекта своя схема, в ней своя таблица с pgMigrations.

При апдейте из гита можно вызвать `pgmigrator plan` из `docs/patches` и посмотреть новые патчи.
Внедрять можно на любой стадии проекта.

### CI/CD

Предполагается, что будет собран единый контейнер из pgmigrator вместе с SQL файлами. 

* Через артефакты прокидываем папку c sql файлами `docs/patches/*.sql` (если нужно)
* Собираем образ из `pgmigrator` вместе с этими файлами, отправляем в репозиторий
* Запускаем вручную отдельную джобу с pgmigrator последней версии, прокидываем конфиг.
