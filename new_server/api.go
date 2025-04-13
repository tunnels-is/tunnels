package main

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	gflog "github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/tunnels-is/tunnels/types"
)

func StartAPI() {

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
		tokenGroup.Get("/validate", handleTokenValidate)
		tokenGroup.Delete("/:token_uuid", handleTokenDelete)
		tokenGroup.Delete("/user/:user_uuid?", handleTokenDeleteAll)

		twoFAGroup := authGroup.Group("/2fa")
		twoFAGroup.Post("/setup", handle2FASetup)
		twoFAGroup.Post("/verify", handle2FAVerify)
		twoFAGroup.Post("/verify/:user_uuid", handle2FAVerify)

		userGroup := app.Group("/users")
		userGroup.Post("/", handleCreateUser)
		userGroup.Get("/", handleListUsers)
		userGroup.Get("/:uuid", handleGetUser)
		userGroup.Put("/:uuid", handleUpdateUser)
		userGroup.Delete("/:uuid", handleDeleteUser)

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
	for i := range ClientCoreMappings {
		if ClientCoreMappings[i] == nil {
			continue
		}

		if ClientCoreMappings[i].DHCP != nil {
			for _, v := range response.Devices {
				if v.DHCP.Token == ClientCoreMappings[i].DHCP.Token {
					continue outerloop
				}
			}
		}

		d := new(types.ListDevice)
		d.AllowedIPs = make([]string, 0)
		for _, v := range ClientCoreMappings[i].AllowedHosts {
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

		d.RAM = ClientCoreMappings[i].RAM
		d.CPU = ClientCoreMappings[i].CPU
		d.Disk = ClientCoreMappings[i].Disk
		if ClientCoreMappings[i].DHCP != nil {
			response.DHCPAssigned++
			d.DHCP = *ClientCoreMappings[i].DHCP
		}

		d.IngressQueue = len(ClientCoreMappings[i].ToUser)
		d.EgressQueue = len(ClientCoreMappings[i].FromUser)
		d.Created = ClientCoreMappings[i].Created
		if ClientCoreMappings[i].PortRange != nil {
			d.StartPort = ClientCoreMappings[i].PortRange.StartPort
			d.EndPort = ClientCoreMappings[i].PortRange.EndPort
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
