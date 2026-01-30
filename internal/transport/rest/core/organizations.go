package core

import (
	"context"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/core/pkg"
)

// ListOrganizations handles GET /organizations
func (h *CoreServer) ListOrganizations(ctx context.Context, request corev1.ListOrganizationsRequestObject) (corev1.ListOrganizationsResponseObject, error) {
	return nil, pkg.Wrap(pkg.NotImplemented, nil, "ListOrganizations not implemented yet")
}

// CreateOrganization handles POST /organizations
func (h *CoreServer) CreateOrganization(ctx context.Context, request corev1.CreateOrganizationRequestObject) (corev1.CreateOrganizationResponseObject, error) {
	return nil, pkg.Wrap(pkg.NotImplemented, nil, "CreateOrganization not implemented yet")
}

// DeleteOrganization handles DELETE /organizations/{id}
func (h *CoreServer) DeleteOrganization(ctx context.Context, request corev1.DeleteOrganizationRequestObject) (corev1.DeleteOrganizationResponseObject, error) {
	return nil, pkg.Wrap(pkg.NotImplemented, nil, "DeleteOrganization not implemented yet")
}

// GetOrganization handles GET /organizations/{id}
func (h *CoreServer) GetOrganization(ctx context.Context, request corev1.GetOrganizationRequestObject) (corev1.GetOrganizationResponseObject, error) {
	return nil, pkg.Wrap(pkg.NotImplemented, nil, "GetOrganization not implemented yet")
}

// UpdateOrganization handles PATCH /organizations/{id}
func (h *CoreServer) UpdateOrganization(ctx context.Context, request corev1.UpdateOrganizationRequestObject) (corev1.UpdateOrganizationResponseObject, error) {
	return nil, pkg.Wrap(pkg.NotImplemented, nil, "UpdateOrganization not implemented yet")
}

// RemoveOrganizationMember handles DELETE /organizations/{id}/members
func (h *CoreServer) RemoveOrganizationMember(ctx context.Context, request corev1.RemoveOrganizationMemberRequestObject) (corev1.RemoveOrganizationMemberResponseObject, error) {
	return nil, pkg.Wrap(pkg.NotImplemented, nil, "RemoveOrganizationMember not implemented yet")
}

// ListOrganizationMembers handles GET /organizations/{id}/members
func (h *CoreServer) ListOrganizationMembers(ctx context.Context, request corev1.ListOrganizationMembersRequestObject) (corev1.ListOrganizationMembersResponseObject, error) {
	return nil, pkg.Wrap(pkg.NotImplemented, nil, "ListOrganizationMembers not implemented yet")
}

// AddOrganizationMember handles POST /organizations/{id}/members
func (h *CoreServer) AddOrganizationMember(ctx context.Context, request corev1.AddOrganizationMemberRequestObject) (corev1.AddOrganizationMemberResponseObject, error) {
	return corev1.AddOrganizationMember200Response{}, nil
}
