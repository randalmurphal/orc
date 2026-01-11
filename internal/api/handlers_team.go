package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/randalmurphal/orc/internal/db"
)

// createTeamMemberRequest is the request body for creating a team member.
type createTeamMemberRequest struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Initials    string `json:"initials"`
	Role        string `json:"role,omitempty"`
}

// updateTeamMemberRequest is the request body for updating a team member.
type updateTeamMemberRequest struct {
	Email       string `json:"email,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	Initials    string `json:"initials,omitempty"`
	Role        string `json:"role,omitempty"`
}

// claimTaskRequest is the request body for claiming a task.
type claimTaskRequest struct {
	MemberID string `json:"member_id"`
}

// releaseTaskRequest is the request body for releasing a task.
type releaseTaskRequest struct {
	MemberID string `json:"member_id"`
}

// activityListResponse is the response for listing activity.
type activityListResponse struct {
	Activities []db.ActivityLog `json:"activities"`
	Total      int              `json:"total"`
}

// handleListTeamMembers returns all team members.
func (s *Server) handleListTeamMembers(w http.ResponseWriter, r *http.Request) {
	pdb, err := db.OpenProject(".")
	if err != nil {
		s.jsonError(w, "failed to open database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer pdb.Close()

	members, err := pdb.ListTeamMembers()
	if err != nil {
		s.jsonError(w, "failed to list team members: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Ensure we return empty array, not null
	if members == nil {
		members = []db.TeamMember{}
	}

	s.jsonResponse(w, members)
}

// handleCreateTeamMember creates a new team member.
func (s *Server) handleCreateTeamMember(w http.ResponseWriter, r *http.Request) {
	var req createTeamMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Email == "" {
		s.jsonError(w, "email is required", http.StatusBadRequest)
		return
	}
	if req.DisplayName == "" {
		s.jsonError(w, "display_name is required", http.StatusBadRequest)
		return
	}
	if req.Initials == "" {
		s.jsonError(w, "initials is required", http.StatusBadRequest)
		return
	}

	// Validate role
	role := db.TeamMemberRole(req.Role)
	if role == "" {
		role = db.RoleMember
	} else if role != db.RoleAdmin && role != db.RoleMember && role != db.RoleViewer {
		s.jsonError(w, "invalid role: must be admin, member, or viewer", http.StatusBadRequest)
		return
	}

	pdb, err := db.OpenProject(".")
	if err != nil {
		s.jsonError(w, "failed to open database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer pdb.Close()

	// Check if member with email already exists
	existing, err := pdb.GetTeamMemberByEmail(req.Email)
	if err != nil {
		s.jsonError(w, "failed to check existing member: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if existing != nil {
		s.jsonError(w, "member with this email already exists", http.StatusConflict)
		return
	}

	member := &db.TeamMember{
		Email:       req.Email,
		DisplayName: req.DisplayName,
		Initials:    req.Initials,
		Role:        role,
	}

	if err := pdb.CreateTeamMember(member); err != nil {
		s.jsonError(w, "failed to create team member: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	s.jsonResponse(w, member)
}

// handleGetTeamMember retrieves a single team member.
func (s *Server) handleGetTeamMember(w http.ResponseWriter, r *http.Request) {
	memberID := r.PathValue("id")
	if memberID == "" {
		s.jsonError(w, "member_id required", http.StatusBadRequest)
		return
	}

	pdb, err := db.OpenProject(".")
	if err != nil {
		s.jsonError(w, "failed to open database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer pdb.Close()

	member, err := pdb.GetTeamMember(memberID)
	if err != nil {
		s.jsonError(w, "failed to get team member: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if member == nil {
		s.jsonError(w, "member not found", http.StatusNotFound)
		return
	}

	s.jsonResponse(w, member)
}

// handleUpdateTeamMember updates a team member.
func (s *Server) handleUpdateTeamMember(w http.ResponseWriter, r *http.Request) {
	memberID := r.PathValue("id")
	if memberID == "" {
		s.jsonError(w, "member_id required", http.StatusBadRequest)
		return
	}

	pdb, err := db.OpenProject(".")
	if err != nil {
		s.jsonError(w, "failed to open database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer pdb.Close()

	member, err := pdb.GetTeamMember(memberID)
	if err != nil {
		s.jsonError(w, "failed to get team member: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if member == nil {
		s.jsonError(w, "member not found", http.StatusNotFound)
		return
	}

	var req updateTeamMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Email != "" {
		// Check if email is unique
		existing, err := pdb.GetTeamMemberByEmail(req.Email)
		if err != nil {
			s.jsonError(w, "failed to check email uniqueness: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if existing != nil && existing.ID != member.ID {
			s.jsonError(w, "email already in use by another member", http.StatusConflict)
			return
		}
		member.Email = req.Email
	}
	if req.DisplayName != "" {
		member.DisplayName = req.DisplayName
	}
	if req.Initials != "" {
		member.Initials = req.Initials
	}
	if req.Role != "" {
		role := db.TeamMemberRole(req.Role)
		if role != db.RoleAdmin && role != db.RoleMember && role != db.RoleViewer {
			s.jsonError(w, "invalid role: must be admin, member, or viewer", http.StatusBadRequest)
			return
		}
		member.Role = role
	}

	if err := pdb.UpdateTeamMember(member); err != nil {
		s.jsonError(w, "failed to update team member: "+err.Error(), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, member)
}

// handleDeleteTeamMember removes a team member.
func (s *Server) handleDeleteTeamMember(w http.ResponseWriter, r *http.Request) {
	memberID := r.PathValue("id")
	if memberID == "" {
		s.jsonError(w, "member_id required", http.StatusBadRequest)
		return
	}

	pdb, err := db.OpenProject(".")
	if err != nil {
		s.jsonError(w, "failed to open database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer pdb.Close()

	// Check if member exists
	member, err := pdb.GetTeamMember(memberID)
	if err != nil {
		s.jsonError(w, "failed to get team member: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if member == nil {
		s.jsonError(w, "member not found", http.StatusNotFound)
		return
	}

	if err := pdb.DeleteTeamMember(memberID); err != nil {
		s.jsonError(w, "failed to delete team member: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleClaimTask claims a task for a team member.
func (s *Server) handleClaimTask(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	if taskID == "" {
		s.jsonError(w, "task_id required", http.StatusBadRequest)
		return
	}

	var req claimTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.MemberID == "" {
		s.jsonError(w, "member_id is required", http.StatusBadRequest)
		return
	}

	pdb, err := db.OpenProject(".")
	if err != nil {
		s.jsonError(w, "failed to open database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer pdb.Close()

	// Verify task exists
	task, err := pdb.GetTask(taskID)
	if err != nil {
		s.jsonError(w, "failed to get task: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if task == nil {
		s.jsonError(w, "task not found", http.StatusNotFound)
		return
	}

	// Verify member exists
	member, err := pdb.GetTeamMember(req.MemberID)
	if err != nil {
		s.jsonError(w, "failed to get team member: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if member == nil {
		s.jsonError(w, "member not found", http.StatusNotFound)
		return
	}

	if err := pdb.ClaimTask(taskID, req.MemberID); err != nil {
		s.jsonError(w, "failed to claim task: "+err.Error(), http.StatusConflict)
		return
	}

	// Return the claim info
	claim, err := pdb.GetActiveTaskClaim(taskID)
	if err != nil {
		s.jsonError(w, "failed to get claim: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]any{
		"task_id":    taskID,
		"member_id":  req.MemberID,
		"claimed_at": claim.ClaimedAt,
		"member":     member,
	}

	w.WriteHeader(http.StatusCreated)
	s.jsonResponse(w, response)
}

// handleReleaseTask releases a task claim.
func (s *Server) handleReleaseTask(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	if taskID == "" {
		s.jsonError(w, "task_id required", http.StatusBadRequest)
		return
	}

	var req releaseTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.MemberID == "" {
		s.jsonError(w, "member_id is required", http.StatusBadRequest)
		return
	}

	pdb, err := db.OpenProject(".")
	if err != nil {
		s.jsonError(w, "failed to open database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer pdb.Close()

	// Verify the claim exists and belongs to this member
	claim, err := pdb.GetActiveTaskClaim(taskID)
	if err != nil {
		s.jsonError(w, "failed to get claim: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if claim == nil {
		s.jsonError(w, "task is not claimed", http.StatusBadRequest)
		return
	}
	if claim.MemberID != req.MemberID {
		s.jsonError(w, "task is claimed by another member", http.StatusForbidden)
		return
	}

	if err := pdb.ReleaseTask(taskID, req.MemberID); err != nil {
		s.jsonError(w, "failed to release task: "+err.Error(), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, map[string]any{
		"task_id":     taskID,
		"member_id":   req.MemberID,
		"released_at": time.Now(),
	})
}

// handleGetTaskClaim returns the current claim status for a task.
func (s *Server) handleGetTaskClaim(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	if taskID == "" {
		s.jsonError(w, "task_id required", http.StatusBadRequest)
		return
	}

	pdb, err := db.OpenProject(".")
	if err != nil {
		s.jsonError(w, "failed to open database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer pdb.Close()

	claim, err := pdb.GetActiveTaskClaim(taskID)
	if err != nil {
		s.jsonError(w, "failed to get claim: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if claim == nil {
		s.jsonResponse(w, map[string]any{
			"task_id":    taskID,
			"is_claimed": false,
		})
		return
	}

	// Get member info
	member, err := pdb.GetTeamMember(claim.MemberID)
	if err != nil {
		s.jsonError(w, "failed to get member: "+err.Error(), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, map[string]any{
		"task_id":    taskID,
		"is_claimed": true,
		"member_id":  claim.MemberID,
		"claimed_at": claim.ClaimedAt,
		"member":     member,
	})
}

// handleListActivity returns the activity feed.
func (s *Server) handleListActivity(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	opts := db.ListActivityOpts{
		TaskID:   query.Get("task_id"),
		MemberID: query.Get("member_id"),
		Limit:    50, // Default limit
	}

	if action := query.Get("action"); action != "" {
		opts.Action = db.ActivityAction(action)
	}

	if sinceStr := query.Get("since"); sinceStr != "" {
		if t, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			opts.Since = &t
		}
	}

	if limitStr := query.Get("limit"); limitStr != "" {
		if limit := parseIntOrDefault(limitStr, 50); limit > 0 && limit <= 200 {
			opts.Limit = limit
		}
	}

	if offsetStr := query.Get("offset"); offsetStr != "" {
		opts.Offset = parseIntOrDefault(offsetStr, 0)
	}

	pdb, err := db.OpenProject(".")
	if err != nil {
		s.jsonError(w, "failed to open database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer pdb.Close()

	activities, err := pdb.ListActivity(opts)
	if err != nil {
		s.jsonError(w, "failed to list activity: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Ensure we return empty array, not null
	if activities == nil {
		activities = []db.ActivityLog{}
	}

	s.jsonResponse(w, activityListResponse{
		Activities: activities,
		Total:      len(activities),
	})
}

// handleGetMemberClaims returns all active claims for a team member.
func (s *Server) handleGetMemberClaims(w http.ResponseWriter, r *http.Request) {
	memberID := r.PathValue("id")
	if memberID == "" {
		s.jsonError(w, "member_id required", http.StatusBadRequest)
		return
	}

	pdb, err := db.OpenProject(".")
	if err != nil {
		s.jsonError(w, "failed to open database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer pdb.Close()

	claims, err := pdb.GetMemberClaims(memberID)
	if err != nil {
		s.jsonError(w, "failed to get member claims: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Ensure we return empty array, not null
	if claims == nil {
		claims = []db.TaskClaim{}
	}

	s.jsonResponse(w, claims)
}

// parseIntOrDefault parses a string to int, returning default on error.
func parseIntOrDefault(s string, defaultVal int) int {
	var result int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + int(c-'0')
		} else {
			return defaultVal
		}
	}
	if len(s) == 0 {
		return defaultVal
	}
	return result
}
