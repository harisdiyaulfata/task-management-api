package handler

import (
	"net/http"

	"github.com/example/task-management-api/internal/usecase"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type TeamHandler struct{ usecase *usecase.TeamUsecase }

func NewTeamHandler(usecase *usecase.TeamUsecase) *TeamHandler { return &TeamHandler{usecase: usecase} }

type createTeamRequest struct {
	Name string `json:"name" binding:"required,max=100"`
}

type addTeamMemberRequest struct {
	UserID uuid.UUID `json:"user_id" binding:"required"`
}

func (h *TeamHandler) Create(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	var request createTeamRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		abortInvalidJSON(c, err)
		return
	}
	team, err := h.usecase.Create(c.Request.Context(), userID, request.Name)
	if err != nil {
		abortDomainError(c, err)
		return
	}
	c.JSON(http.StatusCreated, team)
}

func (h *TeamHandler) ListMyTeams(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	teams, err := h.usecase.ListMyTeams(c.Request.Context(), userID)
	if err != nil {
		abortDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": teams})
}

func (h *TeamHandler) ListMembers(c *gin.Context) {
	requesterID, teamID, ok := userAndTeamID(c)
	if !ok {
		return
	}
	members, err := h.usecase.ListMembers(c.Request.Context(), requesterID, teamID)
	if err != nil {
		abortDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": members})
}

func (h *TeamHandler) AddMember(c *gin.Context) {
	requesterID, teamID, ok := userAndTeamID(c)
	if !ok {
		return
	}
	var request addTeamMemberRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		abortInvalidJSON(c, err)
		return
	}
	if err := h.usecase.AddMember(c.Request.Context(), requesterID, teamID, request.UserID); err != nil {
		abortDomainError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *TeamHandler) RemoveMember(c *gin.Context) {
	requesterID, teamID, ok := userAndTeamID(c)
	if !ok {
		return
	}
	userID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		abortInvalidJSON(c, err)
		return
	}
	if err := h.usecase.RemoveMember(c.Request.Context(), requesterID, teamID, userID); err != nil {
		abortDomainError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func userAndTeamID(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	teamID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		abortInvalidJSON(c, err)
		return uuid.Nil, uuid.Nil, false
	}
	return userID, teamID, true
}
