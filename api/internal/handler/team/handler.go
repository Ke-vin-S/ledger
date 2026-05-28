package team

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	jwtauth "github.com/Ke-vin-S/ledger/api/internal/auth"
	"github.com/Ke-vin-S/ledger/api/internal/domain/team"
	"github.com/Ke-vin-S/ledger/api/internal/handler"
)

type Handler struct {
	teams       *team.Service
	frontendURL string
}

func New(teams *team.Service, frontendURL string) *Handler {
	return &Handler{teams: teams, frontendURL: frontendURL}
}

// Routes mounts all team and invite-link endpoints (no /v1 prefix).
func (h *Handler) Routes(authMW func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()
	r.Use(authMW)

	// Team CRUD
	r.Get("/", h.ListTeams)
	r.Post("/", h.CreateTeam)
	r.Get("/{teamID}", h.GetTeam)
	r.Patch("/{teamID}", h.UpdateTeam)
	r.Delete("/{teamID}", h.DeleteTeam)

	// Membership
	r.Get("/{teamID}/members", h.ListMembers)
	r.Post("/{teamID}/members/invite", h.InviteMember)
	r.Post("/{teamID}/members/anonymous", h.AddAnonymousMember)
	r.Post("/{teamID}/members/request", h.RequestJoin)
	r.Get("/{teamID}/members/requests", h.ListJoinRequests)
	r.Post("/{teamID}/members/requests/{rid}/approve", h.ApproveJoin)
	r.Post("/{teamID}/members/requests/{rid}/reject", h.RejectJoin)
	r.Patch("/{teamID}/members/{uid}/role", h.ChangeRole)
	r.Delete("/{teamID}/members/{uid}", h.RemoveMember)

	// Invite links
	r.Post("/{teamID}/invite-links", h.CreateInviteLink)
	r.Get("/{teamID}/invite-links", h.ListInviteLinks)
	r.Delete("/{teamID}/invite-links/{lid}", h.RevokeInviteLink)

	return r
}

// InviteLinkRoute returns the handler for POST /invite/:token (mounted separately).
func (h *Handler) JoinViaInviteLink(w http.ResponseWriter, r *http.Request) {
	uid := jwtauth.MustUserID(r.Context())
	rawToken := chi.URLParam(r, "token")
	if rawToken == "" {
		handler.Error(w, r, http.StatusBadRequest, "INVALID_REQUEST", "token is required")
		return
	}
	m, err := h.teams.JoinViaInviteLink(r.Context(), rawToken, uid)
	if err != nil {
		h.handleError(w, r, err)
		return
	}
	handler.JSON(w, r, http.StatusOK, toMemberResponse(m))
}

// ── Team CRUD ─────────────────────────────────────────────────────────────────

func (h *Handler) ListTeams(w http.ResponseWriter, r *http.Request) {
	uid := jwtauth.MustUserID(r.Context())
	teams, err := h.teams.ListForUser(r.Context(), uid)
	if err != nil {
		handler.Error(w, r, http.StatusInternalServerError, "SERVER_ERROR", "failed to list teams")
		return
	}
	if teams == nil {
		teams = []*team.Team{}
	}
	handler.JSON(w, r, http.StatusOK, toTeamResponses(teams))
}

func (h *Handler) CreateTeam(w http.ResponseWriter, r *http.Request) {
	uid := jwtauth.MustUserID(r.Context())
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Currency    string `json:"currency"`
		IsPublic    bool   `json:"is_public"`
	}
	if !handler.Decode(w, r, &body) {
		return
	}
	t, err := h.teams.Create(r.Context(), uid, body.Name, body.Description, body.Currency, body.IsPublic)
	if err != nil {
		handler.Error(w, r, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	handler.JSON(w, r, http.StatusCreated, toTeamResponse(t))
}

func (h *Handler) GetTeam(w http.ResponseWriter, r *http.Request) {
	uid := jwtauth.MustUserID(r.Context())
	teamID, ok := parseUUID(w, r, "teamID")
	if !ok {
		return
	}
	t, err := h.teams.GetForMember(r.Context(), teamID, uid)
	if err != nil {
		h.handleError(w, r, err)
		return
	}
	handler.JSON(w, r, http.StatusOK, toTeamResponse(t))
}

func (h *Handler) UpdateTeam(w http.ResponseWriter, r *http.Request) {
	uid := jwtauth.MustUserID(r.Context())
	teamID, ok := parseUUID(w, r, "teamID")
	if !ok {
		return
	}
	var body struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		IsPublic    *bool   `json:"is_public"`
	}
	if !handler.Decode(w, r, &body) {
		return
	}
	t, err := h.teams.Update(r.Context(), teamID, uid, body.Name, body.Description, body.IsPublic)
	if err != nil {
		h.handleError(w, r, err)
		return
	}
	handler.JSON(w, r, http.StatusOK, toTeamResponse(t))
}

func (h *Handler) DeleteTeam(w http.ResponseWriter, r *http.Request) {
	uid := jwtauth.MustUserID(r.Context())
	teamID, ok := parseUUID(w, r, "teamID")
	if !ok {
		return
	}
	if err := h.teams.Delete(r.Context(), teamID, uid); err != nil {
		h.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Membership ────────────────────────────────────────────────────────────────

func (h *Handler) ListMembers(w http.ResponseWriter, r *http.Request) {
	uid := jwtauth.MustUserID(r.Context())
	teamID, ok := parseUUID(w, r, "teamID")
	if !ok {
		return
	}
	members, err := h.teams.ListMembers(r.Context(), teamID, uid)
	if err != nil {
		h.handleError(w, r, err)
		return
	}
	if members == nil {
		members = []*team.TeamMember{}
	}
	handler.JSON(w, r, http.StatusOK, toMemberResponses(members))
}

func (h *Handler) InviteMember(w http.ResponseWriter, r *http.Request) {
	uid := jwtauth.MustUserID(r.Context())
	teamID, ok := parseUUID(w, r, "teamID")
	if !ok {
		return
	}
	var body struct {
		Email string `json:"email"`
	}
	if !handler.Decode(w, r, &body) {
		return
	}
	m, err := h.teams.InviteMember(r.Context(), teamID, uid, body.Email)
	if err != nil {
		h.handleError(w, r, err)
		return
	}
	handler.JSON(w, r, http.StatusCreated, toMemberResponse(m))
}

func (h *Handler) AddAnonymousMember(w http.ResponseWriter, r *http.Request) {
	uid := jwtauth.MustUserID(r.Context())
	teamID, ok := parseUUID(w, r, "teamID")
	if !ok {
		return
	}
	var body struct {
		UserID string `json:"user_id"`
	}
	if !handler.Decode(w, r, &body) {
		return
	}
	anonID, err := uuid.Parse(body.UserID)
	if err != nil {
		handler.Error(w, r, http.StatusBadRequest, "INVALID_ID", "invalid user_id")
		return
	}
	m, err := h.teams.AddAnonymousMember(r.Context(), teamID, uid, anonID)
	if err != nil {
		h.handleError(w, r, err)
		return
	}
	handler.JSON(w, r, http.StatusCreated, toMemberResponse(m))
}

func (h *Handler) RequestJoin(w http.ResponseWriter, r *http.Request) {
	uid := jwtauth.MustUserID(r.Context())
	teamID, ok := parseUUID(w, r, "teamID")
	if !ok {
		return
	}
	var body struct {
		Message string `json:"message"`
	}
	if !handler.Decode(w, r, &body) {
		return
	}
	m, err := h.teams.RequestJoin(r.Context(), teamID, uid, body.Message)
	if err != nil {
		h.handleError(w, r, err)
		return
	}
	handler.JSON(w, r, http.StatusCreated, toMemberResponse(m))
}

func (h *Handler) ListJoinRequests(w http.ResponseWriter, r *http.Request) {
	uid := jwtauth.MustUserID(r.Context())
	teamID, ok := parseUUID(w, r, "teamID")
	if !ok {
		return
	}
	requests, err := h.teams.ListJoinRequests(r.Context(), teamID, uid)
	if err != nil {
		h.handleError(w, r, err)
		return
	}
	if requests == nil {
		requests = []*team.TeamMember{}
	}
	handler.JSON(w, r, http.StatusOK, toMemberResponses(requests))
}

func (h *Handler) ApproveJoin(w http.ResponseWriter, r *http.Request) {
	uid := jwtauth.MustUserID(r.Context())
	teamID, ok := parseUUID(w, r, "teamID")
	if !ok {
		return
	}
	rid, ok := parseUUID(w, r, "rid")
	if !ok {
		return
	}
	m, err := h.teams.ApproveJoin(r.Context(), teamID, rid, uid)
	if err != nil {
		h.handleError(w, r, err)
		return
	}
	handler.JSON(w, r, http.StatusOK, toMemberResponse(m))
}

func (h *Handler) RejectJoin(w http.ResponseWriter, r *http.Request) {
	uid := jwtauth.MustUserID(r.Context())
	teamID, ok := parseUUID(w, r, "teamID")
	if !ok {
		return
	}
	rid, ok := parseUUID(w, r, "rid")
	if !ok {
		return
	}
	m, err := h.teams.RejectJoin(r.Context(), teamID, rid, uid)
	if err != nil {
		h.handleError(w, r, err)
		return
	}
	handler.JSON(w, r, http.StatusOK, toMemberResponse(m))
}

func (h *Handler) ChangeRole(w http.ResponseWriter, r *http.Request) {
	uid := jwtauth.MustUserID(r.Context())
	teamID, ok := parseUUID(w, r, "teamID")
	if !ok {
		return
	}
	targetUID, ok := parseUUID(w, r, "uid")
	if !ok {
		return
	}
	var body struct {
		Role string `json:"role"`
	}
	if !handler.Decode(w, r, &body) {
		return
	}
	m, err := h.teams.ChangeRole(r.Context(), teamID, targetUID, uid, body.Role)
	if err != nil {
		h.handleError(w, r, err)
		return
	}
	handler.JSON(w, r, http.StatusOK, toMemberResponse(m))
}

func (h *Handler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	uid := jwtauth.MustUserID(r.Context())
	teamID, ok := parseUUID(w, r, "teamID")
	if !ok {
		return
	}
	targetUID, ok := parseUUID(w, r, "uid")
	if !ok {
		return
	}
	if err := h.teams.RemoveMember(r.Context(), teamID, targetUID, uid); err != nil {
		h.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Invite links ──────────────────────────────────────────────────────────────

func (h *Handler) CreateInviteLink(w http.ResponseWriter, r *http.Request) {
	uid := jwtauth.MustUserID(r.Context())
	teamID, ok := parseUUID(w, r, "teamID")
	if !ok {
		return
	}
	var body struct {
		MaxUses        *int `json:"max_uses"`
		ExpiresInHours *int `json:"expires_in_hours"`
	}
	if !handler.Decode(w, r, &body) {
		return
	}
	link, rawToken, err := h.teams.CreateInviteLink(r.Context(), teamID, uid, body.MaxUses, body.ExpiresInHours)
	if err != nil {
		h.handleError(w, r, err)
		return
	}
	inviteURL := h.frontendURL + "/invite/" + rawToken
	handler.JSON(w, r, http.StatusCreated, inviteLinkResponse{
		ID:         link.ID,
		InviteURL:  inviteURL,
		MaxUses:    link.MaxUses,
		UseCount:   link.UseCount,
		ExpiresAt:  link.ExpiresAt,
		CreatedAt:  link.CreatedAt,
	})
}

func (h *Handler) ListInviteLinks(w http.ResponseWriter, r *http.Request) {
	uid := jwtauth.MustUserID(r.Context())
	teamID, ok := parseUUID(w, r, "teamID")
	if !ok {
		return
	}
	links, err := h.teams.ListInviteLinks(r.Context(), teamID, uid)
	if err != nil {
		h.handleError(w, r, err)
		return
	}
	if links == nil {
		links = []*team.InviteLink{}
	}
	resp := make([]inviteLinkResponse, len(links))
	for i, l := range links {
		resp[i] = inviteLinkResponse{
			ID:        l.ID,
			MaxUses:   l.MaxUses,
			UseCount:  l.UseCount,
			ExpiresAt: l.ExpiresAt,
			CreatedAt: l.CreatedAt,
		}
	}
	handler.JSON(w, r, http.StatusOK, resp)
}

func (h *Handler) RevokeInviteLink(w http.ResponseWriter, r *http.Request) {
	uid := jwtauth.MustUserID(r.Context())
	teamID, ok := parseUUID(w, r, "teamID")
	if !ok {
		return
	}
	lid, ok := parseUUID(w, r, "lid")
	if !ok {
		return
	}
	if err := h.teams.RevokeInviteLink(r.Context(), teamID, lid, uid); err != nil {
		h.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Response DTOs ─────────────────────────────────────────────────────────────

type teamResponse struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	Description *string    `json:"description,omitempty"`
	Currency    string     `json:"currency"`
	IsPublic    bool       `json:"is_public"`
	OwnerID     uuid.UUID  `json:"owner_id"`
	CreatedAt   time.Time  `json:"created_at"`
}

type memberResponse struct {
	ID             uuid.UUID  `json:"id"`
	TeamID         uuid.UUID  `json:"team_id"`
	UserID         uuid.UUID  `json:"user_id"`
	DisplayName    string     `json:"display_name"`
	AvatarURL      *string    `json:"avatar_url,omitempty"`
	IdentityType   string     `json:"identity_type"`
	Role           string     `json:"role"`
	Status         string     `json:"status"`
	JoinedAt       *time.Time `json:"joined_at,omitempty"`
	RequestMessage *string    `json:"request_message,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type inviteLinkResponse struct {
	ID        uuid.UUID  `json:"id"`
	InviteURL string     `json:"invite_url,omitempty"`
	MaxUses   *int       `json:"max_uses"`
	UseCount  int        `json:"use_count"`
	ExpiresAt *time.Time `json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
}

func toTeamResponse(t *team.Team) teamResponse {
	return teamResponse{
		ID:          t.ID,
		Name:        t.Name,
		Description: t.Description,
		Currency:    t.Currency,
		IsPublic:    t.IsPublic,
		OwnerID:     t.OwnerID,
		CreatedAt:   t.CreatedAt,
	}
}

func toTeamResponses(teams []*team.Team) []teamResponse {
	resp := make([]teamResponse, len(teams))
	for i, t := range teams {
		resp[i] = toTeamResponse(t)
	}
	return resp
}

func toMemberResponse(m *team.TeamMember) memberResponse {
	return memberResponse{
		ID:             m.ID,
		TeamID:         m.TeamID,
		UserID:         m.UserID,
		DisplayName:    m.UserDisplayName,
		AvatarURL:      m.UserAvatarURL,
		IdentityType:   m.UserIdentityType,
		Role:           m.Role,
		Status:         m.Status,
		JoinedAt:       m.JoinedAt,
		RequestMessage: m.RequestMessage,
		CreatedAt:      m.CreatedAt,
	}
}

func toMemberResponses(members []*team.TeamMember) []memberResponse {
	resp := make([]memberResponse, len(members))
	for i, m := range members {
		resp[i] = toMemberResponse(m)
	}
	return resp
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (h *Handler) handleError(w http.ResponseWriter, r *http.Request, err error) {
	switch err {
	case team.ErrNotFound:
		handler.Error(w, r, http.StatusNotFound, "TEAM_NOT_FOUND", "team not found")
	case team.ErrNotMember:
		handler.Error(w, r, http.StatusForbidden, "NOT_TEAM_MEMBER", "you are not a member of this team")
	case team.ErrInsufficientRole:
		handler.Error(w, r, http.StatusForbidden, "INSUFFICIENT_ROLE", "your role does not permit this action")
	case team.ErrAlreadyMember:
		handler.Error(w, r, http.StatusConflict, "ALREADY_MEMBER", "user is already a member of this team")
	case team.ErrTeamNotPublic:
		handler.Error(w, r, http.StatusBadRequest, "TEAM_NOT_PUBLIC", "this team does not accept join requests")
	case team.ErrInviteLinkInvalid:
		handler.Error(w, r, http.StatusBadRequest, "INVITE_LINK_EXHAUSTED", "invite link is invalid, expired, or exhausted")
	case team.ErrCannotRemoveOwner:
		handler.Error(w, r, http.StatusBadRequest, "CANNOT_REMOVE_OWNER", "cannot remove the team owner")
	default:
		handler.Error(w, r, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
	}
}

func parseUUID(w http.ResponseWriter, r *http.Request, param string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, param))
	if err != nil {
		handler.Error(w, r, http.StatusBadRequest, "INVALID_ID", "invalid "+param)
		return uuid.Nil, false
	}
	return id, true
}
