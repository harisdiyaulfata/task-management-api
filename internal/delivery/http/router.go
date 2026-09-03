package delivery

import (
	"log/slog"
	"net/http"

	"github.com/example/task-management-api/internal/delivery/http/handler"
	"github.com/example/task-management-api/internal/delivery/http/middleware"
	"github.com/example/task-management-api/internal/domain"
	"github.com/example/task-management-api/internal/platform/jwt"
	"github.com/gin-gonic/gin"
)

func NewRouter(
	log *slog.Logger,
	authHandler *handler.AuthHandler,
	taskHandler *handler.TaskHandler,
	teamHandler *handler.TeamHandler,
	tokens *jwt.Manager,
	sessions domain.SessionRepository,
) *gin.Engine {
	router := gin.New()
	router.HandleMethodNotAllowed = true
	router.Use(
		middleware.RequestID(),
		middleware.StructuredLogger(log),
		middleware.ErrorHandler(log),
	)

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.NoRoute(func(c *gin.Context) {
		middleware.AbortWithError(c, middleware.NewAppError(http.StatusNotFound, "ROUTE_NOT_FOUND", "route not found", nil))
	})
	router.NoMethod(func(c *gin.Context) {
		middleware.AbortWithError(c, middleware.NewAppError(http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", nil))
	})

	auth := router.Group("/auth")
	auth.POST("/register", authHandler.Register)
	auth.POST("/login", authHandler.Login)
	auth.POST("/logout", middleware.Authenticate(tokens, sessions), authHandler.Logout)

	tasks := router.Group("/tasks", middleware.Authenticate(tokens, sessions))
	tasks.POST("", taskHandler.Create)
	tasks.GET("", taskHandler.List)
	tasks.GET("/:id", taskHandler.GetByID)
	tasks.PUT("/:id", taskHandler.Update)
	tasks.DELETE("/:id", taskHandler.Delete)
	tasks.POST("/:id/assign", taskHandler.Assign)

	teams := router.Group("/teams", middleware.Authenticate(tokens, sessions))
	teams.POST("", teamHandler.Create)
	teams.GET("", teamHandler.ListMyTeams)
	teams.GET("/:id/members", teamHandler.ListMembers)
	teams.POST("/:id/members", teamHandler.AddMember)
	teams.DELETE("/:id/members/:userId", teamHandler.RemoveMember)

	return router
}
