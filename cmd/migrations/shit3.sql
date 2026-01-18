-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TYPE problem_visibility AS ENUM ('private', 'public', 'unlisted');
CREATE TYPE problem_type AS ENUM ('pass-fail', 'scoring', 'interactive', 'multi-pass', 'submit-answer');

CREATE TABLE problems
(
    id         uuid PRIMARY KEY             DEFAULT uuid_generate_v7(),

    owner_id   uuid                REFERENCES users (id) ON DELETE SET NULL,
    visibility problem_visibility  NOT NULL DEFAULT 'private',

    titles     jsonb               NOT NULL,

    short_name varchar(100) UNIQUE NOT NULL,

    created_at timestamptz         NOT NULL DEFAULT now(),
    updated_at timestamptz         NOT NULL DEFAULT now()
);

CREATE INDEX problems_owner_id_idx ON problems (owner_id);
CREATE INDEX problems_short_name_idx ON problems (short_name);
CREATE INDEX problems_visibility_idx ON problems (visibility);
CREATE INDEX problems_created_at_idx ON problems (created_at DESC);

-- Full-text search по названиям задач
-- CREATE INDEX problems_titles_search_idx ON problems USING GIN (to_tsvector('english', titles::text));

CREATE TABLE contests 
(
    id         uuid PRIMARY KEY             DEFAULT uuid_generate_v7(),
 
    owner_id   uuid                REFERENCES users (id) ON DELETE SET NULL,

    titles     jsonb               NOT NULL,

    short_name varchar(100) UNIQUE NOT NULL,

    created_at timestamptz         NOT NULL DEFAULT now(),
    updated_at timestamptz         NOT NULL DEFAULT now()
);

CREATE TABLE contest_problems
(
    contest_id  uuid REFERENCES contests (id) ON DELETE CASCADE,
    problem_id  uuid REFERENCES problems (id) ON DELETE CASCADE,
    ordinal     integer NOT NULL,

    package_hash varchar(64) NOT NULL,

    PRIMARY KEY (contest_id, problem_id)
);

-- ============================================================================
-- СТРУКТУРА S3 BUCKET для задач
-- ============================================================================
-- 
-- s3://gate149-problems/
-- │
-- └── problems/
--     │
--     └── {problem_id}/                    # UUID задачи из таблицы problems
--         │
--         ├── metadata.json
--         ├── tests/                       # ТЕСТЫ
--         │   ├── tests.json               # Метаданные тестов (groups, ordinals, etc)
--         │   ├── 01.in
--         │   ├── 01.out
--         │   ├── 02.in
--         │   ├── 02.out
--         │   └── ...
--         │
--         ├── checker/                     # ЧЕКЕР
--         │   ├── checker.cpp              
--         │   ├── checker               
--         │
--         ├── validator/                   # ВАЛИДАТОР
--         │   ├── validator.cpp
--         │   ├── validator
--         │
--         ├── generators/                  # ГЕНЕРАТОРЫ
--         │   ├── gen.cpp
--         │   ├── gen
--         │   ├── gen_border.cpp
--         │   ├── gen_border
--         │
--         ├── interactor/                  # ИНТЕРАКТОР
--         │   ├── interactor.cpp
--         │   ├── interactor
--         │
--         ├── solutions/                   # АВТОРСКИЕ РЕШЕНИЯ (опционально)
--         │   └── main.cpp                  
--         ├── media/
--         │    └── image1.png                 
--         │
--         └── packages/                    # ПОХУЙ ПОКА ЧТО ВООБЩЕ КРИСТАЛЬНО ПОХУЙ
--             ├── v1.zip                   # Версия 1
--             ├── v2.zip                   # Версия 2
--             └── latest.zip               # Symlink
--
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
DROP INDEX IF EXISTS problems_created_at_idx;
DROP INDEX IF EXISTS problems_visibility_idx;
DROP INDEX IF EXISTS problems_short_name_idx;
DROP INDEX IF EXISTS problems_owner_id_idx;

DROP TABLE IF EXISTS problems;

DROP TYPE IF EXISTS problem_type;
DROP TYPE IF EXISTS problem_visibility;

DROP EXTENSION IF EXISTS pg_trgm;
DROP EXTENSION IF EXISTS "uuid-ossp";
-- +goose StatementEnd
