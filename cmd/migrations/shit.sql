-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- Унифицированная таблица для файлов
-- (аватарки, файлы задач, исполняемые файлы и тп)
-- 
-- сам файл храним в s3, а в бд есть запись о файле
CREATE TABLE files
(
    id          uuid PRIMARY KEY     DEFAULT uuid_generate_v7(),
    owner_id    uuid        REFERENCES users (id) ON DELETE SET NULL,

    storage_key text        NOT NULL,
    filename    text        NOT NULL,
    mime_type   text,
    extension   text,
    size_bytes  bigint      NOT NULL,
    sha256      bytea UNIQUE,

    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE problem_extra_files
(
    problem_id uuid        NOT NULL REFERENCES problems (id) ON DELETE CASCADE,
    file_id    uuid        NOT NULL REFERENCES files (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE problem_special_files
(
    id uuid PRIMARY KEY DEFAULT uuid_generate_v7(),

    -- код программы 
    --
    -- всё ещё вопрос про расширение файла
    -- 
    -- возможно имеет смысл хранить прямо в бд в виде строки, 
    -- потому что он небольшой + хочется быстро отображать этот код в интерфейсе
    src_file_id  uuid NOT NULL REFERENCES files (id) ON DELETE CASCADE,

    -- исполняемый файл (после компиляции).
    --
    -- если не компилируемый, то просто копируем src_file_id в exec_file_id?
    -- (здесь типо нужно подумать про всякие питоны и тп)
    exec_file_id uuid NOT NULL REFERENCES files (id) ON DELETE CASCADE,

    -- команды для компиляции (null = no compilation)
    compile_cmd  text, 

    -- команды для выполнения
    exec_cmd  text NOT NULL,

    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TYPE problem_type AS ENUM ('pass-fail', 'scoring', 'interactive', 'multi-pass', 'submit-answer');

CREATE TABLE IF NOT EXISTS problems
(
    id              uuid PRIMARY KEY            DEFAULT uuid_generate_v7(),

    owner_id        uuid               REFERENCES users (id) ON DELETE SET NULL,
    visibility      problem_visibility NOT NULL DEFAULT 'private',

    -- titles звучит лучше?
    names           jsonb              NOT NULL DEFAULT '{
      "en": ""
    }',

    short_name      varchar(100) UNIQUE,                     -- короткий ID (example-a-plus-b)
    source          varchar(255),                            -- источник (Codeforces, ICPC, etc)
    -- Тип задачи
    problem_type    problem_type       NOT NULL DEFAULT 'pass-fail',

    --
    time_limit      integer            NOT NULL DEFAULT 1000,
    memory_limit    integer            NOT NULL DEFAULT 128,

    --
    stdout_limit    integer                     DEFAULT 8,   -- МБ, лимит на вывод
    code_size_limit integer                     DEFAULT 256, -- КБ, лимит на размер кода

    max_score       integer,                                 -- NULL = unbounded ?????

    -- Про программы:
    -- файл (чекер, валидатор и тп) - это код, 
    -- поэтому нужно дополнительно хранить название языка (cpp, python, etc)?
    -- или по расширению файла определять язык?

    -- Где будут храниться чекеры (и тп), доступные всем по умолчанию?
    --
    -- возможно в таблице problem_special_files?
    -- но тогда нужно думать про права доступа к этим файлам


    checker         uuid NOT NULL REFERENCES problem_special_files (id) ON DELETE CASCADE,

    validator       uuid REFERENCES problem_special_files (id) ON DELETE CASCADE,

    -- генератор тестов 
    generator       uuid REFERENCES problem_special_files (id) ON DELETE CASCADE,

    -- команды для генератора тестов в формате:
    -- gen 3 3
    -- gen 4 100
    gen_commands    text, -- jsonb?

    -- нужна для интерактивных задач
    interactor      uuid REFERENCES problem_special_files (id) ON DELETE CASCADE,

    -- Если не ошибаюсь, то icpc формат не разделяет условие на блоки, не уверен
    -- Hydro: content?: string (json)
    legend          varchar(4096)      NOT NULL DEFAULT '',
    input_format    varchar(4096)      NOT NULL DEFAULT '',
    output_format   varchar(4096)      NOT NULL DEFAULT '',
    notes           varchar(4096)      NOT NULL DEFAULT '',
    scoring         varchar(4096)      NOT NULL DEFAULT '',
    interaction     varchar(4096)      NOT NULL DEFAULT '',
    statement       varchar(10240)     NOT NULL DEFAULT '',

    -- теперь, html - юзлесс, потому что мы больше ничего не компилируем из latex в html

    created_at      timestamptz        NOT NULL DEFAULT now(),
    updated_at      timestamptz        NOT NULL DEFAULT now(),

    CHECK (length(title) != 0),
    CHECK (memory_limit BETWEEN 4 AND 1024),
    CHECK (time_limit BETWEEN 100 AND 60000),
    CHECK (output_limit IS NULL OR output_limit > 0),
    CHECK (max_score IS NULL OR max_score >= 0)
);

-- Группы тестов (опционально, если нужна гибкость)
CREATE TABLE problem_test_groups
(
    id            uuid PRIMARY KEY     DEFAULT uuid_generate_v7(),
    problem_id    uuid        NOT NULL REFERENCES problems (id) ON DELETE CASCADE,
    ordinal       integer     NOT NULL,
    points        integer     NOT NULL DEFAULT 0,                -- ?????
    points_policy varchar(20) NOT NULL DEFAULT 'complete-group', -- ????

    UNIQUE (problem_id, ordinal),
    CHECK (ordinal > 0)
);

-- Тесты
CREATE TABLE problem_tests
(
    id                uuid PRIMARY KEY      DEFAULT uuid_generate_v7(),
    problem_id        uuid         NOT NULL REFERENCES problems (id) ON DELETE CASCADE,
    test_group_id     uuid REFERENCES problem_test_groups (id) ON DELETE CASCADE,

    ordinal           integer      NOT NULL,

    -- S3 пути:
    input_s3_key      varchar(512) NOT NULL, -- "problems/123/tests/01.in"
    output_s3_key     varchar(512) NOT NULL, -- "problems/123/tests/01.out"

    -- Метаданные:
    input_size_bytes  bigint,                -- размер для оптимизации
    output_size_bytes bigint,

    is_sample         boolean      NOT NULL DEFAULT false,

    created_at        timestamptz  NOT NULL DEFAULT now(),
    updated_at        timestamptz  NOT NULL DEFAULT now(),

    UNIQUE (test_group_id, ordinal),
    CHECK (ordinal > 0),
    CHECK (length(input_s3_key) > 0),
    CHECK (length(output_s3_key) > 0)
);

CREATE INDEX problem_tests_problem_id_idx ON problem_tests (problem_id);
CREATE INDEX problem_tests_group_id_idx ON problem_tests (test_group_id, ordinal);
