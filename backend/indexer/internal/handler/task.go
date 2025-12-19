package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/video-analitics/indexer/internal/middleware"
	"github.com/video-analitics/indexer/internal/repo"
	"go.mongodb.org/mongo-driver/mongo"
)

type TaskHandler struct {
	taskRepo *repo.ScanTaskRepo
	db       *mongo.Database
}

func NewTaskHandler(taskRepo *repo.ScanTaskRepo, db *mongo.Database) *TaskHandler {
	return &TaskHandler{
		taskRepo: taskRepo,
		db:       db,
	}
}

// GetTask godoc
// @Summary Get scan task by ID
// @Description Get scan task details and status
// @Tags tasks
// @Produce json
// @Param id path string true "Task ID"
// @Success 200 {object} repo.ScanTask
// @Failure 404 {object} ErrorResponse
// @Router /api/scan-tasks/{id} [get]
func (h *TaskHandler) Get(c *fiber.Ctx) error {
	id := c.Params("id")

	task, err := h.taskRepo.FindByID(c.Context(), id)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to fetch task"})
	}
	if task == nil {
		return c.Status(404).JSON(ErrorResponse{Error: "task not found"})
	}

	return c.JSON(task)
}

type ListTasksResponse struct {
	Items []repo.ScanTask `json:"items"`
	Total int64           `json:"total"`
}

// ListTasks godoc
// @Summary List recent scan tasks
// @Description Get list of recent scan tasks
// @Tags tasks
// @Security BearerAuth
// @Produce json
// @Param site_id query string false "Filter by site ID"
// @Param domain query string false "Filter by domain (partial match)"
// @Param status query string false "Filter by status (pending, processing, completed, failed, cancelled)"
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} ListTasksResponse
// @Router /api/scan-tasks [get]
func (h *TaskHandler) List(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	isAdmin := middleware.IsAdmin(c)

	siteID := c.Query("site_id")
	domain := c.Query("domain")
	status := c.Query("status")
	limit, _ := strconv.ParseInt(c.Query("limit", "20"), 10, 64)
	offset, _ := strconv.ParseInt(c.Query("offset", "0"), 10, 64)

	if limit > 1000 {
		limit = 1000
	}

	var tasks []repo.ScanTask
	var total int64
	var err error

	if isAdmin {
		tasks, total, err = h.taskRepo.FindWithPagination(c.Context(), siteID, domain, status, limit, offset)
	} else {
		// Use aggregation pipeline - efficient even with millions of sites
		tasks, total, err = h.taskRepo.FindByUserAccess(c.Context(), userID, h.db, siteID, domain, status, limit, offset)
	}

	if err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to fetch tasks"})
	}

	if tasks == nil {
		tasks = []repo.ScanTask{}
	}

	return c.JSON(ListTasksResponse{Items: tasks, Total: total})
}

type CancelTasksRequest struct {
	TaskIDs []string `json:"task_ids"`
}

type CancelTasksResponse struct {
	CancelledCount int64 `json:"cancelled_count"`
}

// CancelTasks godoc
// @Summary Cancel scan tasks
// @Description Cancel pending or processing scan tasks
// @Tags tasks
// @Accept json
// @Produce json
// @Param request body CancelTasksRequest true "Task IDs to cancel"
// @Success 200 {object} CancelTasksResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/scan-tasks/cancel [post]
func (h *TaskHandler) Cancel(c *fiber.Ctx) error {
	var req CancelTasksRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(ErrorResponse{Error: "invalid request body"})
	}

	if len(req.TaskIDs) == 0 {
		return c.Status(400).JSON(ErrorResponse{Error: "task_ids required"})
	}

	cancelled, err := h.taskRepo.CancelMany(c.Context(), req.TaskIDs)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to cancel tasks"})
	}

	return c.JSON(CancelTasksResponse{CancelledCount: cancelled})
}
