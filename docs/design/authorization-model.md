# RFC: единая модель авторизации

Статус: предложение для обсуждения (Phase 1). Спецификация задачи: [issue #36](https://github.com/brawler2011/gate/issues/36).
Срез исследования: 2026-11-08. Этот RFC описывает целевое поведение; изменения кода,
контрактов, миграций и генерация в Phase 1 не выполняются. Доставка RFC — частичная
доставка #36 (`Refs #36`), а не основание закрывать issue.

## 1. Исследование текущей реализации

Изучены следующие источники (пути относительно корня репозитория):

| Источник | Наблюдение |
| --- | --- |
| `backend/internal/access/model/permissions.go` | Есть OrgAction, ProblemAction и ContestAction; TeamAction отсутствует. Набор bool permissions и маска контеста дублируют решения. |
| `backend/internal/access/permissions.go`, `ports.go`, тесты access | Go вычисляет максимум прямых и командных ролей; org owner **и admin** повышаются до contest owner / problem admin. Problem admin сейчас смешивает владение и администрирование. `use_problem` требует аутентификацию и право чтения. |
| Там же: `canSubmit`, `canViewContestProblem`, `HasOrganizationPermission` | Managers могут отправлять решения в любое время; есть post-finish upsolving; публичные условия доступны после старта; организация доступна для чтения всем. Неизвестный OrgAction для гостя сейчас обрабатывается иначе, чем для администратора. |
| `backend/internal/organizations/teams.go` | Team-политики находятся в use case. `closed` виден аутентифицированному пользователю, `secret` — участнику или org manager; список без org-фильтра сейчас возвращает только свои команды. |
| `backend/internal/postgres/access.go`, `*sql` | Transaction-bound адаптеры authorization используют `GetAuthorization*` с `FOR SHARE`. Есть отдельные lock-запросы и проверки в write-транзакциях. Одних запретов на `FOR UPDATE` недостаточно. |
| `backend/internal/postgres/{problems,contests,teams,problem_teams,contest_teams}.sql` | Часть списков уже фильтруется в SQL. Правила различаются: ListContests допускает любого org member; ListAllContests не содержит viewer-фильтра; NULL viewer в ListAllProblems отключает ограничение. Team resource queries уже имеют отдельные фильтры — их нельзя потерять. |
| `backend/internal/postgres/contests.sql` | Get/ListContestProblems соединяются с изменяемой таблицей problems, включая title/limits; для полной snapshot-изоляции этого недостаточно. |
| `backend/internal/contests/model/contest.go`, scoreboard tests | Есть nullable start/end, freeze_duration_minutes, freeze_status, scopes, participation_mode, hide_statements, enable_upsolving. `IsFrozenAt` сам по себе не снимает freeze после конца. |
| `backend/internal/transport/middleware/authz.go`, `backend/internal/transport/ws/observer/access.go` | Middleware содержит доменные правила. Observer уже проверяет сессии и permissions; целевые topics и маскирование должны сохранить существующие ограничения. |
| `backend/migrations/20260822100000_drop_permission_masks_and_access_policy.sql` | Маски уже удалены из БД. Оставшиеся API/Go-поля — legacy-представление, восстанавливать колонки не нужно. |
| `contracts/core/v1/openapi.yaml` | В исходном YAML ровно два `permissions_mask` и ни одного `permissionsMask`; точные владельцы полей перечислены в §7. |

Это не утверждение, что связанные #143, #145, #146 ещё не исправлены: часть защит
уже присутствует. Их регрессии должны остаться при миграции. Удаление старых locks
согласуется с #157; новая архитектура не должна воспроизводить их.

## 2. Архитектура и разрешение ролей

`backend/internal/access` — единственный источник доменных правил. Use case
загружает типизированный access context обычными pool reads, вызывает policy и
только затем выполняет операцию. Составные политики, например
`RequireAttachProblem(actor, contest, problem, package)` и
`RequireGrantOrgRole(actor, org, currentTargetRole, requestedRole)`, проверяют все
стороны отношения. Handler и WS используют те же use cases. Middleware проверяет
credentials/session, заполняет actor context и выполняет coarse `RequireAuth`;
не извлекает ID через reflection и не вычисляет объектные роли.

Неизвестные action/role/scope, невалидные settings и ошибки загрузки означают deny
(ошибка не превращается в «membership отсутствует»). Только подтверждённый
not-found membership означает отсутствие этого grant. Истёкшая учётная запись
получает unauthenticated, а не гостевой доступ по прежним grants. Один `now` UTC
передаётся в policy и SQL как параметр, включая равенство границ времени.

Роли сравниваются только внутри своего scope:

| Scope | Иерархия от низшей к высшей |
| --- | --- |
| Organization | member < admin < owner |
| Team | member < maintainer; управление содержащей организацией выше maintainer |
| Problem | viewer/read < moderator/write < admin < owner |
| Contest | participant < moderator < owner |

Problem `admin` — отдельный **не-владелец** в policy, соответствующий существующему
`problem_teams.permission=admin` и org administration. Прямые problem roles пока
остаются owner/moderator/viewer. `read` и `write` — алиасы viewer/moderator для
командного grant, не новые прямые роли.

Правила наследования:

1. Эффективное операционное право — максимум валидных прямых и командных grants.
   Отсутствие grant не отменяет другой grant; deny-overrides и индивидуальных
   запрещающих ACL в этой модели нет. Contest user/problem block — дополнительное
   бизнес-ограничение отправки, а не ACL на исходный Problem.
2. `owner_id` и прямой membership owner означают владение Problem/Contest. Владелец
   содержащей организации также владелец этих ресурсов для policy. Все источники
   владения учитываются явно; «может manage» не означает «owner».
3. Org admin получает Problem admin и Contest moderator, **не owner**. Он может
   управлять ресурсом, но не удалять его и не менять владельцев. Обычный org member
   не получает доступ к приватным problems/contests автоматически.
4. Глобальный admin получает операционное управление: Organization admin,
   Team org-admin, Problem admin, Contest moderator. Сам по себе global admin
   **не** основание назначать/передавать owner или удалять owner-only ресурс.
   Реальный owner-grant этого пользователя учитывается отдельно. Аварийный обход
   владения не входит в #36 и не должен появиться скрытым bypass.
5. Командные grants действуют только при `team.organization_id = resource.organization_id`
   и непосредственном активном membership пользователя в команде. `parent_team_id`
   не наследует grants. Просмотр команды не делает зрителя её участником.
6. Команда не может владеть ресурсом: целевой ContestTeam grant — participant или
   moderator; ProblemTeam — read/write/admin. Legacy contest team owner необходимо
   до миграции преобразовать в moderator с сохранением явного активного владельца
   ресурса. Это предотвращает получение ownership через управление составом команды.
7. Public visibility даёт чтение, но не редактирование и не участие в закрытом
   контесте. Для public + participation_mode=open аутентифицированный пользователь
   имеет participant-допуск; регистрация перед первой посылкой создаётся атомарно.
   by_request/invite_only требуют подтверждённого direct/team participation grant.

## 3. Полная матрица Scope × Role × Action

`✓` — разрешено, `—` — запрещено, `P` — ресурс public, `C` — команда closed,
`A` — пользователь аутентифицирован и активен, `D` — делегирование по §4,
`T` — временные/settings-условия §5. Каждая строка — отдельная policy action;
скобки объединяют только операции с полностью одинаковыми условиями.
Для ролей вне колонок применяется разрешение наследования из §2. Все неуказанные
операции запрещены. Создание ресурса проверяется в содержащем scope, а не против
ещё не существующего объекта.

### Organization

| Action (capability) | Guest | User без membership | member | admin | owner |
| --- | --- | --- | --- | --- | --- |
| view_org (`view`) | ✓ | ✓ | ✓ | ✓ | ✓ |
| manage_org (`manage`, изменение metadata/join policy) | — | — | — | ✓ | ✓ |
| delete_org (`delete`) | — | — | — | — | ✓ |
| list_org_members (`list_members`) | — | — | ✓ | ✓ | ✓ |
| grant/revoke_org_role (`manage_members`, включая invitations/approve requests) | — | — | — | D | D |
| transfer_org_owner (`transfer_owner`, также назначение owner) | — | — | — | — | D |
| leave_org (`leave`) | — | — | ✓ | ✓ | D |
| create_team (`create_team`) | — | — | — | ✓ | ✓ |
| create_problem (`create_problem`) | — | — | — | ✓ | ✓ |
| create_contest (`create_contest`) | — | — | — | ✓ | ✓ |
| join_org (`join`, только join_policy=open) | — | ✓ | — | — | — |
| request_org_membership (`request_join`, только by_request) | — | ✓ | — | — | — |

Создать организацию может активный global admin (сохранение текущего ограничения
создания); создатель становится первым owner. Для invite_only доступен accept своей
действующей invitation, с проверкой делегирования роли при выдаче и принятии.
Отмена своей заявки/отклонение своего приглашения требуют идентичности адресата.

### Team

| Action (capability) | Guest | User вне команды | member | maintainer | org admin | org owner |
| --- | --- | --- | --- | --- | --- | --- |
| view_team (`view`) | — | A ∧ C | ✓ | ✓ | ✓ | ✓ |
| list_team_members (`list_members`) | — | A ∧ C | ✓ | ✓ | ✓ | ✓ |
| update_team (`edit`) | — | — | — | ✓ | ✓ | ✓ |
| delete_team (`delete`) | — | — | — | — | ✓ | ✓ |
| grant/revoke_team_role (`manage_members`) | — | — | — | D | D | D |
| leave_team (`leave`) | — | — | ✓ | ✓ | при membership | при membership |
| list_team_problems/contests (`list_resources`) | — | A ∧ C | ✓ | ✓ | ✓ | ✓ |

`list_resources` разрешает запрос, но каждый ресурс дополнительно проходит свой
SQL view predicate. Team maintainer может назначать только member; org admin/owner
могут назначать maintainer. Team не имеет owner: её владельцы — owners организации,
поэтому инвариант последнего владельца относится к организации, а не к maintainer.
Создание/изменение resource-team links проверяется по Problem/Contest, не Team edit;
целевая команда должна принадлежать той же организации.

### Problem (вне контеста)

| Action (capability) | Guest | User без grant | viewer/read | moderator/write | admin | owner |
| --- | --- | --- | --- | --- | --- | --- |
| view_problem (`view`, statement + samples) | P | P | ✓ | ✓ | ✓ | ✓ |
| use_problem (`use`, источник attach) | — | A ∧ P | ✓ | ✓ | ✓ | ✓ |
| edit_problem (`edit`, workspace, validate, publish package) | — | — | — | ✓ | ✓ | ✓ |
| admin_problem (`manage`, metadata/visibility, ACL с D) | — | — | — | — | ✓ | ✓ |
| grant/revoke_problem_role/team (`manage_members`) | — | — | — | — | D | D |
| delete_problem (`delete`) | — | — | — | — | — | ✓ |
| transfer_problem_owner (`transfer_owner`, также назначение owner) | — | — | — | — | — | D |
| list_problem_members/teams (`list_members`) | — | — | — | — | ✓ | ✓ |

`use` не даёт скачать полный judge package, тесты, checker или авторские решения.
Пакет для attach загружается сервером. Выдача авторских файлов требует `edit`;
statement-доступ не должен превращаться в доступ к архиву пакета.

### Contest

| Action (capability) | Guest | User без grant | participant | moderator | owner |
| --- | --- | --- | --- | --- | --- |
| get_contest (`view`, metadata) | P | P | ✓ | ✓ | ✓ |
| get_contest_problem (`view_statements`) | P ∧ T | P ∧ T | T | ✓ | ✓ |
| update_contest (`edit`, metadata/settings/times) | — | — | — | ✓ | ✓ |
| admin_contest (`manage_members`, ACL с D) | — | — | — | D | D |
| manage_contest (`manage`, порядок задач, attach/detach/relink, moderation) | — | — | — | ✓ | ✓ |
| delete_contest (`delete`, новая явная action) | — | — | — | — | ✓ |
| transfer_contest_owner (`transfer_owner`, также назначение owner) | — | — | — | — | D |
| get_monitor (`view_scoreboard`) | P ∧ T | P ∧ T | T | ✓ | ✓ |
| view_live_scoreboard (`view_live_scoreboard`, обход freeze) | — | — | — | ✓ | ✓ |
| list_users_submissions (`list_submissions`) | P ∧ T | P ∧ T | T | ✓ | ✓ |
| list_own_submissions (`list_own_submissions`) | — | — | ✓ | ✓ | ✓ |
| get_own_submission (`view_own_submission`) | — | — | ✓ | ✓ | ✓ |
| get_other_user_submission (`view_other_submission`) | P ∧ T | P ∧ T | T | ✓ | ✓ |
| get_submission_details (`view_submission_details`) | P ∧ T | P ∧ T | T | ✓ | ✓ |
| create_submission (`submit`) | — | — | T | T | T |
| list_contest_members/teams (`list_members`) | — | — | — | ✓ | ✓ |

«Own» всегда проверяет `submission.owner_id = actor.id` и принадлежность текущему
контесту; одно совпадение user ID недостаточно. Details требуют доступ к самой
посылке **и** соответствующий details scope. Managers не получают submit bypass:
официальная посылка дополнительно требует participant-допуска и временного окна.
Тестирование manager до старта относится к workspace/package testing по Problem
edit, не к официальным Contest submissions.

Планируемые изменения поведения относительно текущего кода: разделение owner и
admin; org admin/global admin без implicit ownership; moderator может менять
обычные настройки контеста, но не owners; единое окно official submit; completed
contest снимает freeze. Эти решения предлагаются RFC и вступают в силу только
в последующих реализациях после согласования дизайна.

## 4. Делегирование и последний владелец

Для не-владельца с ролью R разрешение на ACL-операцию требует соответствующую
`manage_members` action **и** `oldTarget < R` (если grant существует) **и**
`newTarget < R` (если grant создаётся/изменяется). Проверка только новой роли
позволила бы admin снять owner через понижение. Применяется к добавлению,
повышению, понижению, удалению, приглашениям и командным grants. Пользователь
не может повысить себя. Самостоятельный выход — отдельная action.

Owner может выдавать/отзывать роли `target ≤ R`, включая owner. Назначение,
передача и отзыв ownership требуют реального resource owner или owner содержащей
организации; role rank, global admin, `admin_problem` и `manage_contest` сами по
себе не доказательство владения. Org admin может назначить org member, но не admin
и не owner; Contest moderator — participant; Problem admin — moderator/viewer,
а также team read/write, но не team admin. Владельцы управляют team admin.

Ownership организации канонически хранится в active owner memberships. Для
Problem/Contest нужно нормализовать `owner_id` и direct owner memberships в один
непротиворечивый набор владельцев: `owner_id`, если используется как primary owner,
обязан ссылаться на active direct owner membership. Inherited org owners дают
право управления, но не заменяют требование минимум одного явного активного
владельца ресурса. Создание включает первого owner; transfer атомарно добавляет
нового, меняет primary owner и лишь затем снимает старого при необходимости.

Нельзя удалить, понизить, исключить из организации, деактивировать или дать истечь
последнему активному явному владельцу существующей организации/Problem/Contest.
Temporary/expiring accounts не могут принимать ownership; существующие данные
проходят аудит перед включением инварианта. Удаление самого ресурса owner-ом
допустимо; каскадное удаление организации проверяется отдельной owner action.

Precheck числа owners не защищает от двух конкурентных уходов. Mutation repository
поддерживает инвариант атомарно: короткая SERIALIZABLE write-транзакция читает
активный owner set и изменяет memberships; write skew приводит к serialization
failure. После bounded retry выполняется **новый pool precheck** и новая транзакция;
при невозможности сохранить владельца возвращается conflict. Все пути изменения
owners/account lifecycle обязаны участвовать в этом протоколе, включая expiry и
массовое удаление. Проверка инварианта — целостность данных, не повторный вызов
access внутри транзакции; `SELECT ... FOR UPDATE/SHARE` для неё не требуется.

## 5. Время контеста и изоляция snapshot

Phase вычисляется по одному server `now`; EndTime имеет приоритет над freeze:

| Phase | Точное условие | Statements участника | Official submit | Scoreboard public/participant | Manager |
| --- | --- | --- | --- | --- | --- |
| Pre-start | start != NULL и now < start | запрещены | запрещён | запрещён | preview statements/scoreboard |
| Running | старт наступил, конец не наступил, freeze не активен | разрешены, если !hide_statements | разрешён eligible participant | по monitor_scope | live |
| Scoreboard Freeze | Running window и freeze активен | как Running | как Running | masked, по monitor_scope | live |
| Finished | end != NULL и now >= end | разрешены, если !hide_statements | запрещён | final, без freeze, по monitor_scope | final |

`start=NULL` означает «старт уже наступил», `end=NULL` — без срока окончания;
если оба заданы, требуется start < end. Равенство start разрешает submit, равенство
end запрещает. `freeze_status=auto`: freeze начинается в
`max(start, end - freeze_duration_minutes)` при заданном end и duration > 0;
без start используется end-duration. `frozen` действует только в Running window
с момента явного переключения (его timestamp фиксируется); `unfrozen` отключает
freeze. Finished всегда сильнее `frozen`. Без end возможен только ручной freeze.

Дополнительные условия `T`:

- Statements для public spectator доступны после старта при !hide_statements;
  private contest требует participant/manager grant. Участие не проверяется через
  Problem ACL. `hide_statements` ограничивает неменеджеров и после финиша.
- `monitor_scope`, `submissions_list_scope`, `submissions_review_scope`,
  `submission_details_scope` независимо задают минимальную аудиторию:
  public — любой viewer контеста, participant — participant, moderator — manager.
  Для неменеджеров эти действия доступны только после старта. Значения по умолчанию
  сохраняются как нормализация существующих persisted settings: participant для
  monitor, moderator для остальных. Новые неизвестные значения отклоняются.
- Во freeze REST, WS, история и replay возвращают одинаковую masked projection.
  Публичная standings projection не содержит post-freeze verdict, solved/penalty
  changes, тестов или иных полей, раскрывающих результат; допустим pending count.
  Изменение verdict старой посылки после freeze также не раскрывается: projection
  фиксирует известные результаты на границе freeze, а не только created_at посылки.
- Участник видит собственные verdicts по own actions; это не даёт читать чужие
  freeze-результаты. Details scope не снимает freeze для чужих посылок. Manager
  получает live projection только после `view_live_scoreboard`.
- Официальные отправки закрываются для всех на end. `enable_upsolving` может
  включать отдельный practice flow с отдельными посылками без влияния на official
  standings; он не расширяет `create_submission` этого RFC. Реализация practice и
  virtual sessions не входит в следующие PR #36; нельзя молча трактовать текущий
  флаг как обход временного окна официального контеста.

### Contest–Problem boundary

Attach/relink требует конъюнкцию: `manage_contest(destination)` AND
`use_problem(source)` AND `package.problem_id = source.id` AND `package.status=ready`.
Авторизация источника выполняется и для чужой организации; владение destination
само по себе не даёт прав на private source. Cross-org attach разрешён при всех
этих условиях; cross-org team ACL grants запрещены. Detach/reorder требуют только
manage destination. Relink в любой фазе, включая Running, доступен только managers
с повторной проверкой нового источника/пакета; участник никогда не выбирает package.

После attach `contest_problems.package_id` закрепляет immutable snapshot. Participant
read path использует **только Contest context + ContestProblem + pinned Package**:
ни `HasProblemPermission`, ни чтения/JOIN `problems`, ни workspace/latest package.
Title, limits, statements, samples и timestamps в ContestProblem DTO происходят
из snapshot и contest link; problem_id остаётся идентификатором связи. Изменение
visibility/owner/текста исходного Problem не меняет доступ участника и snapshot.
Published package и необходимые метаданные удерживаются пока есть contest/submission
references; удаление исходного Problem не должно каскадно уничтожать snapshot.

Каждая submission фиксирует package_id на момент приёма. Relink влияет только на
последующие чтения/отправки; уже поставленная в judge очередь посылка использует
свой package_id. Participant получает только statement projection, не URL полного
архива с тестами. Авторизованный source access нужен manager на attach/relink,
но не участнику при каждой загрузке условия.

## 6. Стандартные SQL-предикаты и пагинация

Ниже SQL-шаблоны для будущих sqlc queries, а не новые SQL-функции или миграции.
`:viewer_id` — UUID или NULL для гостя, `:now` — общий timestamp; параметры задаёт
сервер. Клиент не передаёт is_admin/can_manage. NULL **никогда не отключает** фильтр.
Expired authenticated actor отклоняется до запроса; CTE дополнительно не допускает
его private grants. Public reads допускаются для гостя.

Общий префикс каждого запроса (далее alias `a`):

```sql
WITH actor AS (
  SELECT u.id, u.role
  FROM users u
  WHERE u.id = :viewer_id
    AND (u.expires_at IS NULL OR u.expires_at > :now)
)
```

### Problems: `VIEW_PROBLEM(p)`

```sql
WHERE (
  p.visibility = 'public'
  OR EXISTS (
    SELECT 1 FROM actor a
    WHERE a.role = 'admin'
      OR p.owner_id = a.id
      OR EXISTS (
        SELECT 1 FROM organization_members om
        WHERE om.organization_id = p.organization_id AND om.user_id = a.id
          AND om.role IN ('owner', 'admin'))
      OR EXISTS (
        SELECT 1 FROM problem_members pm
        WHERE pm.problem_id = p.id AND pm.user_id = a.id
          AND pm.role IN ('viewer', 'moderator', 'owner'))
      OR EXISTS (
        SELECT 1 FROM problem_teams pt
        JOIN teams t ON t.id = pt.team_id
        JOIN team_members tm ON tm.team_id = t.id
        WHERE pt.problem_id = p.id AND tm.user_id = a.id
          AND t.organization_id = p.organization_id
          AND tm.role IN ('member', 'maintainer')
          AND pt.permission IN ('read', 'write', 'admin'))
  )
)
```

### Contests: `VIEW_CONTEST(c)`

```sql
WHERE (
  c.visibility = 'public'
  OR EXISTS (
    SELECT 1 FROM actor a
    WHERE a.role = 'admin'
      OR c.owner_id = a.id
      OR EXISTS (
        SELECT 1 FROM organization_members om
        WHERE om.organization_id = c.organization_id AND om.user_id = a.id
          AND om.role IN ('owner', 'admin'))
      OR EXISTS (
        SELECT 1 FROM contest_members cm
        WHERE cm.contest_id = c.id AND cm.user_id = a.id
          AND cm.role IN ('participant', 'moderator', 'owner'))
      OR EXISTS (
        SELECT 1 FROM contest_teams ct
        JOIN teams t ON t.id = ct.team_id
        JOIN team_members tm ON tm.team_id = t.id
        WHERE ct.contest_id = c.id AND tm.user_id = a.id
          AND t.organization_id = c.organization_id
          AND tm.role IN ('member', 'maintainer')
          AND ct.role IN ('participant', 'moderator'))
  )
)
```

Это только видимость metadata; statement/submit/freeze проверяются отдельно.
Open participation не добавляет private contests в результат. Любой org member
без resource grant видит только public contests, независимо от organization_id
в фильтре. Legacy `user_has_contest_access` заменяется этим предикатом, а не
сохраняется второй реализацией правил.

### Teams: `VIEW_TEAM(t)`

```sql
WHERE EXISTS (
  SELECT 1 FROM actor a
  WHERE t.privacy = 'closed'
    OR a.role = 'admin'
    OR EXISTS (
      SELECT 1 FROM organization_members om
      WHERE om.organization_id = t.organization_id AND om.user_id = a.id
        AND om.role IN ('owner', 'admin'))
    OR EXISTS (
      SELECT 1 FROM team_members tm
      WHERE tm.team_id = t.id AND tm.user_id = a.id
        AND tm.role IN ('member', 'maintainer'))
)
```

Одинаковая семантика Get и List независимо от наличия organization_id. Отдельный
«мои команды» — дополнительный membership filter, не подмена view predicate.
Это намеренно унифицирует текущий unscoped ListTeams с GetTeam: closed доступен
активному пользователю, secret — своим members и org/global administration.

### Применение к спискам

Каждый List/Count (общий, по org, dashboard, team-linked) использует один и тот же
соответствующий view predicate **до** COUNT, LIMIT и OFFSET. Search, visibility,
organization_id, owner_id, is_template только сужают выборку через AND.
Для team-linked resources запрос дополнительно содержит:

```sql
-- Для problems; для contests аналогично contest_teams/contest_id.
AND EXISTS (
  SELECT 1 FROM problem_teams link
  WHERE link.problem_id = p.id AND link.team_id = :team_id
)
```

Use case предварительно требует `view_team(team_id)`. Это не заменяет
`VIEW_PROBLEM(p)` / `VIEW_CONTEST(c)` в SQL: зритель closed-команды не получает
доступ к её private resources (#145). Membership в другой допустимой команде или
прямой grant, напротив, может дать доступ к ресурсу.

Форма реализации: `visible AS (SELECT ... FROM resources WHERE <view> AND <filters>)`,
затем `total AS (SELECT COUNT(*) FROM visible)` и `page AS (SELECT ... FROM visible
ORDER BY created_at DESC, id DESC LIMIT :limit OFFSET :offset)` в **одном statement**.
`total LEFT JOIN page ON TRUE` сохраняет total для пустой страницы; адаптер удаляет
только sentinel пустой строки, не фильтрует права в памяти. При иной сортировке
всегда добавляется уникальный id tie-breaker. Общий statement snapshot гарантирует
согласованные count/page без write-транзакции. Все grants выражаются EXISTS,
поэтому несколько команд не размножают строки и COUNT.

Capabilities для возвращённой страницы вычисляются из batch-loaded grant facts
и общей policy; число запросов ограничено числом типов facts, не длиной страницы.
Нельзя сначала LIMIT, затем вызвать HasPermission на каждой строке и отбрасывать
недоступные ресурсы. Это относится и к dashboard с коротким списком.

## 7. Точный план DTO и capabilities

Исследован `components.schemas` в `contracts/core/v1/openapi.yaml`. Целевой список
**16 существующих схем**, в которые добавляется обязательный `capabilities`:

| Schema | Scope/семантика capabilities | Изменение |
| --- | --- | --- |
| OrganizationModel | Organization текущего viewer | добавить |
| TeamModel | Team текущего viewer | добавить |
| ProblemModel | исходный Problem текущего viewer | добавить |
| ProblemsListItemModel | тот же Problem, list projection | добавить |
| DashboardProblemModel | тот же Problem, dashboard projection | добавить |
| ContestModel | Contest текущего viewer | добавить; покрывает оба contest lists и detail |
| DashboardContestModel | тот же Contest, dashboard projection | добавить |
| ContestProblemModel | **ContestProblem**, а не исходный Problem | добавить |
| ContestProblemListItemModel | тот же ContestProblem, list projection | добавить |
| GetMyContestRoleResponseModel | Contest текущего viewer | заменить permissions_mask |
| ContestTeamModel | действия viewer над конкретным ACL link | удалить permissions_mask, добавить capabilities |
| ProblemTeamModel | действия viewer над конкретным ACL link | добавить |
| ContestMemberModel | действия viewer над конкретным membership | добавить |
| OrganizationMemberModel | действия viewer над конкретным membership | добавить |
| TeamMemberModel | действия viewer над конкретным membership | добавить |
| ProblemMemberModel | действия viewer над конкретным membership | добавить |

Новых resource DTO для этой миграции не требуется.

Member/link capabilities отвечают на вопрос «что текущий viewer может сделать с
этой строкой»: `change_role`, `remove`, а для direct owner — `transfer_owner`, если
позволяют §4 и last-owner invariant. Они **не** представляют права другого
пользователя/всей команды. `role`, `contest_role` и `permission` остаются описанием
назначенного grant. В частности, legacy mask в ContestTeam нельзя механически
преобразовать в capabilities текущего viewer. `change_role` не разрешает произвольную
целевую роль: target-specific server policy обязательна при каждом изменении.

Для ContestProblem используются только `view_statements`, `submit` (если не blocked),
`relink_package`, `remove`, `reorder` (последние три требуют Contest manage).
Нельзя выдавать `edit`, `delete` или `use` исходного Problem на основе Contest role.
Для остальных ресурсов vocabulary задано в матрицах. `manage_members` означает
возможность хотя бы одного допустимого изменения, не право обойти иерархию.

Общая форма поля во всех перечисленных схемах:

```yaml
capabilities:
  type: array
  readOnly: true
  uniqueItems: true
  items:
    type: string
  example: [view, edit, manage]
```

Поле включается в required, никогда не null/omitted; отсутствие разрешений — `[]`.
Значения стабильны, без битовых позиций; порядок лексикографический. Это вычисленная
viewer-specific проекция policy + времени + состояния, не хранимая ACL. UI проверяет
строки и игнорирует незнакомые; сервер заново авторизует каждый запрос. Cache должен
учитывать viewer и revision прав и истекать на следующей временной границе. Public
ответ с private capabilities нельзя отдавать через общий кэш.

Response wrappers (`GetOrganizationResponseModel`, `GetTeamResponseModel`,
`GetContestResponseModel`, `GetContestProblemResponseModel`, `List*ResponseModel`,
`GetUserDashboardResponseModel` и аналогичные) используют вложенные resource refs;
второй массив на wrapper не добавляется. `UserModel`, package/statement value
objects, scoreboard cells, submissions, invitations, join requests, announcements,
clarifications и drafts не получают capabilities в рамках этой миграции четырёх
scopes: их операции проверяются use cases через родительский scope/идентичность.
Request DTO не получают ни capabilities, ни replacement mask.

В OpenAPI нет `permissions_mask` у ContestMemberModel или ContestModel; не следует
приписывать им несуществующие поля. В frontend legacy имя может быть camelCase
`permissionsMask`: клиентские производные маски удаляются вместе с адаптерами и
выводом прав из role. Сначала изменяется YAML, затем `task gen:contracts` и клиент;
генерируемые `backend/contracts` и `frontend/contracts` вручную не редактируются.
Issue не имеет backward-compatible label: план — атомарная замена API+UI в одном PR,
без поддержки двух конкурирующих моделей.

## 8. Lock-free access и граница транзакции

Гарантия для всех новых policies и мигрированных путей:

- Access не открывает `Within`/write-транзакции и не принимает transaction-bound
  repositories. Read ports связываются только с pool-backed nonlocking queries.
- В authorization запрещены все row-locks: `FOR UPDATE`, `FOR NO KEY UPDATE`,
  `FOR SHARE`, `FOR KEY SHARE`, `Lock*`, а также advisory locks, удерживающие grants.
  GET/list/WS admission не запускают write-транзакции ради проверки прав.
- Policy вычисляется до записи по одному typed context; capability computation
  использует те же функции. Запрещён «повторный access check внутри tx для надёжности».
- Обычный single-row UPDATE не требует явной multi-row транзакции. `Within` остаётся
  только там, где несколько мутаций должны быть атомарны (owner transfer,
  submission+outbox, attach/order и т.п.). Присущие DML блокировки записываемых строк
  не являются authorization locks и не удерживаются на read-only policy paths.

Разделение precheck и записи создаёт TOCTOU. Целевой контракт отзыва: после
завершённого revoke новый запрос не может использовать старый grant; незавершённая
команда также не должна записать данные по устаревшему precheck. Для этого future
mutation layer использует optimistic revisions, не блокировку access:

1. Nonlocking statement snapshot загружает policy facts и security revisions всех
   зависимостей (account/session, org, team membership/grant, resource/link).
2. Policy выдаёт решение и opaque decision token с actor/action/resource,
   revisions и `valid_until` ближайшей temporal границы. Это внутренний токен,
   capabilities клиента им не являются.
3. Атомарный write guard проверяет revisions и valid_until. При multi-row mutation
   SERIALIZABLE транзакция читает revision rows без row-locks и выполняет запись;
   все revoke/change paths атомарно увеличивают соответствующие revisions. Это
   чтение версии, не повторное вычисление policy. Для одной мутации допустим
   guarded single SQL statement с эквивалентным optimistic протоколом.
4. Если revoke пересёк команду, guard/revision mismatch либо serialization conflict
   требует rollback и нового внешнего precheck; нельзя повторить старое решение.
   Команда, сериализованная до revoke, может завершиться; это явная точка порядка.

Сейчас такой общей revision-схемы нет: её проектирование/миграция — обязательная
часть первого implementation PR, а не обещание существующей гарантии. Без guard
невозможно заявлять немедленный отзыв только на основании pool precheck. Запрет
row-locks не означает отсутствие внутренних DML/MVCC locks PostgreSQL.

## 9. WebSocket scope, freeze и отзыв

Topic families: `contest/<id>/public`, `contest/<id>/participants`,
`contest/<id>/managers`, `contest/<id>/own/<user_id>`. Подписка разрешается сервером
через access для конкретной audience/action; клиент не выбирает manager/чужой own
topic произвольным строковым именем. Public topic содержит только данные, доступные
public scope; participant topic требует participation. Settings moderator-only
не должны попадать в более широкую audience.

Producer создаёт audience projections один раз на событие: masked standings для
public/participants, live для managers, собственные verdicts для own user. Полный
judge event не публикуется в общий topic с надеждой на UI-фильтр. История/replay
хранятся/читаются в той же audience projection; переход в freeze не должен позволять
дочитать buffered live payload из старой подписки. Finish публикует final snapshot.

Grant/session/account revisions инвалидируют связанные subscriptions. На revoke
сервер блокирует доставку до reauthorization, очищает очередь прежней audience и
закрывает либо переводит подписку на допустимый topic; cache failure/disconnect от
канала invalidation означает fail closed. Проверки выполняются при subscribe,
reconnect/replay, изменении revisions/settings и temporal transitions. Никакой
per-client фильтрации payload на каждое сообщение; admission/revocation lifecycle
проверяет только валидность подписки. Own topic не сохраняет доступ после потери
права читать private contest.

## 10. Декомпозиция после согласования RFC

Следующие **4 небольших PR** доставляют отдельные проверяемые вертикальные части;
Phase 1 не включает ни один из них. Каждый PR ссылается `Refs #36`; последний
использует `Closes #36` только после проверки всех acceptance criteria.

1. **Policies, роли и безопасные grants.** Typed contexts/actions для четырёх scopes,
   чистые policy evaluators и shared scenario fixtures; разделение owner/admin,
   grant/transfer/last-owner invariants. Nonlocking read ports и optimistic revision
   guard для мигрируемых ACL mutations; аудит owner data/legacy team owner. Проверки:
   admin-to-owner #146, понижение чужого owner, равные роли, последний owner,
   конкурентный transfer/leave/revoke. Остальные handlers пока используют старые
   проверки; новые не подключаются без необходимых revision guards.
2. **SQL visibility и metadata reads.** Внедрение предикатов §6 во все List/Count,
   team resources и dashboard, batch capabilities facts, typed use-case checks на
   metadata reads. Удаление доменных middleware checks только для мигрированных
   операций; перенос legacy read locks согласован с #157. Проверки: #145, гости,
   expired actor, direct/team grants, secret/closed, org filters, COUNT/пустые страницы,
   отсутствие дублей, unknown roles. Go policy ↔ SQL parity на общей матрице.
3. **Contest lifecycle и snapshots.** Attach/relink composite policy, pinned read
   path без Problem, submission package pinning, точные временные границы,
   freeze/final projections. Перенос оставшихся object/relationship checks в access
   и удаление их из middleware; guard для contest commands. Проверки: #143, foreign
   private source, wrong/not-ready package, публикация новой source версии,
   relink во время running, старт/end/freeze equality, own/other details scopes.
4. **Контракты, UI и WS.** Все 16 DTO §7, удаление масок, `task gen:contracts`,
   UI на capabilities; topics и revocation §9 поверх готовых lifecycle projections.
   Проверки содержимого REST/WS/replay при freeze, смене роли/сессии, reconnect,
   invalidation failure; capability parity на list/detail/dashboard/role endpoint.

Критерии готовности реализации: реальные PostgreSQL integration tests для
#143/#145/#146 и конкурентных инвариантов; fixtures покрывают каждую разрешающую и
запрещающую ветку матриц, team grants и global admin; trace/query assertions
доказывают отсутствие transaction/row-lock на authorization reads и отсутствие N+1.
Для каждого implementation PR — `task precommit`, для PostgreSQL сценариев —
`task test:be:integration`. RFC проверяется по исходникам и полноте правил;
он не утверждает, что эти будущие тесты уже написаны или пройдены.
