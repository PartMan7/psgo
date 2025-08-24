package main

import (
	"log"
	"os"

	PSGo "github.com/PartMan7/ps-go"

	"github.com/joho/godotenv"
)

func main() {
	envErr := godotenv.Load(".env")
	if envErr != nil {
		log.Fatalf("Could not load .env! If it does not exist, please create it.")
	}

	Bot := PSGo.New(os.Getenv("PS_USERNAME"), os.Getenv("PS_PASSWORD"), []string{"botdevelopment"})

	Bot.OnMessage = func(message PSGo.Message) {
		if message.BeforeJoin {
			return
		}
		if message.Content == "Ping!" {
			Bot.SendRoom(message.Room, "Pong!")
		}
	}
	Bot.OnConnect = func() {
		log.Println("Connected to Pokémon Showdown")
	}
	Bot.Connect()
}
