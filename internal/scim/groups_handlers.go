package scim

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// SCIM PATCH operation names.
const (
	opAdd     = "add"
	opRemove  = "remove"
	opReplace = "replace"
)

// groupMemberPayload is a lenient decode target for a SCIM group member.
type groupMemberPayload struct {
	Value string `json:"value"`
}

// groupPayload is a lenient decode target for SCIM Group bodies (unknown IdP
// attributes are ignored).
type groupPayload struct {
	Schemas     []string             `json:"schemas"`
	ExternalID  string               `json:"externalId"`
	DisplayName string               `json:"displayName"`
	Members     []groupMemberPayload `json:"members"`
}

// HandleListGroups lists tenant groups, optionally filtered by displayName, with
// SCIM startIndex/count pagination.
//
//nolint:dupl // parallel to HandleListUsers; distinct resource + envelope types.
func (h *Handler) HandleListGroups(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := organizationFromContext(r.Context())
	if !ok {
		h.writeError(w, r, http.StatusUnauthorized, "Authentication required")

		return
	}

	offset, limit := parsePage(r.URL.Query().Get("startIndex"), r.URL.Query().Get("count"))

	records, total, err := h.svc.ListGroups(
		r.Context(), organizationID, parseDisplayNameFilter(r.URL.Query().Get("filter")), offset, limit,
	)
	if err != nil {
		h.mapError(w, r, err)

		return
	}

	resources := make([]Group, len(records))
	for i, record := range records {
		resources[i] = toGroup(record)
	}

	h.writeSCIM(w, r, http.StatusOK, GroupListResponse{
		Schemas:      []string{SchemaListResponse},
		TotalResults: total,
		StartIndex:   offset + 1,
		ItemsPerPage: len(resources),
		Resources:    resources,
	})
}

// HandleCreateGroup provisions a new group.
func (h *Handler) HandleCreateGroup(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := organizationFromContext(r.Context())
	if !ok {
		h.writeError(w, r, http.StatusUnauthorized, "Authentication required")

		return
	}

	payload, err := decodeGroup(r)
	if err != nil {
		h.writeError(w, r, http.StatusBadRequest, "Request body is invalid")

		return
	}

	record, err := h.svc.CreateGroup(r.Context(), organizationID, toGroupInput(payload))
	if err != nil {
		h.mapError(w, r, err)

		return
	}

	h.writeSCIM(w, r, http.StatusCreated, toGroup(record))
}

// HandleGetGroup returns a single group resource.
func (h *Handler) HandleGetGroup(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := organizationFromContext(r.Context())
	if !ok {
		h.writeError(w, r, http.StatusUnauthorized, "Authentication required")

		return
	}

	record, err := h.svc.GetGroup(r.Context(), organizationID, chi.URLParam(r, "groupID"))
	if err != nil {
		h.mapError(w, r, err)

		return
	}

	h.writeSCIM(w, r, http.StatusOK, toGroup(record))
}

// HandleReplaceGroup applies a full-resource update.
func (h *Handler) HandleReplaceGroup(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := organizationFromContext(r.Context())
	if !ok {
		h.writeError(w, r, http.StatusUnauthorized, "Authentication required")

		return
	}

	payload, err := decodeGroup(r)
	if err != nil {
		h.writeError(w, r, http.StatusBadRequest, "Request body is invalid")

		return
	}

	record, err := h.svc.ReplaceGroup(r.Context(), organizationID, chi.URLParam(r, "groupID"), toGroupInput(payload))
	if err != nil {
		h.mapError(w, r, err)

		return
	}

	h.writeSCIM(w, r, http.StatusOK, toGroup(record))
}

// HandlePatchGroup applies membership and rename changes.
func (h *Handler) HandlePatchGroup(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := organizationFromContext(r.Context())
	if !ok {
		h.writeError(w, r, http.StatusUnauthorized, "Authentication required")

		return
	}

	var req PatchRequest
	if err := decodeBody(r, &req); err != nil {
		h.writeError(w, r, http.StatusBadRequest, "Request body is invalid")

		return
	}

	record, err := h.svc.PatchGroup(r.Context(), organizationID, chi.URLParam(r, "groupID"), parseGroupPatch(req))
	if err != nil {
		h.mapError(w, r, err)

		return
	}

	h.writeSCIM(w, r, http.StatusOK, toGroup(record))
}

// HandleDeleteGroup removes a group resource.
func (h *Handler) HandleDeleteGroup(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := organizationFromContext(r.Context())
	if !ok {
		h.writeError(w, r, http.StatusUnauthorized, "Authentication required")

		return
	}

	if err := h.svc.DeleteGroup(r.Context(), organizationID, chi.URLParam(r, "groupID")); err != nil {
		h.mapError(w, r, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func toGroup(record GroupRecord) Group {
	members := make([]GroupMember, 0, len(record.Members))
	for _, id := range record.Members {
		members = append(members, GroupMember{Value: id, Display: ""})
	}

	return Group{
		Schemas:     []string{SchemaGroup},
		ID:          record.ID,
		ExternalID:  record.ExternalID,
		DisplayName: record.DisplayName,
		Members:     members,
		Meta: &Meta{
			ResourceType: resourceTypeGroup,
			Created:      record.CreatedAt.UTC().Format(time.RFC3339),
			LastModified: record.UpdatedAt.UTC().Format(time.RFC3339),
			Location:     "Groups/" + record.ID,
		},
	}
}

func toGroupInput(payload groupPayload) GroupInput {
	return GroupInput{
		DisplayName: payload.DisplayName,
		ExternalID:  payload.ExternalID,
		MemberIDs:   memberIDsFromPayload(payload.Members),
	}
}

func memberIDsFromPayload(members []groupMemberPayload) []string {
	ids := make([]string, 0, len(members))
	for _, member := range members {
		if value := strings.TrimSpace(member.Value); value != "" {
			ids = append(ids, value)
		}
	}

	return ids
}

func decodeGroup(r *http.Request) (groupPayload, error) {
	var payload groupPayload
	if err := decodeBody(r, &payload); err != nil {
		return groupPayload{}, err
	}

	return payload, nil
}

// parseDisplayNameFilter extracts the value from a `displayName eq "x"` filter.
func parseDisplayNameFilter(filter string) string {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return ""
	}

	_, after, found := strings.Cut(filter, "displayName eq ")
	if !found {
		return ""
	}

	return strings.Trim(strings.TrimSpace(after), `"`)
}

func parseGroupPatch(req PatchRequest) []GroupPatchChange {
	changes := make([]GroupPatchChange, 0, len(req.Operations))

	for _, op := range req.Operations {
		if change, ok := parseGroupPatchOp(op); ok {
			changes = append(changes, change)
		}
	}

	return changes
}

func parseGroupPatchOp(op PatchOp) (GroupPatchChange, bool) {
	lowerOp := strings.ToLower(strings.TrimSpace(op.Op))
	path := strings.TrimSpace(op.Path)

	if strings.EqualFold(path, "displayName") {
		var name string
		if json.Unmarshal(op.Value, &name) != nil {
			return GroupPatchChange{}, false
		}

		return GroupPatchChange{DisplayName: &name, MemberOp: GroupMembersUnchanged, MemberIDs: nil}, true
	}

	if id, ok := memberIDFromPath(path); ok {
		if lowerOp != opRemove {
			return GroupPatchChange{}, false
		}

		return GroupPatchChange{DisplayName: nil, MemberOp: GroupMembersRemove, MemberIDs: []string{id}}, true
	}

	if strings.EqualFold(path, "members") {
		// A `remove` on "members" with NO value clears the whole attribute; one
		// WITH a value (even an empty array) removes only those ids.
		if lowerOp == opRemove && len(op.Value) == 0 {
			return GroupPatchChange{DisplayName: nil, MemberOp: GroupMembersClear, MemberIDs: nil}, true
		}

		return memberOpChange(lowerOp, memberValues(op.Value))
	}

	if path == "" && (lowerOp == opReplace || lowerOp == opAdd) {
		return parseGroupPatchBody(op.Value, lowerOp)
	}

	return GroupPatchChange{}, false
}

func memberOpChange(lowerOp string, ids []string) (GroupPatchChange, bool) {
	switch lowerOp {
	case opAdd:
		return GroupPatchChange{DisplayName: nil, MemberOp: GroupMembersAdd, MemberIDs: ids}, true
	case opReplace:
		return GroupPatchChange{DisplayName: nil, MemberOp: GroupMembersReplace, MemberIDs: ids}, true
	case opRemove:
		return GroupPatchChange{DisplayName: nil, MemberOp: GroupMembersRemove, MemberIDs: ids}, true
	default:
		return GroupPatchChange{}, false
	}
}

// parseGroupPatchBody handles a pathless op whose value is an object that may
// carry a displayName and/or a members array.
func parseGroupPatchBody(raw json.RawMessage, lowerOp string) (GroupPatchChange, bool) {
	var body struct {
		DisplayName *string              `json:"displayName"`
		Members     []groupMemberPayload `json:"members"`
	}
	if json.Unmarshal(raw, &body) != nil {
		return GroupPatchChange{}, false
	}

	change := GroupPatchChange{DisplayName: body.DisplayName, MemberOp: GroupMembersUnchanged, MemberIDs: nil}
	changed := body.DisplayName != nil

	if body.Members != nil {
		change.MemberIDs = memberIDsFromPayload(body.Members)
		change.MemberOp = GroupMembersReplace

		if lowerOp == opAdd {
			change.MemberOp = GroupMembersAdd
		}

		changed = true
	}

	return change, changed
}

func memberValues(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}

	var many []groupMemberPayload
	if json.Unmarshal(raw, &many) == nil {
		return memberIDsFromPayload(many)
	}

	var single groupMemberPayload
	if json.Unmarshal(raw, &single) == nil && strings.TrimSpace(single.Value) != "" {
		return []string{strings.TrimSpace(single.Value)}
	}

	return nil
}

// memberIDFromPath extracts X from a `members[value eq "X"]` patch path.
func memberIDFromPath(path string) (string, bool) {
	if !strings.HasPrefix(path, "members[") || !strings.HasSuffix(path, "]") {
		return "", false
	}

	inner := path[len("members[") : len(path)-1]

	_, after, found := strings.Cut(inner, "value eq ")
	if !found {
		return "", false
	}

	return strings.Trim(strings.TrimSpace(after), `"`), true
}
