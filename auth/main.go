package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	gflog "github.com/gofiber/fiber/v2/middleware/logger" // Renamed to avoid clash
	"github.com/gofiber/fiber/v2/middleware/recover"
)

var logger *slog.Logger

func main() {
	logHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}) // Use LevelInfo in prod
	logger = slog.New(logHandler)
	slog.SetDefault(logger)

	logger.Info("Initializing databases...")
	if err := initDBs(logger); err != nil {
		logger.Error("Fatal: Failed to initialize databases", slog.Any("error", err))
		os.Exit(1)
	}
	defer closeDBs(logger)

	logger.Info("Setting up Google OAuth...")
	if err := setupGoogleOAuth(); err != nil {
		logger.Error("Fatal: Failed to setup Google OAuth", slog.Any("error", err))
		os.Exit(1)
	}

	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			message := "Internal Server Error"

			var e *fiber.Error
			if errors.As(err, &e) {
				code = e.Code
				message = e.Message
			} else {
				logger.Error("Unhandled error in handler", slog.Any("error", err), slog.String("path", c.Path()))
			}

			if code >= 500 {
				// Already logged above if it wasn't a fiber.Error
			} else {
				logger.Warn("Client error occurred", slog.Int("status", code), slog.String("message", message), slog.String("path", c.Path()))
			}

			c.Status(code)
			return c.JSON(fiber.Map{
				"error":   true,
				"message": message,
			})
		},
	})

	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: fmt.Sprintf("Origin, Content-Type, Accept, %s", AuthHeader),
		AllowMethods: "GET, POST, PUT, DELETE, OPTIONS",
	}))

	app.Use(recover.New())

	app.Use(gflog.New(gflog.Config{
		TimeFormat: "2006-01-02 15:04:05",
		Output:     os.Stdout,
	}))

	// --- Routes ---

	// Health Check
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok"})
	})

	// Authentication Routes
	authGroup := app.Group("/auth")
	if googleOAuthEnabled() {
		authGroup.Get("/google/login", handleGoogleLogin)       // Redirects to Google
		authGroup.Get("/google/callback", handleGoogleCallback) // Handles callback from Google
	}

	tokenGroup := authGroup.Group("/token")
	tokenGroup.Get("/validate", handleTokenValidate)             // Validates the token in X-Auth-Token header
	tokenGroup.Delete("/:token_uuid", handleTokenDelete)         // Delete specific token (self or admin)
	tokenGroup.Delete("/user/:user_uuid?", handleTokenDeleteAll) // Delete all tokens for a user (self default, admin target)

	twoFAGroup := authGroup.Group("/2fa")
	twoFAGroup.Post("/setup", handle2FASetup)              // Generate secret/QR (Requires Auth)
	twoFAGroup.Post("/verify", handle2FAVerify)            // Verify initial code to enable (Requires Auth)
	twoFAGroup.Post("/verify/:user_uuid", handle2FAVerify) // Verify code during login (Requires User UUID)

	userGroup := app.Group("/users")
	userGroup.Post("/", handleCreateUser)        // Create User (Admin/Manager)
	userGroup.Get("/", handleListUsers)          // List Users (Admin/Manager)
	userGroup.Get("/:uuid", handleGetUser)       // Get User (Admin/Manager/Self)
	userGroup.Put("/:uuid", handleUpdateUser)    // Update User (Permissions vary by field)
	userGroup.Delete("/:uuid", handleDeleteUser) // Delete User (Admin Only)

	groupGroup := app.Group("/groups")
	groupGroup.Post("/", handleCreateGroup)        // Create Group (Admin/Manager)
	groupGroup.Get("/", handleListGroups)          // List Groups (Admin/Manager)
	groupGroup.Get("/:uuid", handleGetGroup)       // Get Group (Admin/Manager)
	groupGroup.Put("/:uuid", handleUpdateGroup)    // Update Group (Admin/Manager)
	groupGroup.Delete("/:uuid", handleDeleteGroup) // Delete Group (Admin/Manager)

	groupMembershipGroup := groupGroup.Group("/:group_uuid")
	groupMembershipGroup.Put("/users/:user_uuid", handleAddUserToGroup)               // Add User to Group (Admin/Manager)
	groupMembershipGroup.Delete("/users/:user_uuid", handleRemoveUserFromGroup)       // Remove User from Group (Admin/Manager)
	groupMembershipGroup.Put("/servers/:server_uuid", handleAddServerToGroup)         // Add Server to Group (Admin)
	groupMembershipGroup.Delete("/servers/:server_uuid", handleRemoveServerFromGroup) // Remove Server from Group (Admin)

	serverGroup := app.Group("/servers")
	serverGroup.Post("/", handleCreateServer)        // Create Server (Admin)
	serverGroup.Get("/", handleListServers)          // List Servers (Admin)
	serverGroup.Get("/:uuid", handleGetServer)       // Get Server (Admin)
	serverGroup.Put("/:uuid", handleUpdateServer)    // Update Server (Admin)
	serverGroup.Delete("/:uuid", handleDeleteServer) // Delete Server (Admin)

	// Start Server
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	logger.Info("Starting server...", slog.String("port", port))
	err := app.Listen(":" + port)
	if err != nil {
		logger.Error("Fatal: Server failed to start", slog.Any("error", err))
		os.Exit(1)
	}
}
