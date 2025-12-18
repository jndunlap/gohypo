package main

import (
	"log"

	"gohypo/ui"
)

func main() {
	app, err := ui.NewApp(ui.Config{
		Port: "8080",
	})
	if err != nil {
		log.Fatal("Failed to create UI app:", err)
	}

	log.Println("Starting GoHypo UI on http://localhost:8080")
	log.Fatal(app.Start())
}
