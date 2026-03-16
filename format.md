# Формат хранения задач в Gate (актуально)

## 1) Две разные сущности: задача и пакет задачи

В системе есть два уровня хранения:

1. **Workshop-репозиторий (Git)** — рабочая копия задачи, которую редактирует пользователь.
2. **Published package** — неизменяемая версия для судейской системы.

### Workshop-репозиторий

Пример структуры:

```
/{problem_id}/
│
├── manifest.json
├── .git/
├── .gitignore
├── README.md
│
├── tests/
│   ├── tests.json
│   ├── 01.in
│   ├── 01.out
│   ├── 02.in
│   ├── 02.out
│   └── ...
│
├── checkers/
│   ├── checker.cpp
│   └── testlib.h
├── validators/
│   └── validator.cpp
├── generators/
│   ├── gen.cpp
│   └── gen_border.cpp
├── interactors/
│   └── interactor.cpp
├── solutions/
│   ├── main.cpp
│   └── wrong_answer.cpp
└── media/
    ├── image1.png
    └── diagram.svg
```

---

## 2) Формат `manifest.json`

### Обязательные и важные поля

- `problem_type`: одно из `pass-fail`, `scoring`, `interactive`, `multi-pass`, `submit-answer`.
- `max_score`:
  - обязателен для `scoring`;
  - должен быть `null` для остальных типов.
- Лимиты (`time_limit_ms`, `memory_limit_mb`, `stdout_limit_mb`, `code_size_limit_kb`) должны быть положительными.
- В `statements` должен быть минимум один язык; у каждого языка обязательны непустые `title` и `legend`.
- `meta_files[].type`: `checker`, `validator`, `generator`, `interactor`.

### Пример

```json
{
  "last_updated": "2026-03-16T11:00:00Z",
  "problem_type": "scoring",
  "max_score": 100,

  "time_limit_ms": 2000,
  "memory_limit_mb": 256,
  "stdout_limit_mb": 64,
  "code_size_limit_kb": 256,

  "meta_files": [
    {
      "type": "checker",
      "filename": "checkers/checker.cpp",
      "compiler": "cpp17",
      "binary_sha256": "a3c5e8d9f1b2c4a6e7d8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0",
      "dependencies": [{ "filename": "testlib.h", "version": "0.9.41" }]
    },
    {
      "type": "validator",
      "filename": "validators/validator.cpp",
      "compiler": "cpp17",
      "binary_sha256": null,
      "dependencies": [{ "filename": "testlib.h", "version": "0.9.41" }]
    },
    {
      "type": "generator",
      "filename": "generators/gen.cpp",
      "compiler": "cpp17",
      "binary_sha256": null,
      "dependencies": []
    }
  ],

  "statements": {
    "ru": {
      "title": "Сумма чисел",
      "legend": "Даны два целых числа A и B. Найдите их сумму.",
      "input_format": "Два целых числа A и B (-10^9 <= A, B <= 10^9).",
      "output_format": "Выведите одно число — сумму A и B.",
      "notes": "",
      "interaction": "",
      "scoring": ""
    },
    "en": {
      "title": "Sum of Numbers",
      "legend": "Given two integers A and B. Find their sum.",
      "input_format": "Two integers A and B (-10^9 <= A, B <= 10^9).",
      "output_format": "Print one number — the sum of A and B.",
      "notes": "",
      "interaction": "",
      "scoring": ""
    }
  }
}
```

---

## 3) Формат `tests/tests.json`

### Правила

- `tests[].ordinal` — последовательные номера `1..N` без пропусков.
- `groups[].ordinal` — последовательные номера `0..M-1` без пропусков.
- `groups[].tests` — диапазон `[start, end]`.
- `points_policy`: `complete-group` или `each-test`.
- Для `method = "generated"` поле `generator` обязательно.
- Для `problem_type = "scoring"` сумма `groups[].points` должна равняться `max_score`.

### Пример

```json
{
  "groups": [
    {
      "ordinal": 0,
      "name": "Samples",
      "points": 0,
      "points_policy": "complete-group",
      "depends_on": [],
      "tests": [1, 2]
    },
    {
      "ordinal": 1,
      "name": "Small numbers",
      "points": 30,
      "points_policy": "each-test",
      "depends_on": [0],
      "tests": [3, 10]
    },
    {
      "ordinal": 2,
      "name": "Large numbers",
      "points": 70,
      "points_policy": "complete-group",
      "depends_on": [0, 1],
      "tests": [11, 50]
    }
  ],
  "tests": [
    { "ordinal": 1, "method": "manual", "generator": null, "is_sample": true },
    { "ordinal": 2, "method": "manual", "generator": null, "is_sample": true },
    {
      "ordinal": 3,
      "method": "generated",
      "generator": "gen 1 10",
      "is_sample": false
    },
    {
      "ordinal": 4,
      "method": "generated",
      "generator": "gen 1 100",
      "is_sample": false
    }
  ]
}
```

---

## 4) Важное про имена файлов тестов

Для публикации и загрузки пакета система ожидает формат:

```
tests/%02d.in
tests/%02d.out
```

То есть `01.in`, `01.out`, `02.in`, `02.out`, и т.д.
