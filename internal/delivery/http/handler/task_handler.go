package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/example/task-management-api/internal/delivery/http/middleware"
	"github.com/example/task-management-api/internal/domain"
	"github.com/example/task-management-api/internal/usecase"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type TaskHandler struct{ usecase *usecase.TaskUsecase }

func NewTaskHandler(usecase *usecase.TaskUsecase) *TaskHandler { return &TaskHandler{usecase: usecase} }

type createTaskRequest struct {
	Title       string            `json:"title" binding:"required"`
	Description string            `json:"description"`
	Status      domain.TaskStatus `json:"status"`
	DueDate     *time.Time        `json:"due_date"`
}

type updateTaskRequest struct {
	Title       *string            `json:"title"`
	Description *string            `json:"description"`
	Status      *domain.TaskStatus `json:"status"`
	DueDate     *time.Time         `json:"due_date"`
}

type assignTaskRequest struct {
	AssigneeID uuid.UUID `json:"assignee_id" binding:"required"`
}

func (h *TaskHandler) Create(c *gin.Context) {
	ownerID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	key, err := uuid.Parse(c.GetHeader("Idempotency-Key"))
	if err != nil || key == uuid.Nil {
		middleware.AbortWithError(c, middleware.NewAppError(http.StatusBadRequest, "INVALID_IDEMPOTENCY_KEY", "Idempotency-Key must be a UUID", err))
		return
	}
	var request createTaskRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		abortInvalidJSON(c, err)
		return
	}
	result, err := h.usecase.CreateIdempotent(c.Request.Context(), ownerID, key, usecase.CreateTaskInput(request))
	if err != nil {
		abortDomainError(c, err)
		return
	}
	c.Data(result.StatusCode, "application/json; charset=utf-8", result.Body)
}

func (h *TaskHandler) List(c *gin.Context) {
	ownerID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	filter, err := taskFilterFromQuery(c)
	if err != nil {
		middleware.AbortWithError(c, middleware.NewAppError(http.StatusBadRequest, "INVALID_QUERY", "query parameters are invalid", err))
		return
	}
	result, err := h.usecase.List(c.Request.Context(), ownerID, filter)
	if err != nil {
		abortDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *TaskHandler) GetByID(c *gin.Context) {
	ownerID, taskID, ok := userAndTaskID(c)
	if !ok {
		return
	}
	task, err := h.usecase.GetByID(c.Request.Context(), ownerID, taskID)
	if err != nil {
		abortDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, task)
}

func (h *TaskHandler) Update(c *gin.Context) {
	ownerID, taskID, ok := userAndTaskID(c)
	if !ok {
		return
	}
	var request updateTaskRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		abortInvalidJSON(c, err)
		return
	}
	task, err := h.usecase.Update(c.Request.Context(), ownerID, taskID, domain.TaskUpdate(request))
	if err != nil {
		abortDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, task)
}

func (h *TaskHandler) Delete(c *gin.Context) {
	ownerID, taskID, ok := userAndTaskID(c)
	if !ok {
		return
	}
	if err := h.usecase.Delete(c.Request.Context(), ownerID, taskID); err != nil {
		abortDomainError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *TaskHandler) Assign(c *gin.Context) {
	ownerID, taskID, ok := userAndTaskID(c)
	if !ok {
		return
	}
	var request assignTaskRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		abortInvalidJSON(c, err)
		return
	}
	task, err := h.usecase.Assign(c.Request.Context(), ownerID, taskID, request.AssigneeID)
	if err != nil {
		abortDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, task)
}

func authenticatedUserID(c *gin.Context) (uuid.UUID, bool) {
	value, ok := c.Get(middleware.AuthenticatedUserIDKey)
	if !ok {
		middleware.AbortWithError(c, middleware.NewAppError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication is required", nil))
		return uuid.Nil, false
	}
	userID, ok := value.(uuid.UUID)
	if !ok {
		middleware.AbortWithError(c, middleware.NewAppError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication is required", nil))
		return uuid.Nil, false
	}
	return userID, true
}

func userAndTaskID(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	ownerID, ok := authenticatedUserID(c)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.AbortWithError(c, middleware.NewAppError(http.StatusBadRequest, "INVALID_TASK_ID", "task ID must be a UUID", err))
		return uuid.Nil, uuid.Nil, false
	}
	return ownerID, taskID, true
}

func taskFilterFromQuery(c *gin.Context) (domain.TaskFilter, error) {
	filter := domain.TaskFilter{Search: c.Query("search")}
	var err error
	if value := c.Query("page"); value != "" {
		filter.Page, err = strconv.Atoi(value)
		if err != nil {
			return domain.TaskFilter{}, err
		}
	}
	if value := c.Query("limit"); value != "" {
		filter.Limit, err = strconv.Atoi(value)
		if err != nil {
			return domain.TaskFilter{}, err
		}
	}
	if value := c.Query("status"); value != "" {
		status := domain.TaskStatus(value)
		filter.Status = &status
	}
	return filter, nil
}
