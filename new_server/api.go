package main

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/tunnels-is/tunnels/types"
)

func StartAPI() {

	app := fiber.New(fiber.Config{
		ErrorHandler:          APIErrorHandler,
		IdleTimeout:           time.Second * 60,
		WriteTimeout:          time.Second * 60,
		ReadTimeout:           time.Second * 60,
		DisableStartupMessage: true,
		DisableKeepalive:      true,
	})

	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: fmt.Sprintf("Origin, Content-Type, Accept, %s", authHeader),
		AllowMethods: "GET, POST, PUT, DELETE, OPTIONS",
	}))

	app.Use(TimingMiddleware())
	app.Use(recover.New())

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok"})
	})

	if AUTHEnabled {
		authGroup := app.Group("/auth")
		if googleOAuthEnabled() {
			authGroup.Get("/google/login", handleGoogleLogin)
			authGroup.Get("/google/callback", handleGoogleCallback)
		}

		authGroup.Post("/login", handleLogin)
		authGroup.Post("/logout", handleLogout)

		tokenGroup := authGroup.Group("/token")
		// tokenGroup.Get("/validate", handleTokenValidate)
		tokenGroup.Delete("/:token_uuid", handleTokenDelete)
		tokenGroup.Delete("/user/:user_uuid", handleTokenDeleteAll)

		twoFAGroup := authGroup.Group("/2fa")
		twoFAGroup.Post("/setup", handle2FASetup)
		twoFAGroup.Post("/enable/:user_uuid", handle2FAEnable)
		twoFAGroup.Post("/confirm", handle2FAConfirm)

		userGroup := app.Group("/users")
		userGroup.Post("/", handleCreateUser)
		userGroup.Get("/", handleListUsers)
		userGroup.Get("/:uuid", handleGetUser)
		userGroup.Put("/:uuid", handleUpdateUser)
		// userGroup.Delete("/:uuid", handleDeleteUser)

		groupGroup := app.Group("/groups")
		groupGroup.Post("/", handleCreateGroup)
		groupGroup.Get("/", handleListGroups)
		groupGroup.Get("/:uuid", handleGetGroup)
		groupGroup.Put("/:uuid", handleUpdateGroup)
		groupGroup.Delete("/:uuid", handleDeleteGroup)

		groupMembershipGroup := groupGroup.Group("/:group_uuid")
		groupMembershipGroup.Put("/users/:user_uuid", handleAddUserToGroup)
		groupMembershipGroup.Delete("/users/:user_uuid", handleRemoveUserFromGroup)
		groupMembershipGroup.Put("/servers/:server_uuid", handleAddServerToGroup)
		groupMembershipGroup.Delete("/servers/:server_uuid", handleRemoveServerFromGroup)

		serverGroup := app.Group("/servers")
		serverGroup.Post("/", handleCreateServer)
		serverGroup.Get("/", handleListServers)
		serverGroup.Get("/:uuid", handleGetServer)
		serverGroup.Put("/:uuid", handleUpdateServer)
		serverGroup.Delete("/:uuid", handleDeleteServer)
	}

	if LANEnabled {
		lanGroup := app.Group("/lan")
		lanGroup.Post("/firewall", HTTP_Firewall)
		lanGroup.Get("/devices", HTTP_ListDevices)
	}

	Config := Config.Load()
	addr := fmt.Sprintf("%s:%s",
		Config.APIIP,
		Config.APIPort,
	)

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		MaxVersion: tls.VersionTLS13,
		// CurvePreferences: []tls.CurveID{tls.X25519MLKEM768, tls.CurveP521},
		Certificates: []tls.Certificate{*KeyPair.Load()}}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", addr, err)
	}

	tlsListener := tls.NewListener(ln, tlsConfig)
	logger.Info("Starting server...", slog.String("addr", addr))
	if err := app.Listener(tlsListener); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}

func APIErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	message := "Internal Server Error"

	var e *fiber.Error
	if errors.As(err, &e) {
		code = e.Code
		message = e.Message
	} else {
		logger.Error("middlware caught error", slog.Any("error", err), slog.String("path", c.Path()), slog.Any("stacktrace", getStacktraceLines(3)))
	}

	c.Status(code)
	return c.JSON(fiber.Map{
		"message": message,
	})
}

func TimingMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		slog.Info("API",
			slog.Any("method", c.Method()),
			slog.Any("path", c.Path()),
			slog.Any("code", c.Response().StatusCode()),
			slog.Any("ms", time.Since(start).Milliseconds()),
		)
		return err
	}
}

func errResponse(c *fiber.Ctx, code int, msg string, slogArgs ...any) error {
	logger.Error(msg, slogArgs...)
	return c.Status(code).JSON(fiber.Map{"message": msg})
}

func HTTP_validateKey(c *fiber.Ctx) (ok bool) {
	key := c.Get("X-API-KEY")
	Config := Config.Load()
	if key != Config.AdminApiKey || Config.AdminApiKey != "" {
		return false
	}
	return true
}

func HTTP_ListDevices(c *fiber.Ctx) error {
	if !HTTP_validateKey(c) {
		return errResponse(c, 401, "Unauthorized")
	}

	response := new(types.DeviceListResponse)
	response.Devices = make([]*types.ListDevice, 0)
outerloop:
	for i := range clientCoreMappings {
		if clientCoreMappings[i] == nil {
			continue
		}

		if clientCoreMappings[i].DHCP != nil {
			for _, v := range response.Devices {
				if v.DHCP.Token == clientCoreMappings[i].DHCP.Token {
					continue outerloop
				}
			}
		}

		d := new(types.ListDevice)
		d.AllowedIPs = make([]string, 0)
		for _, v := range clientCoreMappings[i].AllowedHosts {
			if v.Type == "auto" {
				continue
			}
			d.AllowedIPs = append(d.AllowedIPs,
				fmt.Sprintf("%d-%d-%d-%d",
					v.IP[0],
					v.IP[1],
					v.IP[2],
					v.IP[3],
				))
		}

		d.RAM = clientCoreMappings[i].RAM
		d.CPU = clientCoreMappings[i].CPU
		d.Disk = clientCoreMappings[i].Disk
		if clientCoreMappings[i].DHCP != nil {
			response.DHCPAssigned++
			d.DHCP = *clientCoreMappings[i].DHCP
		}

		d.IngressQueue = len(clientCoreMappings[i].ToUser)
		d.EgressQueue = len(clientCoreMappings[i].FromUser)
		d.Created = clientCoreMappings[i].Created
		if clientCoreMappings[i].PortRange != nil {
			d.StartPort = clientCoreMappings[i].PortRange.StartPort
			d.EndPort = clientCoreMappings[i].PortRange.EndPort
		}
		response.Devices = append(response.Devices, d)
	}

	response.DHCPFree = len(DHCPMapping) - response.DHCPAssigned

	for i := range response.Devices {
		response.Devices[i].DHCP.Token = "redacted"
	}

	err := json.NewEncoder(c.Response().BodyWriter()).Encode(response)
	if err != nil {
		return errResponse(c, 500, "encoding error", err)
	}
	return nil
}

func HTTP_Firewall(c *fiber.Ctx) error {
	fr := new(types.FirewallRequest)
	if err := c.BodyParser(&fr); err != nil {
		return errResponse(c, fiber.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err), slog.Any("error", err))
	}

	mapping := validateDHCPTokenAndIP(fr)
	if mapping == nil {
		return errResponse(c, 401, "Unauthorized")
	}

	syncFirewallState(fr, mapping)

	return c.SendStatus(200)
}
