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
    bot := PSGo.New("username", "password")
	bot.Connect()
}
```

## Contributing

Feel free to open issues or submit pull requests!

## License

MIT