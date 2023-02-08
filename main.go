package main

import (
	"embed"
	"math/rand"
	"os/exec"
	"runtime"
	"time"

	_ "github.com/Gasoid/photoDumper/docs"
	"github.com/Gasoid/photoDumper/sources"
	"github.com/Gasoid/photoDumper/sources/instagram"
	"github.com/Gasoid/photoDumper/sources/vk"

	local "github.com/Gasoid/photoDumper/storage/localfs"
)

// open opens the specified URL in the default browser of the user.
func open(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
	}
	args = append(args, url)
	return exec.Command(cmd, args...).Start()
}

//go:embed build/*
var staticAssets embed.FS

// @title        PhotoDumper
// @version      1.1.2
// @description  app downloads photos from vk.

// @contact.name  Rinat Almakhov
// @contact.url   https://github.com/Gasoid/

// @license.name  MIT License
// @license.url   https://github.com/Gasoid/photoDumper/blob/main/LICENSE

// @host      localhost:8080
// @BasePath  /api/
// @securitydefinitions.apikey ApiKeyAuth
// @in query
// @name api_key
func main() {
	rand.Seed(time.Now().UnixNano())
	sources.AddSource(vk.NewService())
	sources.AddSource(instagram.NewService())
	sources.AddStorage(local.NewService())
	router := setupRouter()
	if router != nil {
		// go open("http://localhost:8080/")
		router.Run(":8080")
	}
}
