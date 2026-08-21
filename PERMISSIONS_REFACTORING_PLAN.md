# Архитектурный план: Рефакторинг системы прав доступа (RBAC + ABAC)

## 1. Согласованные архитектурные решения

1. **GitHub-style модель сущностей и наследование**:
   - **Организация** = GitHub Organization (верхнеуровневый контейнер).
   - `Org Owner` и `Org Admin` автоматически получают полные права управления (`RoleOwner` / `ProblemRoleOwner`) на все контесты и задачи своей организации.
   - `Org Member` по умолчанию наследует только публичные права, либо доступ через команды или прямое добавление.
   - Пользователь **обязан** быть участником организации, чтобы его можно было добавить в команду или напрямую в контест/задачу этой организации.
   - При исключении пользователя из организации выполняется каскадное удаление из всех команд, контестов и задач внутри организации.
   - Публичные ресурсы (`visibility == "public"`) доступны для чтения всем пользователям и гостям.

2. **Двухфазный движок авторизации (RBAC + ABAC)**:
   - **Фаза 1 (RBAC - Subject Resolution)**: вычисление эффективной роли $R_{eff} \in \{\text{RoleGuest}, \text{RoleNone}, \text{RoleParticipant}, \text{RoleModerator}, \text{RoleOwner}\}$:
     $$R_{eff} = \max(\text{GlobalAdmin}, \text{OrgAdmin/Owner}, \text{DirectRole}, \max(\text{TeamRoles}), \text{PublicOpenParticipant})$$
   - **Фаза 2 (ABAC - Rule Engine)**: вычисление прав через чистые функции от $(R_{eff}, \text{ResourceAttributes}, \text{EnvironmentContext})$.
   - **Полная ликвидация статических масок**: удаление статической колонки `permissions_mask` в таблицах `contest_members` и `contest_teams`, а также устранение дублирующего слоя `AccessPolicy`. Права всегда вычисляются динамически на лету.

3. **Режимы видимости и участия**:
   - `visibility`: `public` | `private`.
   - `participation_mode`: `open` | `invite_only`.
     - `open`: любой авторизованный пользователь динамически получает роль `RoleParticipant` и может отправлять решения во время контеста без предварительной регистрации.
     - `invite_only`: отправка решений разрешена только явно добавленным участникам или командам.

4. **Жизненный цикл контестов и дорешивание (Codeforces-style)**:
   - До старта контеста: посылки запрещены всем, кроме модераторов/админов; условия скрыты.
   - Во время контеста: участники могут отправлять решения.
   - После окончания: при `enable_upsolving: true` любой авторизованный пользователь с доступом к просмотру контеста может отправлять решения в режиме дорешивания (вне зачета).
   - Монитор: доступ вычисляется динамически на основе `monitor_scope` (`public`, `participant`, `moderator`) и параметров заморозки (`freeze_status`, `freeze_duration_minutes`).

---

## 2. Исправление 7 выявленных багов

| № | Проблема | Архитектурное решение |
|---|---|---|
| 1 | В настройках контеста нет тумблера для отключения «Черновиков» | Добавить `enable_drafts` (bool) в `ContestSettings`, OpenAPI контракты, бэкенд и интерфейс настроек контеста. |
| 2 | Для участника не отображается вкладка «Отправить решение», хотя из задачи отправить можно | Устранить статический `permissions_mask = 0`, синхронизировать `canSubmitSolution` с бэкенд-авторизацией. |
| 3 | Изменение настроек монитора ничего не меняет (участник не может смотреть) | Вычислять `GetMonitor` динамически от актуального `monitor_scope` в `contest.settings` без переопределения статической маской. |
| 4 | Можно добавить пользователя в контест без членства в организации | Внедрить валидацию в UseCase: добавление в контест/задачу/команду требует предварительного `org_members`. При удалении из org — каскадное удаление. |
| 5 | Пользователь не из организации видит список приватных контестов и задач | Добавить фильтрацию в `ListOrganizationContests` и `ListProblems`: не-члены видят только `public` ресурсы. |
| 6 | Участник организации видит вкладку «Настройки» и получает 403 | Показывать вкладки «Настройки» и «Участники» в шапке организации только пользователям с правами `manage_org` (`owner`, `admin`, `global_admin`). |
| 7 | Ошибка в интерфейсе при создании команды из админки и переходе в неё | Исправить обработку админских прав в `TeamsUseCase`, загрузку команды и привязанных контестов/задач. |

---

## 3. План реализации по TDD (Test-Driven Development) & DoD

```
1. [Backend Unit & Domain Tests] → verify: go test -v ./internal/domain/... ./internal/usecase/...
2. [Backend Permission Engine & UseCases] → verify: go test -v ./internal/usecase/...
3. [Backend Integration & API Authz Tests] → verify: go test -v -tags=integration ./tests/integration/...
4. [Backend Migrations & Handlers Codegen] → verify: task verify-codegen && go test ./...
5. [Frontend Unit & Integration Tests (bun test)] → verify: bun test
6. [Frontend UI & Component Updates] → verify: bun run typecheck && bun run lint && bun test
7. [End-to-End Multi-user Verification (Playwright)] → verify: task test:e2e
```

### Этап 1: Бэкенд TDD (Domain & Unit Tests)
1. Табличные юнит-тесты авторизации (`backend/internal/usecase/permissions_test.go`):
   - Матрица всех ролей (`Guest`, `NonMember`, `OrgMember`, `TeamParticipant`, `TeamModerator`, `DirectParticipant`, `DirectModerator`, `OrgAdmin`, `OrgOwner`, `GlobalAdmin`).
   - Матрица всех действий (`GetContest`, `GetContestProblem`, `CreateSubmission`, `GetMonitor`, `ListSubmissions`, `ManageContest`, `Drafts`, `Upsolving`).
   - Все состояния контеста (`pre-start`, `running`, `frozen`, `finished`, `virtual`, `upsolving`).
2. Юнит-тесты валидаций членства в организации при добавлении в контест/команду/задачу.

### Этап 2: Бэкенд реализация (Движок RBAC/ABAC и UseCases)
1. Реализация структуры `AuthzEngine` в `backend/internal/usecase/permissions.go`.
2. Обновление `contestsUC`, `problemsUC`, `organizationsUC`, `teamsUC`, `draftsUC`.
3. Каскадное удаление членств при удалении пользователя из организации.
4. Обновление фильтрации приватных контестов/задач для сторонних пользователей.

### Этап 3: Бэкенд миграции, контракты и интеграционные тесты
1. Миграция базы данных: удаление `permissions_mask` и `access_policy`.
2. Обновление sqlc-файлов и генерация кода.
3. Обновление `openapi.yaml` (`enable_drafts`, `enable_upsolving`, `enable_virtual_contests`, `participation_mode`) и кодогенерация (`task gen`).
4. Обновление и запуск интеграционных тестов (`tests/integration/permissions_matrix_test.go`, `authz_middleware_test.go`).

### Этап 4: Фронтенд TDD и реализация
1. Добавление скрипта `"test": "bun test"` в `frontend/package.json`.
2. Юнит-тесты на `bun:test` для `frontend/lib/permissions.ts`.
3. Удаление устаревших статических масок и запросов из `permissions.ts`.
4. Обновление компонентов `ContestHeaderNav`, `OrgHeaderNav`, `SettingsSection`, `OrgTeamsTab`, `TeamMembersManagement`.
5. Валидация: `bun run typecheck`, `bun run lint`, `bun test`.

### Этап 5: Сквозное E2E тестирование
1. Playwright тесты для многопользовательских сценариев.
2. Проверка через `task test:e2e`.
