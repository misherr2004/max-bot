package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	maxbot "github.com/max-messenger/max-bot-api-client-go"

	"kontroler-ts/db"
	"kontroler-ts/handlers"
)

func main() {
	_ = godotenv.Load()

	token := os.Getenv("TOKEN")
	if token == "" {
		log.Fatal("TOKEN не задан в .env")
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./kontroler.db"
	}
	uploadsDir := os.Getenv("UPLOADS_DIR")
	if uploadsDir == "" {
		uploadsDir = "./uploads"
	}

	database, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("Ошибка открытия БД: %v", err)
	}
	defer database.Close()

	api, err := maxbot.New(token)
	if err != nil {
		log.Fatalf("Ошибка инициализации MAX API: %v", err)
	}

	bot := handlers.New(api, database, uploadsDir)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		<-sig
		log.Println("Завершение работы...")
		cancel()
	}()

	log.Println("Бот «Контролер-ТС» запущен")

	for upd := range api.GetUpdates(ctx) {
		u := upd
		go bot.HandleUpdate(ctx, u)
	}
}
