package PSGo

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"

	"github.com/google/go-querystring/query"
	"github.com/gorilla/websocket"
)

type Challstr struct {
	challengekeyid string
	challstr       string
}

type Client struct {
	connection *websocket.Conn
	Connect    func()
	Send       func(line string)

	login    func(challstr Challstr)
	onLine   func(room string, line string, beforeJoin bool)
	onBatch  func(batchMessage string)
	onPacket func(packet string)
}

func toId(input string) string {
	return regexp.MustCompile("[^a-z0-9]").ReplaceAllString(strings.ToLower(input), "")
}

func getNonce() string {
	const allchars = "abcdefghijklmnopqrstuvwxyz0123456789"
	chars := make([]byte, 8)
	id := 100 + rand.Intn(900)
	for i := range chars {
		chars[i] = allchars[rand.Intn(len(allchars))]
	}
	return fmt.Sprintf("%d/%s", id, string(chars))
}

func readPump(client *Client) {
	for {
		packetType, packet, err := client.connection.ReadMessage()
		if err != nil {
			log.Println("Message read error:", err)
			break
		}
		if packetType == websocket.TextMessage {
			client.onPacket(string(packet))
		}
	}
}

func New(username, password string) *Client {
	client := Client{}

	client.Send = func(line string) {
		lines := make([]string, 1)
		lines[0] = line

		bytes, _ := json.Marshal(lines)
		client.connection.WriteMessage(websocket.TextMessage, bytes)
	}

	client.login = func(challstr Challstr) {
		var res *http.Response
		var err error

		hasPass := password != ""

		if hasPass {
			queryValues := url.Values{}
			queryValues.Add("act", "login")
			queryValues.Add("name", toId(username))
			queryValues.Add("pass", password)
			queryValues.Add("challengekeyid", challstr.challengekeyid)
			queryValues.Add("challstr", challstr.challstr)
			res, err = http.PostForm("https://play.pokemonshowdown.com/action.php", queryValues)
		} else {
			queryValues, _ := query.Values(struct {
				act            string
				userid         string
				challengekeyid string
				challstr       string
			}{
				act:            "getassertion",
				userid:         toId(username),
				challengekeyid: challstr.challengekeyid,
				challstr:       challstr.challstr,
			})
			res, err = http.Get(fmt.Sprintf("https://play.pokemonshowdown.com/action.php?%s", queryValues.Encode()))
		}

		if err != nil {
			log.Fatalf("Error logging in: %s", err)
		}

		bodyBytes, _ := io.ReadAll(res.Body)
		body := string(bodyBytes)

		if body == ";" {
			log.Fatalf("Username is registered but no password given.")
		}
		if len(body) < 50 {
			log.Fatalf("Failed to login: %s", body)
		}
		if strings.Contains(body, "heavy load") {
			log.Fatalf("The login server is under heavy load.")
		}

		var trnData string
		if hasPass {
			var parsed struct{ Assertion string }
			json.Unmarshal([]byte(body[1:]), &parsed)
			if strings.HasPrefix(parsed.Assertion, ";;") {
				log.Fatal(parsed.Assertion[2:])
			}
			trnData = parsed.Assertion
		} else {
			trnData = body
		}

		client.Send(fmt.Sprintf("|/trn %s,0,%s", username, trnData))
	}

	client.onLine = func(room string, line string, beforeJoin bool) {
		input := strings.SplitN(line, "|", 3)

		switch lineType := input[1]; lineType {
		case "challstr":
			args := strings.SplitN(input[2], "|", 2)
			client.login(Challstr{challengekeyid: args[0], challstr: args[1]})
		}
	}
	client.onBatch = func(batchMessage string) {
		if batchMessage == "" {
			return
		}
		room := "lobby"
		if !strings.Contains(batchMessage, "\n") {
			client.onLine(room, batchMessage, false)
			return
		}

		lines := strings.Split(batchMessage, "\n")
		beforeJoin := false
		for index, line := range lines {
			if index == 0 && strings.HasPrefix(line, ">") {
				setRoom := lines[0][1:]
				if setRoom != "" {
					room = setRoom
				}
			} else {
				if strings.HasPrefix(line, "|init|") {
					beforeJoin = true
				}
				client.onLine(room, line, beforeJoin)
			}
		}
	}
	client.onPacket = func(packet string) {
		if !strings.HasPrefix(packet, "a") {
			return
		}
		var batches []string
		json.Unmarshal([]byte(packet[1:]), &batches)
		for _, batch := range batches {
			client.onBatch(batch)
		}
	}

	client.Connect = func() {
		nonce := getNonce()
		websocketUrl := fmt.Sprintf("%s://%s%s/showdown/%s/websocket", "wss", "sim3.psim.us", "", nonce)
		log.Println(websocketUrl)
		connection, _, err := websocket.DefaultDialer.Dial(websocketUrl, nil)
		if err != nil {
			log.Fatalf("Could not connect to socket")
		}
		client.connection = connection

		go readPump(&client)
		log.Println("Connected to socket")

		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

		<-sigs // Wait for interrupt or terminate signal
		log.Println("Shutting down...")
		client.connection.Close()
	}

	return &client
}
