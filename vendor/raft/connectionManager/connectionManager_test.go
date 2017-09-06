package connectionManager

import (
	"fmt"
	"os"
	"raft/terr"
	"strconv"
	"testing"
	"time"
)

const (
	numMessages = 5
	breakAt     = 1000
)

func Test_run(t *testing.T) {
	//build the servers
	s1 := New("S1ERVER", "localhost:30000", os.Stdout)
	go s1.handleMessages()
	s2 := New("S2ERVER", "localhost:30001", os.Stdout)
	go s2.handleMessages()
	s3 := New("S3ERVER", "localhost:30002", os.Stdout)
	go s3.handleMessages()
	s4 := New("S4ERVER", "localhost:30003", os.Stdout)
	go s4.handleMessages()
	s5 := New("S5ERVER", "localhost:30004", os.Stdout)
	go s5.handleMessages()
	//make connections
	s1.Connect(s2.Address)
	s1.Connect(s3.Address)
	s1.Connect(s4.Address)
	s1.Connect(s5.Address)
	s2.Connect(s3.Address)
	s2.Connect(s4.Address)
	s2.Connect(s5.Address)
	s3.Connect(s4.Address)
	s3.Connect(s5.Address)
	s4.Connect(s5.Address)

	time.Sleep(time.Millisecond * 100)
	//send a bunch of messages
	for i := 0; i < numMessages; i++ {
		s1MSG := s1.BuildMessage(Print, strconv.Itoa(i))
		s2MSG := s2.BuildMessage(Print, strconv.Itoa(i))
		s3MSG := s3.BuildMessage(Print, strconv.Itoa(i))
		s4MSG := s4.BuildMessage(Print, strconv.Itoa(i))
		s5MSG := s5.BuildMessage(Print, strconv.Itoa(i))

		if i == 1 {
		}
		//broadcast sends all the messages to all other servers
		s1.Broadcast(s1MSG)
		s2.Broadcast(s2MSG)
		s3.Broadcast(s3MSG)
		s4.Broadcast(s4MSG)
		s5.Broadcast(s5MSG)
	}
	fmt.Println(s5.Address)
	s1.PM(s5.Address, s1.BuildMessage(Print, "999999"))
	//on verb level 5, the printing takes longer than this, so make sure if you need to see, that you extend this to around 5 seconds.
	time.Sleep(time.Millisecond * 2000) // allow the messages to arrive and get counted

	//collect the results
	S1MS := s1.messagesSent.Get()
	S1MR := s1.messagesRecieved.Get()
	S2MS := s2.messagesSent.Get()
	S2MR := s2.messagesRecieved.Get()
	S3MS := s3.messagesSent.Get()
	S3MR := s3.messagesRecieved.Get()
	S4MS := s4.messagesSent.Get()
	S4MR := s4.messagesRecieved.Get()
	S5MS := s5.messagesSent.Get()
	S5MR := s5.messagesRecieved.Get()

	//print the results
	terr.VerbPrint(outputTo, 1, verb, s1.ID, "Sent:", S1MS)
	terr.VerbPrint(outputTo, 1, verb, s1.ID, "Recieved:", S1MR)
	terr.VerbPrint(outputTo, 1, verb, s2.ID, "Sent:", S2MS)
	terr.VerbPrint(outputTo, 1, verb, s2.ID, "Recieved:", S2MR)
	terr.VerbPrint(outputTo, 1, verb, s3.ID, "Sent:", S3MS)
	terr.VerbPrint(outputTo, 1, verb, s3.ID, "Recieved:", S3MR)
	terr.VerbPrint(outputTo, 1, verb, s4.ID, "Sent:", S4MS)
	terr.VerbPrint(outputTo, 1, verb, s4.ID, "Recieved:", S4MR)
	terr.VerbPrint(outputTo, 1, verb, s5.ID, "Sent:", S5MS)
	terr.VerbPrint(outputTo, 1, verb, s5.ID, "Recieved:", S5MR)

	//check for dropped messages
	//the number of messages + 1 for connect times the number of servers
	// S1 should have an extra send for the PM, and S5 an extra recieve
	if S1MS != (numMessages+1)*4+1 {
		t.Error("Messages Not SENT from Server 1")
	}
	if S1MR != (numMessages+1)*4 {
		t.Error("Messages Missed on Server 1")
	}
	if S2MS != (numMessages+1)*4 {
		t.Error("Messages Not SENT from Server 2")
	}
	if S2MR != (numMessages+1)*4 {
		t.Error("Messages Missed on Server 2")
	}
	if S3MS != (numMessages+1)*4 {
		t.Error("Messages Not SENT from Server 3")
	}
	if S3MR != (numMessages+1)*4 {
		t.Error("Messages Missed on Server 3")
	}
	if S4MS != (numMessages+1)*4 {
		t.Error("Messages Not SENT from Server 4")
	}
	if S4MR != (numMessages+1)*4 {
		t.Error("Messages Missed on Server 4")
	}
	if S5MS != (numMessages+1)*4 {
		t.Error("Messages Not SENT from Server 5")
	}
	if S5MR != (numMessages+1)*4+1 {
		t.Error("Messages Missed on Server 5")
	}

}

//reads out the messages from the incomming buffer.
func (M *ConnectionManager) handleMessages() {
	for {
		select {
		case msg := <-M.Messages:
			terr.VerbPrint(outputTo, 5, verb, M.ID, "RECIEVED:", msg)
		}
	}
}
