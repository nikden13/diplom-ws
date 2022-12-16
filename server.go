package main

import (
	"./onlineClients"
	"./waitClients"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"net/http"
	"strings"
	"sync"
	"time"
)

type MessageOutput struct {
	Type    string   `json:"type"`
	Text    string   `json:"text"`
	Clients []Client `json:"clients"`
}

type MessageInput struct {
	Type      string `json:"type"`
	Text      string `json:"text"`
	PublicKey string `json:"publicKey"`
	Address   string `json:"address"`
	ChainKey  string `json:"chainKey"`
	Sign      string `json:"sign"`
}

type Client struct {
	Address   string `json:"address"`
	PublicKey string `json:"publicKey"`
	Online    bool   `json:"online"`
	ChainKey  string `json:"chainKey"`
	Sign      string `json:"sign"`
}

const (
	CountForLog int    = 1
	AuthUrl     string = "http://localhost:8888/api/auth/check?token="
)

var (
	socketUpgrader   = websocket.Upgrader{}
	socketConnection *websocket.Conn
	ipV4V6           = map[string]string{
		"[::1": "127.0.0.1",
	}
	timeConnection    = make(map[string]int64)
	addressConnection = make(map[string]*websocket.Conn)
	activeDialogs     = make(map[string][]string)
)

func verifyToken(token string) bool {
	clientHttp := http.Client{
		Timeout: 20 * time.Second,
	}

	result, err := clientHttp.Get(AuthUrl + token)
	if err != nil {
		fmt.Println(err)
		return false
	}
	type Response struct {
		data    bool
		success bool
	}
	var response Response
	_ = json.NewDecoder(result.Body).Decode(&response)

	return response.data
}

func getAddressFromToken(token string) string {
	payload := strings.Split(token, ".")[1]
	payloadBytes, _ := base64.StdEncoding.DecodeString(payload)
	type Payload struct {
		Address string `json:"address"`
		Iss     string `json:"iss"`
		Iat     int    `json:"iat"`
		Exp     int    `json:"exp"`
	}
	var payloadStruct Payload
	_ = json.Unmarshal(payloadBytes, &payloadStruct)
	return payloadStruct.Address
}

func Connect(response http.ResponseWriter, request *http.Request) {
	socketUpgrader.CheckOrigin = func(request *http.Request) bool {
		return true
	}

	socketConnection, err := socketUpgrader.Upgrade(response, request, nil)
	if err != nil {
		fmt.Println(err)
	}

	token := request.URL.Query().Get("token")
	if !verifyToken(token) {
		SendMessageError(socketConnection, MessageOutput{Text: "401", Type: "error"})
		socketConnection.Close()
		return
	}

	clientPeer := GetClientPeer(request)
	address := getAddressFromToken(token)
	if address == "" {
		fmt.Println(clientPeer, address, "Неверный адрес")
		SendMessageError(socketConnection, MessageOutput{Text: "Неверный адрес"})
		socketConnection.Close()
		return
	}
	fmt.Println("Соединение установлено", clientPeer, address)
	setAllDataAboutClient(address, clientPeer, socketConnection)
	printOnlineInfo()

	go Handle(socketConnection, address)
}

func Handle(socketConnection *websocket.Conn, address string) {
	for {
		messageInput, err := ReadMessage(socketConnection)
		fmt.Println(messageInput)
		if err != nil {
			fmt.Println(err)
			break
		}

		wasError := false
		if messageInput.Type == "online" {
			fmt.Println(address + " запрашивает онлайн - " + messageInput.Address)
			_, ok := activeDialogs[address]
			if !ok {
				activeDialogs[address] = []string{}
			}
			activeDialogs[address] = append(activeDialogs[address], messageInput.Address)

			_, ok = addressConnection[messageInput.Address]
			online := false
			if ok {
				online = true
			}
			clients := []Client{{Address: messageInput.Address, Online: online}}
			SendMessageSuccess(socketConnection, MessageOutput{Clients: clients, Type: "online"})
		} else {
			fmt.Println(address + " отправил сообщение - " + messageInput.Address)
			if !handleSendMessage(socketConnection, messageInput, address) {
				wasError = false
			}
		}

		if wasError {
			break
		}
		fmt.Printf("Сообщение серверу: %s. Тип: %s. От: %s\n", messageInput.Text, messageInput.Type, address)
	}
	socketConnection.Close()
	fmt.Println("Соединение потеряно", address)
	sendMessageAboutCloseConnection(address)
	reset(address)
}

func sendMessageAboutCloseConnection(closeAddress string) {
	dialogs, ok := activeDialogs[closeAddress]
	if ok {
		for _, address := range dialogs {
			connection, ok := addressConnection[address]
			if ok {
				clients := []Client{{Address: closeAddress, Online: false}}
				SendMessageSuccess(connection, MessageOutput{Clients: clients, Type: "online"})
			}
		}
	}
}

func handleSendMessage(socketConnection *websocket.Conn, messageInput MessageInput, addressFrom string) bool {
	connectionTo, ok := addressConnection[messageInput.Address]
	if ok {
		clients := []Client{{Address: addressFrom, PublicKey: messageInput.PublicKey, Sign: messageInput.Sign, ChainKey: messageInput.ChainKey, Online: true}}
		SendMessageSuccess(connectionTo, MessageOutput{Text: messageInput.Text, Type: messageInput.Type, Clients: clients})
	} else {
		SendMessageError(socketConnection, MessageOutput{Text: "Пользователь не онлайн."})
	}

	return true
}

func setAllDataAboutClient(address string, clientPeer string, socketConnection *websocket.Conn) {
	timeConnection[address] = time.Now().Unix()
	onlineClients.SetOnline(address, clientPeer)
	addressConnection[address] = socketConnection
}

func printOnlineInfo() {
	if onlineClients.Count() == 1 || onlineClients.Count()%CountForLog == 0 {
		onlineClients.PrintClients()
	}
}

func reset(address string) {
	fmt.Println("Данные удалены", address)
	onlineClients.RemoveOnlineByAddress(address)
	waitClients.RemoveClient(address)
	delete(addressConnection, address)
	delete(timeConnection, address)
	delete(activeDialogs, address)
}

func SendMessageError(socketConnection *websocket.Conn, message MessageOutput) {
	message.Type = "error"
	sendMessage(socketConnection, message)
}

func SendMessageSuccess(socketConnection *websocket.Conn, message MessageOutput) {
	sendMessage(socketConnection, message)
}

func sendMessage(socketConnection *websocket.Conn, message MessageOutput) {
	messageJson, err := json.Marshal(message)
	if err != nil {
		fmt.Println(err)
	}
	type Message struct {
		conn *websocket.Conn
		mu   sync.Mutex
	}
	messageNew := Message{conn: socketConnection}
	messageNew.mu.Lock()
	defer messageNew.mu.Unlock()

	err = messageNew.conn.WriteMessage(websocket.TextMessage, messageJson)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("Отправлено клиенту: " + string(messageJson))
}

func ReadMessage(socketConnection *websocket.Conn) (MessageInput, error) {
	var message MessageInput
	err := socketConnection.ReadJSON(&message)
	return message, err
}

func GetClientPeer(request *http.Request) string {
	IpAddress := request.Header.Get("X-Real-IP")
	if IpAddress == "" {
		IpAddress = request.Header.Get("X-Forwarder-For")
	}

	if IpAddress == "" {
		IpAddress = request.RemoteAddr
	}

	addressPort := strings.Split(IpAddress, "]:")
	addressV4, ok := ipV4V6[addressPort[0]]
	if ok {
		addressPort[0] = addressV4
	}
	IpAddress = addressPort[0]
	if len(addressPort) > 1 {
		IpAddress += ":" + addressPort[1]
	}

	return IpAddress
}

func main() {
	router := mux.NewRouter()

	router.HandleFunc("/socket", Connect)
	fmt.Println("Start server")
	err := http.ListenAndServe(":18000", router)
	if err != nil {
		return
	}
}
