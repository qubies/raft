package connectionManager

import (
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"net"
	"raft/lockables"
	"raft/terr"
)

//Message is the heart of the RPC system
//Messages are passed around the raft, all pertinent state information is included.
type Message struct {
	SenderID      string
	SenderAddress string
	Type          int         // the type of message
	Payload       interface{} // the payload to drop
}

//message types
const (
	Print = iota
	Append
	Appended
	AddMembers
	VoteRequest
	Vote
	Commit
	Connect
	Ack
	Hello
	Demote
	RA
	Guess
)

//port and address consts
const (
	messageBuffer = 50000 // the number of messages to allow in buffer. too many may cause late action.
	verb          = 0
)

var outputTo io.Writer

type Client struct {
	Address string
	ID      string
}

type ConnectionManager struct {
	ID               string
	Address          string
	Clients          *lockables.StrMap
	Messages         chan Message
	broadcast        chan Message
	register         chan *Client
	stop             chan bool
	listener         net.Listener
	messagesSent     lockables.Counter
	messagesRecieved lockables.Counter
}

func New(ID string, Address string, OutputTo io.Writer) *ConnectionManager {
	outputTo = OutputTo
	C := new(ConnectionManager)
	C.Clients = lockables.NewStrMap()
	C.Address = Address
	C.broadcast = make(chan Message, messageBuffer)
	C.Messages = make(chan Message, messageBuffer)
	C.register = make(chan *Client)
	C.stop = make(chan bool)
	C.ID = ID
	l, err := net.Listen("tcp", Address)
	terr.BreakError("TCP Listener", err)
	C.listener = l
	go C.mainLoop()
	go C.recieve()
	return C
}

func (C *ConnectionManager) mainLoop() {
	for {
		select {
		case newClient := <-C.register:
			C.Clients.Add(newClient.ID, newClient.Address)
		case msg := <-C.broadcast:
			for _, address := range C.Clients.Get() {
				C.PM(address, msg)
			}
		case <-C.stop:
			C.listener.Close()
			return
		}
	}
}

func (C *ConnectionManager) recieve() {
	for {
		select {
		case <-C.stop:
			return
		default:
			conn, err := C.listener.Accept()
			if err != nil {
				terr.VerbPrint(outputTo, 1, verb, "Listener Accept Error", err)
				continue
			}
			dec := gob.NewDecoder(conn)
			msg := new(Message)
			err = dec.Decode(&msg)
			conn.Close()
			if err != nil {
				terr.VerbPrint(outputTo, 1, verb, "Message Decode Error")
			}
			C.messagesRecieved.Inc()
			if msg.Type == Connect {
				//Connect is the inital message, and expects an ack in return
				C.PM(msg.SenderAddress, C.BuildMessage(Ack, nil))
				C.AddClient(msg.SenderID, msg.SenderAddress)
			} else if msg.Type == Ack {
				//an ack means to just add this client to your list.
				C.AddClient(msg.SenderID, msg.SenderAddress)
			}
			C.Messages <- *msg
		}
	}
}

func (C *ConnectionManager) Stop() {
	close(C.stop)
}
func (C *ConnectionManager) Broadcast(msg Message) {
	C.broadcast <- msg
}

func (C *ConnectionManager) PM(Address string, msg Message) {
	go func() {
		conn, err := net.Dial("tcp", Address)
		if err != nil {
			terr.VerbPrint(outputTo, 2, verb, "Dial error from", C.Address, "to address", Address)
			return
		}
		defer conn.Close()
		enc := gob.NewEncoder(conn)
		err = enc.Encode(msg)
		if err != nil {
			terr.VerbPrint(outputTo, 3, verb, "Send Encode Error from PM")
		}
		C.messagesSent.Inc()
	}()
}

func (C *ConnectionManager) Connect(Address string) {
	if Address == "" {
		return
	}
	conn, err := net.Dial("tcp", Address)
	if err != nil {
		terr.VerbPrint(outputTo, 4, verb, "Error contacting address", Address)
		return
	}
	defer conn.Close()
	enc := gob.NewEncoder(conn)
	err = enc.Encode(C.BuildMessage(Connect, nil))
	C.messagesSent.Inc()
	if err != nil {
		terr.VerbPrint(outputTo, 3, verb, "Error encoding message to", Address)
	}
}

func (C *ConnectionManager) AddClient(ID, Address string) {
	if ID == "" || Address == "" {
		terr.VerbPrint(outputTo, 1, verb, "Nil Client addition attempted")
	}
	NC := new(Client)
	NC.Address = Address
	NC.ID = ID
	C.register <- NC
}

func (C *ConnectionManager) BuildMessage(msgType int, payload interface{}) Message {
	if C.ID == "" {
		terr.BreakError("Message Builder", errors.New("Sender ID Empty"))
	}
	msg := new(Message)
	msg.SenderAddress = C.Address
	msg.SenderID = C.ID
	msg.Type = msgType
	msg.Payload = payload
	return *msg
}
func (manager *ConnectionManager) RecSent() (int, int) {
	return manager.messagesRecieved.Get(), manager.messagesSent.Get()
}

// STRINGERS
func (msg Message) String() string {
	ans := "Message From: " + msg.SenderID[:6] + " @ " + msg.SenderAddress
	ans += " --- Type: "
	switch msg.Type {
	case Print:
		ans += "Print"
	case Append:
		ans += "Append"
	case Connect:
		ans += "Connect"
	case AddMembers:
		ans += "AddMembers"
	case VoteRequest:
		ans += "VoteRequest"
	case Vote:
		ans += "Vote"
	case Commit:
		ans += "Commit"
	case Hello:
		ans += "Hello"
	case Ack:
		ans += "Ack"
	}
	if msg.Payload != nil {
		ans += " +++ Payload: " + fmt.Sprint(msg.Payload)
	}
	return ans
}
