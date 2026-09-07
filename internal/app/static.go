package app

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"
)

func registerStaticFiles(app *fiber.App) {
	// Serve static files from the ./public directory.
	app.Get("/*", static.New("./public"))
}
