-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- ============================================================================
-- ENUMS
-- ============================================================================
CREATE TYPE problem_visibility AS ENUM ('private', 'public', 'unlisted');
CREATE TYPE problem_type AS ENUM ('pass-fail', 'scoring', 'interactive', 'multi-pass', 'submit-answer');

-- ============================================================================
-- ТАБЛИЦА FILES - УПРОЩЕННАЯ
-- ============================================================================
-- Теперь тут только аватарки и пользовательские файлы
-- Файлы задач (тесты, чекеры и тп) → в S3 напрямую
CREATE TABLE files
(
    id          uuid PRIMARY KEY     DEFAULT uuid_generate_v7(),
    owner_id    uuid        REFERENCES users (id) ON DELETE SET NULL,

    storage_key text        NOT NULL,  -- путь в S3
    filename    text        NOT NULL,
    mime_type   text,
    size_bytes  bigint      NOT NULL,
    sha256      bytea UNIQUE,

    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX files_owner_id_idx ON files(owner_id);
CREATE INDEX files_sha256_idx ON files(sha256) WHERE sha256 IS NOT NULL;

-- ============================================================================
-- ТАБЛИЦА PROBLEMS - ОСНОВНАЯ
-- ============================================================================
CREATE TABLE problems
(
    id              uuid PRIMARY KEY            DEFAULT uuid_generate_v7(),

    owner_id        uuid               REFERENCES users (id) ON DELETE SET NULL,
    visibility      problem_visibility NOT NULL DEFAULT 'private',

    titles          jsonb              NOT NULL DEFAULT '{"en": ""}',
    short_name      varchar(100) UNIQUE,
    source          varchar(255),
    problem_type    problem_type       NOT NULL DEFAULT 'pass-fail',

    -- Лимиты
    time_limit      integer            NOT NULL DEFAULT 1000,  -- мс
    memory_limit    integer            NOT NULL DEFAULT 128,   -- МБ
    stdout_limit    integer                     DEFAULT 8,     -- МБ
    code_size_limit integer                     DEFAULT 256,   -- КБ
    max_score       integer,

    -- ============================================
    -- КЛЮЧЕВОЕ ИЗМЕНЕНИЕ: s3_bucket_path
    -- ============================================
    -- Вместо ссылок на problem_special_files, problem_tests, etc
    -- просто храним путь к папке в S3, где лежат ВСЕ файлы задачи
    --
    -- Пример: "problems/550e8400-e29b-41d4-a716-446655440000"
    --
    s3_bucket_path  text               NOT NULL,

    -- Версия задачи (увеличивается при изменении тестов/чекера)
    version         integer            NOT NULL DEFAULT 1,

    
    -- Условие задачи в БД (для быстрого доступа и full-text search)
    -- Структура: jsonb с полями по языкам
    -- Пример: {"en": {"legend": "...", "input": "...", "output": "..."}, "ru": {...}}
    statement       jsonb              NOT NULL DEFAULT '{}',
    -- 286: можно сделать структуру вместо jsonb? но тогда как хранить две версии условия для 2 языков?
    -- 
    -- Альтернатива: можно использовать отдельные поля для каждого языка
    -- statement_en    text            NOT NULL DEFAULT '',
    -- statement_ru    text            NOT NULL DEFAULT '',
    
    -- Дополнительные метаданные для поиска и отображения
    tags            text[]             DEFAULT '{}',           -- ["graph", "dp", "math"] ну типа
    difficulty      varchar(20),                               -- Может быть когда-нибудь...
    
    created_at      timestamptz        NOT NULL DEFAULT now(),
    updated_at      timestamptz        NOT NULL DEFAULT now(),

    CHECK (memory_limit BETWEEN 4 AND 1024),
    CHECK (time_limit BETWEEN 100 AND 60000),
    CHECK (max_score IS NULL OR max_score >= 0)
);

CREATE INDEX problems_owner_id_idx ON problems(owner_id);
CREATE INDEX problems_short_name_idx ON problems(short_name);
CREATE INDEX problems_visibility_idx ON problems(visibility);
CREATE INDEX problems_created_at_idx ON problems(created_at DESC);
CREATE INDEX problems_tags_idx ON problems USING GIN(tags);
CREATE INDEX problems_statement_idx ON problems USING GIN(statement jsonb_path_ops);

-- Full-text search по названиям задач
CREATE INDEX problems_titles_search_idx ON problems USING GIN(to_tsvector('english', titles::text));

-- ============================================================================
-- СТРУКТУРА S3 BUCKET для задач
-- ============================================================================
-- 
-- s3://gate149-problems/
-- └── problems/
--     └── {problem_id}/                    # UUID из таблицы problems
--         │
--         ├── manifest.json                # Главный манифест задачи
--         │                                # (версия, файлы, метаданные)
--         │
--         ├── statement/                   # Условие (опционально, если большое)
--         │   ├── en.md                    # Или в БД поле statement
--         │   ├── ru.md
--         │   └── assets/
--         │       └── images/
--         │
--         ├── tests/                       # Тесты
--         │   ├── tests.json               # Метаданные: groups, ordinals, samples
--         │   ├── 01.in
--         │   ├── 01.out
--         │   └── ...
--         │
--         ├── checker/                     # Чекер
--         │   ├── checker.cpp
--         │   ├── checker                  # Скомпилированный (опционально)
--         │   └── meta.json                # {lang: "cpp17", compile_cmd: "..."}
--         │
--         ├── validator/                   # Валидатор (опционально)
--         │   ├── validator.cpp
--         │   └── meta.json
--         │
--         ├── generator/                   # Генератор (опционально)
--         │   ├── generator.cpp
--         │   ├── gen_commands.txt         # Команды: "gen 3 3", "gen 4 100"
--         │   └── meta.json
--         │
--         ├── interactor/                  # Интерактор (для interactive задач)
--         │   ├── interactor.cpp
--         │   └── meta.json
--         │
--         ├── solutions/                   # Авторские решения (опционально)
--         │   ├── main.cpp
--         │   └── slow.py
--         │
--         └── packages/                    # Готовые пакеты для judge
--             ├── v1.zip
--             └── latest.zip
--
-- ============================================================================
-- ФОРМАТ manifest.json
-- ============================================================================
-- {
--   "problem_id": "uuid",
--   "version": 2,
--   "last_updated": "2026-01-15T20:30:00Z",
--   
--   "files": {
--     "checker": {"source": "checker/checker.cpp", "language": "cpp17"},
--     "validator": {"source": "validator/validator.cpp"},
--     "generator": {"source": "generator/generator.cpp"},
--     "interactor": null
--   },
--   
--   "tests": {
--     "manifest_path": "tests/tests.json",
--     "total_tests": 50,
--     "total_groups": 5,
--     "total_size_bytes": 12458960
--   }
-- }
--
-- ============================================================================
-- ФОРМАТ tests/tests.json
-- ============================================================================
-- {
--   "version": 2,
--   "last_updated": "2026-01-15T20:30:00Z",
--   
--   "groups": [
--     {
--       "ordinal": 1,
--       "name": "Samples",
--       "points": 0,
--       "points_policy": "complete-group",
--       "tests": [1, 2]
--     },
--     {
--       "ordinal": 2,
--       "name": "Main tests",
--       "points": 100,
--       "points_policy": "each-test",
--       "tests": [3, 4, 5, ...]
--     }
--   ],
--   
--   "tests": [
--     {
--       "ordinal": 1,
--       "group": 1,
--       "input_path": "01.in",
--       "output_path": "01.out",
--       "input_content": "1 2\n",      -- для samples (опционально)
--       "output_content": "3\n",        -- для samples (опционально)
--       "is_sample": true,
--       "size_bytes": 15
--     }
--   ]
-- }
--
-- ============================================================================
-- УДАЛЕННЫЕ ТАБЛИЦЫ (переносим в S3):
-- ============================================================================
-- ❌ problem_tests          → S3: tests/tests.json + tests/*.in, tests/*.out
-- ❌ problem_test_groups    → S3: tests/tests.json (groups array)
-- ❌ problem_special_files  → S3: checker/, validator/, generator/, interactor/
-- ❌ problem_extra_files    → S3: extra/ folder
-- ❌ problem_packages       → S3: packages/v{version}.zip
--
-- ============================================================================
-- ПРЕИМУЩЕСТВА НОВОЙ СХЕМЫ:
-- ============================================================================
-- ✅ Атомарность: все файлы задачи в одной папке S3
-- ✅ Простой backup: aws s3 sync s3://bucket/problems/{id}/ ./backup/
-- ✅ Простой импорт: загрузил zip → распаковал в S3 → INSERT в problems
-- ✅ Версионирование: S3 поддерживает версии файлов из коробки
-- ✅ Меньше строк в БД: вместо 200+ строк на задачу → 1 строка
-- ✅ Быстрее запросы: нет JOIN по 3-4 таблицам
-- ✅ Кэширование: Redis кэширует manifest.json и tests.json
--
-- ============================================================================
-- КАК РАБОТАЕТ В КОДЕ:
-- ============================================================================
-- 1. User запрашивает задачу
-- 2. Backend: SELECT * FROM problems WHERE id = $1 (получили метаданные)
-- 3. Backend: проверяет Redis кэш для tests.json
--    - Есть → возвращает из кэша (1ms)
--    - Нет → скачивает из S3 → кэширует на 1 час → возвращает (20-50ms)
-- 4. Frontend: отображает задачу с сэмплами
-- 5. Для judge: скачивает все тесты из S3 (или из pre-built package)
--
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS problems_titles_search_idx;
DROP INDEX IF EXISTS problems_statement_idx;
DROP INDEX IF EXISTS problems_tags_idx;
DROP INDEX IF EXISTS problems_created_at_idx;
DROP INDEX IF EXISTS problems_visibility_idx;
DROP INDEX IF EXISTS problems_short_name_idx;
DROP INDEX IF EXISTS problems_owner_id_idx;

DROP TABLE IF EXISTS problems;

DROP INDEX IF EXISTS files_sha256_idx;
DROP INDEX IF EXISTS files_owner_id_idx;
DROP TABLE IF EXISTS files;

DROP TYPE IF EXISTS problem_type;
DROP TYPE IF EXISTS problem_visibility;

DROP EXTENSION IF EXISTS pg_trgm;
DROP EXTENSION IF EXISTS "uuid-ossp";
-- +goose StatementEnd
