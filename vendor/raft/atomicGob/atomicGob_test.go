package atomicGob

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"strconv"
	"testing"
	"time"
)

var theGob, theGob2 *GobFile
var sendMessage []MockMessage
var pwd string

/******* INITIALIZE ***************************/
func Test_Build(t *testing.T) {
	pwd, _ = os.Getwd()
	sendMessage = append(sendMessage, MockMessage{Msg: "0"})
	theGob = NewGobFile(pwd+"/MockMessage.txt", sendMessage)
	theGob2 = NewGobFile(pwd+"/MockMessage2.txt", sendMessage)
}

/******* BENCHMARKS ********************************/

//Benchmark tests the capabilities of potential bottleneck fuctions
func Benchmark_Write(b *testing.B) {
	for n := 0; n < b.N; n++ {
		theGob2.Write()
	}
}

/******* TESTS **********************************/
func Test_Write(t *testing.T) {
	for i := 1; i < 10; i++ {
		sendMessage = append(sendMessage, MockMessage{Msg: strconv.Itoa(i)})

		theGob.Lock()
		theGob.Data = sendMessage
		theGob.Unlock()
		theGob2.Lock()
		theGob2.Data = sendMessage
		theGob2.Unlock()
		theGob.Write()
		theGob2.Write()
	}
	theGob.Wait() // wait until the writes are finished.
	theGob2.Wait()
	if bytes.Compare(theGob.GetMD5(), theGob2.GetMD5()) != 0 {
		t.Error("byte values unequal")
	}
}

func Test_Read(t *testing.T) {
	var GI, GI2, GI3 interface{}
	theGob.Read(&GI)
	theGob.Wait()
	G := GI.([]MockMessage)
	for i, x := range G {
		if x.Msg != strconv.Itoa(i) {
			t.Error("Initial Gob Not in Seqence")
		}
	}
	fmt.Println(G[0].Msg)
	//delete the main file, and try to read. read should still succeed
	os.Remove(pwd + "/MockMessage.txt")
	theGob.file.Close()
	theGob.Read(&GI2)
	theGob.Wait()
	G2 := GI2.([]MockMessage)
	fmt.Println(G2)
	//test full failure
	os.Remove(pwd + "/MockMessage.txt.bak")
	os.Remove(pwd + "/MockMessage.txt")
	theGob.file.Close()
	theGob.fileBak.Close()
	err := theGob.Read(&GI3)
	theGob.Wait()
	if err != nil {
		t.Error("Gob Reader did not produce proper error")
	}
}

type MockMessage struct {
	Msg string
}

func Test_Gob(t *testing.T) {
	client, server := net.Pipe()
	sendMessage := MockMessage{"hello"}
	go SendGob(sendMessage, server)
	var recieveMessage MockMessage
	recieveMessage2, ok := RecieveGob(client, sendMessage).(MockMessage)
	if !ok {
		t.Error("Gob Decode Type Assertion Failure")
	}
	recieveMessage = recieveMessage2

	time.Sleep(time.Millisecond * 100)
	if recieveMessage.Msg != sendMessage.Msg {
		t.Error("gob decode not equal")
	}
	client.Close()
	server.Close()
}
