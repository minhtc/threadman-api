package app

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestRegisterStaticFiles(t *testing.T) {
	t.Chdir("../..")

	app := fiber.New()
	app.Get("/healthz", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})
	registerStaticFiles(app)

	response, err := app.Test(httptest.NewRequest("GET", "/robots.txt", nil))
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("robots.txt status = %d, want %d", response.StatusCode, fiber.StatusOK)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "User-agent: *") {
		t.Fatalf("robots.txt body = %q", body)
	}

	response, err = app.Test(httptest.NewRequest("GET", "/index.html", nil))
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("index.html status = %d, want %d", response.StatusCode, fiber.StatusOK)
	}
	body, err = io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "Leaderboard") {
		t.Fatalf("index.html does not contain the dashboard title")
	}

	response, err = app.Test(httptest.NewRequest("GET", "/healthz", nil))
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("healthz status = %d, want %d", response.StatusCode, fiber.StatusOK)
	}
}
