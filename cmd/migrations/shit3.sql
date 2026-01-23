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
-- СТРУКТУРА SEAWEEDFS ХРАНИЛИЩА ДЛЯ ЗАДАЧ
-- ============================================================================
-- 
-- seaweedfs://gate149-problems/problems/{problem_id}/
-- │
-- ├── latest/
-- │   └── version.txt                      # Содержит: hash актуальной версии
-- │
-- ├── versions/
-- │   ├── {version_hash_1}/                # Иммутабельная версия 1
-- │   │   ├── manifest.json                # Основные метаданные задачи
-- │   │   │
-- │   │   ├── tests/
-- │   │   │   ├── tests.json               # Метаданные тестов (groups, points, etc)
-- │   │   │   ├── 01.in
-- │   │   │   ├── 01.out
-- │   │   │   ├── 02.in
-- │   │   │   ├── 02.out
-- │   │   │   └── ...
-- │   │   │
-- │   │   ├── checker/
-- │   │   │   ├── checker.cpp              # Исходник чекера
-- │   │   │   └── checker                  # Бинарник (Linux amd64)
-- │   │   │
-- │   │   ├── validator/
-- │   │   │   ├── validator.cpp
-- │   │   │   └── validator
-- │   │   │
-- │   │   ├── generator/
-- │   │   │   ├── gen.cpp
-- │   │   │   ├── gen                      # Бинарник
-- │   │   │   ├── gen_border.cpp
-- │   │   │   └── gen_border               # Бинарник
-- │   │   │
-- │   │   ├── interactor/                  # Для интерактивных задач
-- │   │   │   ├── interactor.cpp
-- │   │   │   └── interactor
-- │   │   │
-- │   │   ├── solutions/                   # Авторские решения (опционально)
-- │   │   │   ├── main.cpp
-- │   │   │   └── wrong_answer.cpp
-- │   │   │
-- │   │   └── media/                       # Изображения для условия
-- │   │       ├── image1.png
-- │   │       └── graph.svg
-- │   │
-- │   └── {version_hash_2}/                # Версия 2
-- │       └── ...
-- │
-- └── workspace/                           # Рабочая папка (только на диске, не в SeaweedFS)
--     └── ...                              # Редактируемая версия до коммита
--
--
-- ============================================================================
-- ПРИМЕР manifest.json
-- ============================================================================
-- {
--   "last_updated": "2026-01-23T15:30:00Z",
--   "problem_type": "pass-fail",
--   "max_score": null,
--   
--   "time_limit_ms": 2000,
--   "memory_limit_mb": 256,
--   "stdout_limit_mb": 64,
--   "code_size_limit_kb": 64,
--   
--   "meta_files": [
--     {
--       "type": "checker",
--       "filename": "checker.cpp",
--       "compiler": "cpp17",
--       "binary_sha256": "a3c5e8d9f1b2c4a6e7d8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0",
--       "dependencies": [
--         {"filename": "testlib.h", "version": 941}
--       ]
--     },
--     {
--       "type": "validator",
--       "filename": "validator.cpp",
--       "compiler": "cpp17",
--       "binary_sha256": "b4d6f9e2a3c5b7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2",
--       "dependencies": [
--         {"filename": "testlib.h", "version": 941}
--       ]
--     },
--     {
--       "type": "generator",
--       "filename": "gen.cpp",
--       "compiler": "cpp17",
--       "binary_sha256": "c5e7a0f3b4d6c8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3",
--       "dependencies": []
--     }
--   ],
--   
--   "statements": {
--     "ru": {
--       "title": "Сумма чисел",
--       "legend": "Даны два числа A и B. Найдите их сумму.",
--       "input_format": "Два целых числа A и B (-10^9 <= A, B <= 10^9).",
--       "output_format": "Выведите одно число — сумму A и B.",
--       "notes": "Обратите внимание на overflow.",
--       "interaction": "",
--       "scoring": ""
--     },
--     "en": {
--       "title": "Sum of Numbers",
--       "legend": "Given two numbers A and B. Find their sum.",
--       "input_format": "Two integers A and B (-10^9 <= A, B <= 10^9).",
--       "output_format": "Print one number — the sum of A and B.",
--       "notes": "Watch out for overflow.",
--       "interaction": "",
--       "scoring": ""
--     }
--   }
-- }
--
--
-- ============================================================================
-- ПРИМЕР tests/tests.json
-- ============================================================================
-- {
--   "groups": [
--     {
--       "ordinal": 0,
--       "name": "Samples",
--       "points": 0,
--       "points_policy": "complete-group",
--       "depends_on": [],
--       "tests": [1, 2]
--     },
--     {
--       "ordinal": 1,
--       "name": "Small numbers",
--       "points": 30,
--       "points_policy": "each-test",
--       "depends_on": [0],
--       "tests": [3, 10]
--     },
--     {
--       "ordinal": 2,
--       "name": "Large numbers",
--       "points": 70,
--       "points_policy": "complete-group",
--       "depends_on": [0, 1],
--       "tests": [11, 50]
--     }
--   ],
--   "tests": [
--     {"ordinal": 1, "method": "manual", "generator": null, "is_sample": true},
--     {"ordinal": 2, "method": "manual", "generator": null, "is_sample": true},
--     {"ordinal": 3, "method": "generated", "generator": "gen 1 10", "is_sample": false},
--     {"ordinal": 4, "method": "generated", "generator": "gen 1 100", "is_sample": false}
--   ]
-- }
--
--
-- ============================================================================
-- ПРИМЕР СТРУКТУРЫ НА ДИСКЕ (workshop/мастерская)
-- ============================================================================
-- 
-- /var/gate149/problems/{problem_id}/         # Рабочая папка на диске
-- │
-- ├── manifest.json                           # Редактируемый манифест
-- │
-- ├── tests/
-- │   ├── tests.json                          # Метаданные тестов
-- │   ├── 01.in                               # Только исходники тестов
-- │   ├── 01.out
-- │   ├── 02.in
-- │   ├── 02.out
-- │   └── ...
-- │
-- ├── checker/
-- │   └── checker.cpp                         # Только исходники (без бинарников)
-- │
-- ├── validator/
-- │   └── validator.cpp
-- │
-- ├── generators/
-- │   ├── gen.cpp
-- │   └── gen_border.cpp
-- │
-- ├── interactor/
-- │   └── interactor.cpp
-- │
-- ├── solutions/
-- │   ├── main.cpp                            # Авторское решение
-- │   └── wa.cpp                              # Неправильное решение для тестов
-- │
-- └── media/
--     ├── image1.png
--     └── diagram.svg
--
-- WORKFLOW:
-- 1. Пользователь редактирует файлы в /var/gate149/problems/{id}/
-- 2. При нажатии "Commit":
--    - Backend компилирует все .cpp файлы в бинарники
--    - Вычисляет SHA256 каждого бинарника и обновляет manifest.json
--    - Вычисляет hash всей версии: sha256
--    - Копирует всё (исходники + бинарники) в SeaweedFS:
--      seaweedfs://problems/{id}/versions/{version_hash}/
--    - Обновляет latest/version.txt с новым hash
--
--
-- ============================================================================
-- УДАЛЕННЫЕ ТАБЛИЦЫ (переносим в SeaweedFS):
-- ============================================================================
-- ❌ problem_tests          → SeaweedFS: tests/tests.json + tests/*.in/*.out
-- ❌ problem_test_groups    → SeaweedFS: tests/tests.json (groups array)
-- ❌ problem_special_files  → SeaweedFS: checker/, validator/, generator/, interactor/
-- ❌ problem_extra_files    → SeaweedFS: solutions/, media/
-- ❌ problem_packages       → SeaweedFS: versions/{hash}/ (иммутабельные версии)
--
--
-- ============================================================================
-- ПРЕИМУЩЕСТВА НОВОЙ СХЕМЫ:
-- ============================================================================
-- ✅ Атомарность: вся версия задачи в одной папке
-- ✅ Иммутабельность: versions/{hash}/ никогда не меняются
-- ✅ Простой backup: копируем папку целиком
-- ✅ Простой импорт: распаковали zip → скомпилировали → загрузили в SeaweedFS
-- ✅ Версионирование: каждый коммит = новая версия с уникальным hash
-- ✅ Меньше строк в БД: вместо 200+ строк на задачу → 1 строка в problems
-- ✅ Быстрее запросы: нет JOIN по 3-4 таблицам
-- ✅ Кэширование: Redis кэширует manifest.json и tests.json (TTL 1 час)
-- ✅ Скорость: бинарники предкомпилированы, SHA256 проверка 4ms
-- ✅ Безопасность: SHA256 защищает от подмены бинарников
--
--
-- ============================================================================
-- КАК РАБОТАЕТ В КОДЕ:
-- ============================================================================
-- 1.1. User запрашивает задачу (через latest)
--    - Backend: SELECT * FROM problems WHERE id = $1
--    - Backend: читает seaweedfs://problems/{id}/latest/version.txt → получает hash
-- 1.2 User запрашивает задачу (когда где-то хранится фиксированный hash)
-- 2. Backend: проверяет Redis кэш для manifest.json:
--    - Есть → возвращает из кэша (1ms)
--    - Нет → скачивает из seaweedfs://problems/{id}/versions/{hash}/manifest.json
--            → кэширует на 1 час → возвращает (20-50ms)
-- 3. Frontend: отображает задачу с примерами
-- 
-- 1. User отправляет решение
-- 2. Runner:
--    - Получает hash версии из contest_problems.package_hash (для контеста)
--      или из latest/version.txt (для архива задач)
--    - Скачивает всю папку seaweedfs://problems/{id}/versions/{hash}/
--    - Проверяет SHA256 всех бинарников (checker, validator, interactor)
--    - Запускает тестирование
--
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
