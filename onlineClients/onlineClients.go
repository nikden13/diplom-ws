package onlineClients

import (
	"encoding/json"
	"fmt"
	"strings"
)

var (
	onlineClients = make(map[string]string)
)

func GetClients() map[string]string {
	return onlineClients
}

func Count() int {
	return len(onlineClients)
}

func PrintClients() {
	clients := GetClients()

	clientsJson, err := json.Marshal(clients)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("-------------------------------------------------------------------------")
	fmt.Printf("Онлайн: %d\n", Count())
	for _, client := range strings.Split(string(clientsJson), ",") {
		fmt.Println(client)
	}
	fmt.Println("-------------------------------------------------------------------------")
}

func SetOnline(address string, peer string) {
	onlineClients[address] = peer
}

func RemoveOnlineByPeer(peer string) {
	for address, mapPeer := range onlineClients {
		if mapPeer == peer {
			delete(onlineClients, address)
		}
	}
}

func RemoveOnlineByAddress(address string) {
	delete(onlineClients, address)
}

func GetPeerByAddress(address string) (string, bool) {
	client, ok := onlineClients[address]

	return client, ok
}
