# Pokémon Showdown Client

A client that connects to [Pokémon Showdown](https://pokemonshowdown.com/), written in Go.

> **Status:** Work in progress. Contributions and feedback are welcome!

## Features

- Connects to Pokémon Showdown servers
(that's literally it for now)

## Getting Started

```go
import (
    "github.com/PartMan7/ps-go"
)

func Main() {
	Bot := PSGo.New("Username", "password", []string{"botdevelopment"})

	Bot.OnMessage = func(message PSGo.Message) {
		if !message.BeforeJoin && message.Content == "Ping!" {
			Bot.SendRoom(message.Room, "Pong!")
		}
	}
	Bot.Connect()
}
```

## Contributing

Feel free to open issues or submit pull requests!

## License

MIT