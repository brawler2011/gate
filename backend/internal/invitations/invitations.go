// Package invitations brings users into organizations, teams and contests with their consent:
// named invitations they accept or decline, and join links anyone logged in can follow.
package invitations

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	notificationsmodel "github.com/brawler2011/gate/backend/internal/notifications/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

type Kind string

const (
	KindOrganization Kind = "organization"
	KindTeam         Kind = "team"
	KindContest      Kind = "contest"
)

// Target names what an invitation or link leads to; Team* and Contest* are set for their kind only.
type Target struct {
	Kind              Kind
	ID                uuid.UUID
	OrganizationID    uuid.UUID
	OrganizationLogin string
	OrganizationName  string
	TeamName          string
	ContestLogin      string
	ContestTitle      string
}

type Invitation struct {
	ID              uuid.UUID
	Target          Target
	UserID          uuid.UUID
	Username        string
	InviterUsername string
	Role            string
	CreatedAt       time.Time
}

type NewInvitation struct {
	Target    Target
	UserID    uuid.UUID
	InviterID uuid.UUID
	Role      string
}

type Repo interface {
	OrganizationID(ctx context.Context, login string) (uuid.UUID, error)
	GetInvitation(ctx context.Context, id uuid.UUID) (Invitation, error)
	ListInvitations(ctx context.Context, target Target) ([]Invitation, error)
	JoinLink(ctx context.Context, target Target) (*string, error)
	JoinLinkTarget(ctx context.Context, token string) (Target, error)
}

type TxStore interface {
	access.Loader
	Repo
	GetUserById(ctx context.Context, id uuid.UUID) (accessmodel.User, error)
	CreateNotification(ctx context.Context, input *notificationsmodel.CreateNotificationInput) (*notificationsmodel.Notification, error)
	CreateInvitation(ctx context.Context, input NewInvitation) (uuid.UUID, error)
	// DeleteInvitation fails with ErrRequestResolved when the invitation is already gone.
	DeleteInvitation(ctx context.Context, id uuid.UUID) error
	SetJoinLink(ctx context.Context, target Target, token string, inviterID uuid.UUID) error
	DeleteJoinLink(ctx context.Context, target Target) error
	// OrganizationRole and ContestRole return "" for users without one.
	OrganizationRole(ctx context.Context, orgID, userID uuid.UUID) (accessmodel.OrganizationRole, error)
	ContestRole(ctx context.Context, contestID, userID uuid.UUID) (accessmodel.ContestRole, error)
	// The Join methods keep an existing membership as it is.
	JoinOrganization(ctx context.Context, orgID, userID uuid.UUID, role accessmodel.OrganizationRole) error
	JoinTeam(ctx context.Context, teamID, userID uuid.UUID) error
	JoinContest(ctx context.Context, contestID, userID uuid.UUID) error
}

type Transactions interface {
	Within(context.Context, func(TxStore) error) error
}

type Emails interface {
	SendEmail(context.Context, notificationsmodel.Notification)
}

type UseCase struct {
	repo   Repo
	access access.Loader
	tx     Transactions
	emails Emails
}

func NewUseCase(repo Repo, loader access.Loader, tx Transactions, emails Emails) *UseCase {
	return &UseCase{repo: repo, access: loader, tx: tx, emails: emails}
}

func (uc *UseCase) InviteToOrganization(ctx context.Context, orgLogin string, userID uuid.UUID, role accessmodel.OrganizationRole) (*Invitation, error) {
	orgID, err := uc.repo.OrganizationID(ctx, orgLogin)
	if err != nil {
		return nil, err
	}
	var invitation Invitation
	var notification *notificationsmodel.Notification
	err = access.WithinOrganization(ctx, uc.tx, orgID, accessmodel.OrganizationInvite, func(tx TxStore, a access.OrganizationAccess) error {
		if err := a.Invite(userID, role); err != nil {
			return err
		}
		current, err := tx.OrganizationRole(ctx, orgID, userID)
		if err != nil {
			return err
		}
		if current != "" {
			return pkg.Wrap(pkg.ErrAlreadyMember, nil, "user is already a member of the organization")
		}
		invitation, notification, err = invite(ctx, tx, NewInvitation{Target: Target{Kind: KindOrganization, ID: orgID}, UserID: userID, InviterID: a.Actor.Id, Role: string(role)})
		return err
	})
	if err != nil {
		return nil, err
	}
	uc.emails.SendEmail(ctx, *notification)
	return &invitation, nil
}

func (uc *UseCase) InviteToContest(ctx context.Context, contestID, userID uuid.UUID) (*Invitation, error) {
	var invitation Invitation
	var notification *notificationsmodel.Notification
	err := access.WithinContest(ctx, uc.tx, contestID, accessmodel.ContestManage, func(tx TxStore, a access.ContestAccess) error {
		current, err := tx.ContestRole(ctx, contestID, userID)
		if err != nil {
			return err
		}
		if current != "" {
			return pkg.Wrap(pkg.ErrAlreadyMember, nil, "user already takes part in the contest")
		}
		invitation, notification, err = invite(ctx, tx, NewInvitation{Target: Target{Kind: KindContest, ID: contestID}, UserID: userID, InviterID: a.Actor.Id, Role: string(accessmodel.ContestParticipant)})
		return err
	})
	if err != nil {
		return nil, err
	}
	uc.emails.SendEmail(ctx, *notification)
	return &invitation, nil
}

func invite(ctx context.Context, tx TxStore, input NewInvitation) (Invitation, *notificationsmodel.Notification, error) {
	if _, err := tx.GetUserById(ctx, input.UserID); err != nil {
		return Invitation{}, nil, pkg.OnNotFound(err, pkg.ErrNotFound, "user not found")
	}
	id, err := tx.CreateInvitation(ctx, input)
	if err != nil {
		return Invitation{}, nil, err
	}
	inv, err := tx.GetInvitation(ctx, id)
	if err != nil {
		return Invitation{}, nil, err
	}
	link := "/notifications"
	n := &notificationsmodel.CreateNotificationInput{
		UserID: inv.UserID,
		Link:   &link,
		Data: map[string]any{
			"invitation_id":      inv.ID.String(),
			"inviter_username":   inv.InviterUsername,
			"organization_login": inv.Target.OrganizationLogin,
			"organization_name":  inv.Target.OrganizationName,
			"role":               inv.Role,
		},
	}
	if inv.Target.Kind == KindContest {
		n.Type = notificationsmodel.NotificationTypeContestInvitation
		n.Title = fmt.Sprintf("Приглашение на контест %s", inv.Target.ContestTitle)
		n.Body = fmt.Sprintf("Пользователь @%s пригласил вас участвовать в контесте %s", inv.InviterUsername, inv.Target.ContestTitle)
		n.Data["contest_login"] = inv.Target.ContestLogin
		n.Data["contest_title"] = inv.Target.ContestTitle
	} else {
		n.Type = notificationsmodel.NotificationTypeOrgInvitation
		n.Title = fmt.Sprintf("Приглашение в организацию %s", inv.Target.OrganizationName)
		n.Body = fmt.Sprintf("Пользователь @%s пригласил вас вступить в организацию %s (роль: %s)", inv.InviterUsername, inv.Target.OrganizationName, inv.Role)
	}
	notification, err := tx.CreateNotification(ctx, n)
	if err != nil {
		return Invitation{}, nil, err
	}
	return inv, notification, nil
}

func (uc *UseCase) ListOrganizationInvitations(ctx context.Context, orgLogin string) ([]Invitation, error) {
	orgID, err := uc.repo.OrganizationID(ctx, orgLogin)
	if err != nil {
		return nil, err
	}
	if _, err := access.Organization(ctx, uc.access, orgID, accessmodel.OrganizationInvite); err != nil {
		return nil, err
	}
	return uc.repo.ListInvitations(ctx, Target{Kind: KindOrganization, ID: orgID})
}

func (uc *UseCase) ListContestInvitations(ctx context.Context, contestID uuid.UUID) ([]Invitation, error) {
	if _, err := access.Contest(ctx, uc.access, contestID, accessmodel.ContestManage); err != nil {
		return nil, err
	}
	return uc.repo.ListInvitations(ctx, Target{Kind: KindContest, ID: contestID})
}

// CancelInvitation withdraws a named invitation; organization maintainers withdraw only roles they could grant.
func (uc *UseCase) CancelInvitation(ctx context.Context, id uuid.UUID) error {
	inv, err := uc.repo.GetInvitation(ctx, id)
	if err != nil {
		return err
	}
	cancel := func(tx TxStore) error { return tx.DeleteInvitation(ctx, id) }
	switch inv.Target.Kind {
	case KindOrganization:
		return access.WithinOrganization(ctx, uc.tx, inv.Target.ID, accessmodel.OrganizationInvite, func(tx TxStore, a access.OrganizationAccess) error {
			if err := a.Invite(inv.UserID, accessmodel.OrganizationRole(inv.Role)); err != nil {
				return err
			}
			return cancel(tx)
		})
	case KindContest:
		return access.WithinContest(ctx, uc.tx, inv.Target.ID, accessmodel.ContestManage, func(tx TxStore, _ access.ContestAccess) error { return cancel(tx) })
	}
	return pkg.ErrNotFound
}

func (uc *UseCase) AcceptInvitation(ctx context.Context, id uuid.UUID) error {
	return uc.respond(ctx, id, func(tx TxStore, inv Invitation) error {
		switch inv.Target.Kind {
		case KindOrganization:
			return tx.JoinOrganization(ctx, inv.Target.ID, inv.UserID, accessmodel.OrganizationRole(inv.Role))
		case KindContest:
			return tx.JoinContest(ctx, inv.Target.ID, inv.UserID)
		}
		return pkg.ErrNotFound
	})
}

func (uc *UseCase) DeclineInvitation(ctx context.Context, id uuid.UUID) error {
	return uc.respond(ctx, id, func(TxStore, Invitation) error { return nil })
}

// respond lets only the invitee use a named invitation, once.
func (uc *UseCase) respond(ctx context.Context, id uuid.UUID, apply func(TxStore, Invitation) error) error {
	return access.WithinActor(ctx, uc.tx, func(tx TxStore, user accessmodel.User) error {
		inv, err := tx.GetInvitation(ctx, id)
		if err != nil {
			return pkg.OnNotFound(err, pkg.ErrRequestResolved, "the invitation is no longer pending")
		}
		if inv.UserID != user.Id {
			return pkg.ErrNotFound
		}
		if err := tx.DeleteInvitation(ctx, id); err != nil {
			return err
		}
		return apply(tx, inv)
	})
}

// JoinLink returns the target's link token, or nil while the link is off.
func (uc *UseCase) JoinLink(ctx context.Context, target Target) (*string, error) {
	if err := uc.manageLink(ctx, target, func(TxStore, uuid.UUID) error { return nil }); err != nil {
		return nil, err
	}
	return uc.repo.JoinLink(ctx, target)
}

// ResetJoinLink turns the link on with a fresh token, so the previous link stops working.
func (uc *UseCase) ResetJoinLink(ctx context.Context, target Target) (string, error) {
	token, err := newToken()
	if err != nil {
		return "", err
	}
	err = uc.manageLink(ctx, target, func(tx TxStore, actor uuid.UUID) error { return tx.SetJoinLink(ctx, target, token, actor) })
	return token, err
}

func (uc *UseCase) DisableJoinLink(ctx context.Context, target Target) error {
	return uc.manageLink(ctx, target, func(tx TxStore, _ uuid.UUID) error { return tx.DeleteJoinLink(ctx, target) })
}

func (uc *UseCase) manageLink(ctx context.Context, target Target, fn func(TxStore, uuid.UUID) error) error {
	switch target.Kind {
	case KindTeam:
		return access.WithinTeam(ctx, uc.tx, target.ID, accessmodel.TeamManage, func(tx TxStore, a access.TeamAccess) error { return fn(tx, a.Actor.Id) })
	case KindContest:
		return access.WithinContest(ctx, uc.tx, target.ID, accessmodel.ContestManage, func(tx TxStore, a access.ContestAccess) error { return fn(tx, a.Actor.Id) })
	}
	return pkg.ErrNotFound
}

// LinkTarget shows a logged-in user where a link leads before they follow it.
func (uc *UseCase) LinkTarget(ctx context.Context, token string) (Target, error) {
	if _, err := accessmodel.RequireUser(ctx); err != nil {
		return Target{}, err
	}
	return uc.repo.JoinLinkTarget(ctx, token)
}

// FollowJoinLink makes the caller a team member (joining the organization too, whatever its join policy)
// or a contest participant; following it again changes nothing.
func (uc *UseCase) FollowJoinLink(ctx context.Context, token string) (Target, error) {
	var target Target
	err := access.WithinActor(ctx, uc.tx, func(tx TxStore, user accessmodel.User) error {
		var err error
		target, err = tx.JoinLinkTarget(ctx, token)
		if err != nil {
			return err
		}
		switch target.Kind {
		case KindTeam:
			if err := tx.JoinOrganization(ctx, target.OrganizationID, user.Id, accessmodel.OrganizationMember); err != nil {
				return err
			}
			return tx.JoinTeam(ctx, target.ID, user.Id)
		case KindContest:
			role, err := tx.ContestRole(ctx, target.ID, user.Id)
			if err != nil || role != "" {
				return err
			}
			return tx.JoinContest(ctx, target.ID, user.Id)
		}
		return errors.New("join link without a team or contest")
	})
	return target, err
}

func newToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
