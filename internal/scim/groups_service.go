package scim

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	resourceGroup     = "scim_group"
	actionGroupCreate = "scim.group.created"
	actionGroupUpdate = "scim.group.updated"
	actionGroupDelete = "scim.group.deleted"
)

// CreateGroup provisions a new group into the tenant. Member ids that do not
// resolve to a SCIM user in this organization are dropped (this both prevents
// cross-tenant member references and tolerates the IdP pushing a group before
// all of its users have been synced).
func (s *Service) CreateGroup(ctx context.Context, organizationID string, in GroupInput) (GroupRecord, error) {
	displayName := strings.TrimSpace(in.DisplayName)
	if displayName == "" {
		return GroupRecord{}, ErrInvalid
	}

	if _, err := s.groups.GetByDisplayName(ctx, organizationID, displayName); err == nil {
		return GroupRecord{}, ErrConflict
	} else if !errors.Is(err, ErrNotFound) {
		return GroupRecord{}, fmt.Errorf("check existing scim group: %w", err)
	}

	members, err := s.validateMembers(ctx, organizationID, in.MemberIDs)
	if err != nil {
		return GroupRecord{}, err
	}

	now := time.Now().UTC()
	record := GroupRecord{
		ID:             uuid.NewString(),
		OrganizationID: organizationID,
		ExternalID:     strings.TrimSpace(in.ExternalID),
		DisplayName:    displayName,
		Members:        members,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.groups.Create(ctx, record); err != nil {
		return GroupRecord{}, fmt.Errorf("store scim group: %w", err)
	}

	if err := s.recordAudit(ctx, organizationID, scimActor, actionGroupCreate, resourceGroup, record.ID, map[string]any{
		"displayName": record.DisplayName,
		"members":     len(record.Members),
	}); err != nil {
		return GroupRecord{}, err
	}

	return record, nil
}

// GetGroup returns a group by resource id within the tenant.
func (s *Service) GetGroup(ctx context.Context, organizationID, id string) (GroupRecord, error) {
	record, err := s.groups.GetByID(ctx, organizationID, id)
	if err != nil {
		return GroupRecord{}, wrapGroupLookup(err)
	}

	return record, nil
}

// ListGroups returns a page of tenant groups (optionally filtered by an exact
// displayName) plus the total number of matching groups.
func (s *Service) ListGroups(
	ctx context.Context,
	organizationID, displayNameFilter string,
	offset, limit int,
) ([]GroupRecord, int, error) {
	records, total, err := s.groups.List(ctx, organizationID, strings.TrimSpace(displayNameFilter), offset, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("list scim groups: %w", err)
	}

	return records, total, nil
}

// ReplaceGroup applies a full-resource update (SCIM PUT).
func (s *Service) ReplaceGroup(ctx context.Context, organizationID, id string, in GroupInput) (GroupRecord, error) {
	record, err := s.groups.GetByID(ctx, organizationID, id)
	if err != nil {
		return GroupRecord{}, wrapGroupLookup(err)
	}

	displayName := strings.TrimSpace(in.DisplayName)
	if displayName == "" {
		return GroupRecord{}, ErrInvalid
	}

	members, err := s.validateMembers(ctx, organizationID, in.MemberIDs)
	if err != nil {
		return GroupRecord{}, err
	}

	record.DisplayName = displayName
	record.ExternalID = strings.TrimSpace(in.ExternalID)
	record.Members = members

	return s.saveGroup(ctx, record)
}

// PatchGroup applies normalized SCIM PATCH operations (rename and/or membership
// add/remove/replace) to a group.
func (s *Service) PatchGroup(
	ctx context.Context,
	organizationID, id string,
	changes []GroupPatchChange,
) (GroupRecord, error) {
	record, err := s.groups.GetByID(ctx, organizationID, id)
	if err != nil {
		return GroupRecord{}, wrapGroupLookup(err)
	}

	members := record.Members

	for _, change := range changes {
		if change.DisplayName != nil {
			name := strings.TrimSpace(*change.DisplayName)
			if name == "" {
				return GroupRecord{}, ErrInvalid
			}

			record.DisplayName = name
		}

		members = applyMemberChange(members, change)
	}

	valid, err := s.validateMembers(ctx, organizationID, members)
	if err != nil {
		return GroupRecord{}, err
	}

	record.Members = valid

	return s.saveGroup(ctx, record)
}

// DeleteGroup removes a group resource from the tenant.
func (s *Service) DeleteGroup(ctx context.Context, organizationID, id string) error {
	if _, err := s.groups.GetByID(ctx, organizationID, id); err != nil {
		return wrapGroupLookup(err)
	}

	if err := s.groups.Delete(ctx, organizationID, id); err != nil {
		return fmt.Errorf("delete scim group: %w", err)
	}

	return s.recordAudit(ctx, organizationID, scimActor, actionGroupDelete, resourceGroup, id, nil)
}

func applyMemberChange(current []string, change GroupPatchChange) []string {
	switch change.MemberOp {
	case GroupMembersReplace:
		return append([]string{}, change.MemberIDs...)
	case GroupMembersAdd:
		return append(current, change.MemberIDs...)
	case GroupMembersRemove:
		// Remove only the listed ids; an empty list is a no-op.
		return removeAll(current, change.MemberIDs)
	case GroupMembersClear:
		return nil
	case GroupMembersUnchanged:
		return current
	default:
		return current
	}
}

// validateMembers de-duplicates member ids and keeps only those that resolve to
// a SCIM user in this organization, dropping the rest.
func (s *Service) validateMembers(ctx context.Context, organizationID string, ids []string) ([]string, error) {
	seen := make(map[string]bool, len(ids))
	valid := make([]string, 0, len(ids))

	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if id == "" || seen[id] {
			continue
		}

		seen[id] = true

		if _, err := s.store.GetByID(ctx, organizationID, id); err != nil {
			if errors.Is(err, ErrNotFound) {
				continue
			}

			return nil, fmt.Errorf("validate group member: %w", err)
		}

		valid = append(valid, id)
	}

	return valid, nil
}

func (s *Service) saveGroup(ctx context.Context, record GroupRecord) (GroupRecord, error) {
	record.UpdatedAt = time.Now().UTC()

	if err := s.groups.Update(ctx, record); err != nil {
		return GroupRecord{}, fmt.Errorf("update scim group: %w", err)
	}

	orgID := record.OrganizationID

	meta := map[string]any{"displayName": record.DisplayName, "members": len(record.Members)}
	if err := s.recordAudit(ctx, orgID, scimActor, actionGroupUpdate, resourceGroup, record.ID, meta); err != nil {
		return GroupRecord{}, err
	}

	return record, nil
}

func removeAll(list, toRemove []string) []string {
	drop := make(map[string]bool, len(toRemove))
	for _, id := range toRemove {
		drop[id] = true
	}

	out := make([]string, 0, len(list))
	for _, id := range list {
		if !drop[id] {
			out = append(out, id)
		}
	}

	return out
}

func wrapGroupLookup(err error) error {
	if errors.Is(err, ErrNotFound) {
		return ErrNotFound
	}

	return fmt.Errorf("load scim group: %w", err)
}
