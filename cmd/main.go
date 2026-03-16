package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ssu-bench-api/internal/database"
	"ssu-bench-api/internal/handler"
	"ssu-bench-api/internal/middleware"
	"ssu-bench-api/internal/models"
	"ssu-bench-api/internal/repository"
	"ssu-bench-api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// === 1. Инициализация ===
	if err := godotenv.Load(); err != nil {
		log.Println("[WARN] .env file not found, using environment variables")
	}
	mode := os.Getenv("GIN_MODE")
	if mode == "" {
		mode = gin.DebugMode
	}
	gin.SetMode(mode)
	log.Printf("[INFO] Starting server in %s mode", mode)

	// === 2. Подключение к базе данных ===
	db, err := database.Connect()
	if err != nil {
		log.Fatalf("[FATAL] Failed to connect to database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("[ERROR] Failed to close database connection: %v", err)
		}
	}()
	log.Println("[INFO] Successfully connected to database")

	// === 3. Применение миграций ===
	log.Println("[INFO] Applying database migrations...")
	if err := database.Migrate(db); err != nil {
		log.Fatalf("[FATAL] Failed to apply migrations: %v", err)
	}
	log.Println("[INFO] All migrations applied successfully")

	// === 4. Инициализация зависимостей (DI) ===
	userRepo := repository.NewUserRepository(db)
	taskRepo := repository.NewTaskRepository(db)
	bidRepo := repository.NewBidRepository(db)
	paymentRepo := repository.NewPaymentRepository(db)

	userService := service.NewUserService(userRepo)
	taskService := service.NewTaskService(taskRepo, bidRepo, userRepo, db)
	bidService := service.NewBidService(bidRepo, taskRepo, userRepo, db)
	paymentService := service.NewPaymentService(paymentRepo, taskRepo, bidRepo, userRepo, db)

	authHandler := handler.NewAuthHandler(userService)
	taskHandler := handler.NewTaskHandler(taskService, bidService, paymentService)
	adminHandler := handler.NewAdminHandler(userService, taskService, paymentService)

	// === 5. Настройка роутера ===
	r := gin.New()

	// ИСПРАВЛЕНО: Используем кастомный логгер с request_id
	r.Use(middleware.LoggerWithWriter(os.Stdout))

	// ИСПРАВЛЕНО: Подключаем кастомный RecoveryHandler вместо стандартного
	r.Use(gin.CustomRecovery(middleware.PanicRecoveryHandler))

	r.Use(middleware.RequestID())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "ok",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	})

	api := r.Group("/api/v1")
	{
		auth := api.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
		}
		protected := api.Group("/")
		protected.Use(middleware.JWTAuth())
		{
			protected.GET("/me", authHandler.GetProfile)
			tasks := protected.Group("/tasks")
			{
				tasks.GET("", taskHandler.GetTasks)
				tasks.GET("/:id", taskHandler.GetTask)
			}
			customer := protected.Group("/")
			customer.Use(middleware.RequireRole(models.RoleCustomer, models.RoleAdmin))
			{
				customer.POST("/tasks", taskHandler.CreateTask)
				customer.POST("/tasks/:id/select-bid", taskHandler.SelectBid)
				customer.POST("/tasks/:id/confirm", taskHandler.ConfirmCompletion)
				customer.POST("/tasks/:id/cancel", taskHandler.CancelTask)
			}
			executor := protected.Group("/")
			executor.Use(middleware.RequireRole(models.RoleExecutor, models.RoleAdmin))
			{
				executor.POST("/tasks/:id/bids", taskHandler.CreateBid)
				executor.POST("/bids/:id/complete", taskHandler.MarkBidCompleted)
			}
			// === Admin routes (только для admin) ===
			admin := protected.Group("/admin")
			admin.Use(middleware.RequireRole(models.RoleAdmin))
			{
				admin.GET("/users", adminHandler.ListUsers)
				admin.GET("/users/:id", adminHandler.GetUserDetails)
				admin.POST("/users/:id/block", adminHandler.BlockUser)
				admin.POST("/users/:id/unblock", adminHandler.UnblockUser)
				admin.GET("/payments", adminHandler.ListPayments)
				admin.GET("/tasks", adminHandler.ListAllTasks)
			}
		}
	}

	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "not_found",
			"message": "endpoint not found",
			"path":    c.Request.URL.Path,
		})
	})

	// === 6. Настройка HTTP-сервера ===
	port := getEnv("PORT", "8080")
	readTimeout := getEnvDuration("HTTP_READ_TIMEOUT", "15s")
	writeTimeout := getEnvDuration("HTTP_WRITE_TIMEOUT", "15s")
	idleTimeout := getEnvDuration("HTTP_IDLE_TIMEOUT", "60s")

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}

	// === 7. Запуск сервера с graceful shutdown ===
	log.Printf("[INFO] Starting HTTP server on :%s", port)
	log.Printf("[INFO] Timeouts: read=%v, write=%v, idle=%v", readTimeout, writeTimeout, idleTimeout)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[FATAL] Server failed to start: %v", err)
		}
	}()

	<-quit
	log.Println("[INFO] Shutdown signal received, starting graceful shutdown...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("[FATAL] Server forced to shutdown: %v", err)
	}
	log.Println("[INFO] Server exited gracefully")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvDuration(key, defaultValue string) time.Duration {
	value := getEnv(key, defaultValue)
	duration, err := time.ParseDuration(value)
	if err != nil {
		log.Printf("[WARN] Failed to parse duration %s=%s, using default %s", key, value, defaultValue)
		d, _ := time.ParseDuration(defaultValue)
		return d
	}
	return duration
}
