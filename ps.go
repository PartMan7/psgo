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

type Message struct {
	Room       string
	By         string
	Content    string
	BeforeJoin bool
}
type DirectMessage struct {
	From       string
	To         string
	Content    string
	BeforeJoin bool
}

type Client struct {
	Connect func()

	Send     func(line string)
	SendRoom func(room string, line string)
	SendUser func(user string, line string)

	On        func(room string, event string, line string, beforeJoin bool)
	OnMessage func(message Message)
	OnPM      func(message DirectMessage)
	OnConnect func()

	connection *websocket.Conn
	login      func(challstr Challstr)
	onLine     func(room string, line string, beforeJoin bool)
	onBatch    func(batchMessage string)
	onPacket   func(packet string)
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

func New(username, password string, rooms []string) *Client {
	client := Client{}

	client.On = func(room string, event string, line string, beforeJoin bool) {}
	client.OnConnect = func() {}
	client.OnMessage = func(message Message) {}
	client.OnPM = func(message DirectMessage) {}

	client.Send = func(line string) {
		lines := make([]string, 1)
		lines[0] = line

		bytes, _ := json.Marshal(lines)
		client.connection.WriteMessage(websocket.TextMessage, bytes)
	}
	client.SendRoom = func(room string, line string) {
		client.Send(fmt.Sprintf("%s|%s", room, line))
	}
	client.SendUser = func(user string, line string) {
		client.Send(fmt.Sprintf("|/pm %s,%s", user, line))
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

		lineType := ""
		if len(input) >= 2 {
			lineType = input[1]
		}
		ctx := ""
		if len(input) == 3 {
			ctx = input[2]
		}
		client.On(room, lineType, ctx, beforeJoin)

		switch lineType {
		case "challstr":
			args := strings.SplitN(ctx, "|", 2)
			client.login(Challstr{challengekeyid: args[0], challstr: args[1]})
		case "updateuser":
			name := ctx[1:]
			if !strings.HasPrefix(name, "Guest") {
				client.OnConnect()
				for _, joinRoom := range rooms {
					client.Send(fmt.Sprintf("|/join %s", joinRoom))
				}
			}
		case "c:", "c", "chat":
			args := strings.SplitN(ctx, "|", 3)
			client.OnMessage(Message{Room: room, By: args[1], Content: args[2], BeforeJoin: beforeJoin})
		case "pm":
			log.Println(ctx)
			args := strings.SplitN(ctx, "|", 3)
			client.OnPM(DirectMessage{From: args[0], To: args[1], Content: args[2], BeforeJoin: beforeJoin})
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
		connection, _, err := websocket.DefaultDialer.Dial(websocketUrl, nil)
		if err != nil {
			log.Fatalf("Could not connect to socket")
		}
		client.connection = connection

		go readPump(&client)

		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

		<-sigs // Wait for interrupt or terminate signal
		log.Println("Shutting down...")
		client.connection.Close()
	}

	return &client
}
