package core

import (
	"context"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/internal/invitations"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

func invitationDTO(inv invitations.Invitation) corev1.InvitationModel {
	return corev1.InvitationModel{
		ID:                inv.ID,
		Kind:              corev1.InvitationKind(inv.Target.Kind),
		OrganizationLogin: inv.Target.OrganizationLogin,
		OrganizationName:  inv.Target.OrganizationName,
		ContestLogin:      stringToOptString(inv.Target.ContestLogin),
		ContestTitle:      stringToOptString(inv.Target.ContestTitle),
		UserID:            inv.UserID,
		Username:          inv.Username,
		InviterUsername:   inv.InviterUsername,
		Role:              inv.Role,
		CreatedAt:         inv.CreatedAt,
	}
}

func invitationsDTO(items []invitations.Invitation) *corev1.ListInvitationsResponseModel {
	out := make([]corev1.InvitationModel, len(items))
	for i, inv := range items {
		out[i] = invitationDTO(inv)
	}
	return &corev1.ListInvitationsResponseModel{Invitations: out}
}

func joinLinkTargetDTO(t invitations.Target) *corev1.JoinLinkTargetModel {
	return &corev1.JoinLinkTargetModel{
		Kind:              corev1.InvitationKind(t.Kind),
		ID:                t.ID,
		OrganizationLogin: t.OrganizationLogin,
		OrganizationName:  t.OrganizationName,
		TeamName:          stringToOptString(t.TeamName),
		ContestLogin:      stringToOptString(t.ContestLogin),
		ContestTitle:      stringToOptString(t.ContestTitle),
	}
}

func (h *CoreServer) ListOrganizationInvitations(ctx context.Context, params corev1.ListOrganizationInvitationsParams) (*corev1.ListInvitationsResponseModel, error) {
	items, err := h.Invitations.ListOrganizationInvitations(ctx, params.Login)
	if err != nil {
		return nil, err
	}
	return invitationsDTO(items), nil
}

func (h *CoreServer) InviteOrganizationMember(ctx context.Context, req *corev1.InviteOrganizationMemberRequestModel, params corev1.InviteOrganizationMemberParams) (*corev1.InvitationModel, error) {
	role := accessmodel.OrganizationRole(req.Role)
	if !validateOrganizationRole(string(role)) {
		return nil, pkg.Wrap(pkg.ErrInvalid, nil, "invalid role")
	}
	inv, err := h.Invitations.InviteToOrganization(ctx, params.Login, req.UserID, role)
	if err != nil {
		return nil, err
	}
	res := invitationDTO(*inv)
	return &res, nil
}

func (h *CoreServer) ListContestInvitations(ctx context.Context, params corev1.ListContestInvitationsParams) (*corev1.ListInvitationsResponseModel, error) {
	items, err := h.Invitations.ListContestInvitations(ctx, params.ContestID)
	if err != nil {
		return nil, err
	}
	return invitationsDTO(items), nil
}

func (h *CoreServer) InviteContestParticipant(ctx context.Context, req *corev1.InviteContestParticipantRequestModel, params corev1.InviteContestParticipantParams) (*corev1.InvitationModel, error) {
	inv, err := h.Invitations.InviteToContest(ctx, params.ContestID, req.UserID)
	if err != nil {
		return nil, err
	}
	res := invitationDTO(*inv)
	return &res, nil
}

func (h *CoreServer) CancelInvitation(ctx context.Context, params corev1.CancelInvitationParams) error {
	return h.Invitations.CancelInvitation(ctx, params.ID)
}

func (h *CoreServer) RespondInvitation(ctx context.Context, req *corev1.RespondInvitationRequestModel, params corev1.RespondInvitationParams) error {
	if req.Status == corev1.RespondInvitationRequestModelStatusAccepted {
		return h.Invitations.AcceptInvitation(ctx, params.ID)
	}
	return h.Invitations.DeclineInvitation(ctx, params.ID)
}

func joinLinkDTO(token *string) *corev1.JoinLinkModel {
	if token == nil {
		return &corev1.JoinLinkModel{}
	}
	return &corev1.JoinLinkModel{Token: corev1.NewOptString(*token)}
}

func teamTarget(id uuid.UUID) invitations.Target {
	return invitations.Target{Kind: invitations.KindTeam, ID: id}
}

func (h *CoreServer) GetTeamJoinLink(ctx context.Context, params corev1.GetTeamJoinLinkParams) (*corev1.JoinLinkModel, error) {
	token, err := h.Invitations.JoinLink(ctx, teamTarget(params.ID))
	if err != nil {
		return nil, err
	}
	return joinLinkDTO(token), nil
}

func (h *CoreServer) ResetTeamJoinLink(ctx context.Context, params corev1.ResetTeamJoinLinkParams) (*corev1.JoinLinkModel, error) {
	token, err := h.Invitations.ResetJoinLink(ctx, teamTarget(params.ID))
	if err != nil {
		return nil, err
	}
	return joinLinkDTO(&token), nil
}

func (h *CoreServer) DisableTeamJoinLink(ctx context.Context, params corev1.DisableTeamJoinLinkParams) error {
	return h.Invitations.DisableJoinLink(ctx, teamTarget(params.ID))
}

func (h *CoreServer) GetContestJoinLink(ctx context.Context, params corev1.GetContestJoinLinkParams) (*corev1.JoinLinkModel, error) {
	target := invitations.Target{Kind: invitations.KindContest, ID: params.ContestID}
	token, err := h.Invitations.JoinLink(ctx, target)
	if err != nil {
		return nil, err
	}
	return joinLinkDTO(token), nil
}

func (h *CoreServer) ResetContestJoinLink(ctx context.Context, params corev1.ResetContestJoinLinkParams) (*corev1.JoinLinkModel, error) {
	target := invitations.Target{Kind: invitations.KindContest, ID: params.ContestID}
	token, err := h.Invitations.ResetJoinLink(ctx, target)
	if err != nil {
		return nil, err
	}
	return joinLinkDTO(&token), nil
}

func (h *CoreServer) DisableContestJoinLink(ctx context.Context, params corev1.DisableContestJoinLinkParams) error {
	return h.Invitations.DisableJoinLink(ctx, invitations.Target{Kind: invitations.KindContest, ID: params.ContestID})
}

func (h *CoreServer) GetJoinLinkTarget(ctx context.Context, params corev1.GetJoinLinkTargetParams) (*corev1.JoinLinkTargetModel, error) {
	target, err := h.Invitations.LinkTarget(ctx, params.Token)
	if err != nil {
		return nil, err
	}
	return joinLinkTargetDTO(target), nil
}

func (h *CoreServer) FollowJoinLink(ctx context.Context, params corev1.FollowJoinLinkParams) (*corev1.JoinLinkTargetModel, error) {
	target, err := h.Invitations.FollowJoinLink(ctx, params.Token)
	if err != nil {
		return nil, err
	}
	return joinLinkTargetDTO(target), nil
}
