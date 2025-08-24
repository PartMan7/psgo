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

	psgo := PSGo.New(os.Getenv("PS_USERNAME"), os.Getenv("PS_PASSWORD"))

	psgo.Connect()
}
