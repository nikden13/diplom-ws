package waitClients

import "../onlineClients"

var (
	waitClients = make(map[string][]string)
)

func GetClientsByNeedAddress(address string) map[string]string {
	clients := make(map[string]string)
	for addr, needClientsAddresses := range waitClients {
		for _, needClientsAddress := range needClientsAddresses {
			if needClientsAddress == address {
				peer, ok := onlineClients.GetPeerByAddress(addr)
				if ok {
					clients[addr] = peer
				}
			}
		}
	}

	return clients
}

func SetClient(address string, addressNeed string) {
	waitClients[address] = append(waitClients[address], addressNeed)
}

func RemoveClient(address string) {
	delete(waitClients, address)
}

func RemoveWaitClient(address string) {
	for addr, clientsAddresses := range waitClients {
		var clients []string
		for _, clientsAddress := range clientsAddresses {
			if clientsAddress != address {
				clients = append(clients, clientsAddress)
			}
		}
		waitClients[addr] = clients
	}
}
