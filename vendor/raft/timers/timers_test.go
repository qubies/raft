package timers

import (
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"testing"
	"time"
)

var expire chan bool

//run 1000 counts of random, looking for tests out of bounds.
//function verified by changing the start point.
//testing of random gen not perfect, could occasionally fail erroneously.
func Test_Randoms(t *testing.T) {

	var lower5, upper5 bool //notes if a number has occured in the upper and lower threshold
	// the amount by which an amount can deviate from the max or min within tests.
	//Smaller thresholds indicate greater odds of failure
	const threshold int64 = 1
	//test error on rand gen
	errorTime := getLength(io.LimitReader(os.Stdin, 0))
	if errorTime != 0 {
		t.Error("error handler failed.")
	}
	BrokenTimer := ElectionTimer{time.NewTimer(time.Duration(errorTime) * time.Millisecond)}
	time.Sleep(time.Nanosecond) //sleep allows the goRoutine that sets timer to finish setting up.
	if !BrokenTimer.Expired() {
		t.Error("Rand Error did not cause timer to immediately expire.")
	}
	for i := 0; i < 10000; i++ {
		thisTime := getLength(rand.Reader)
		if !inRange(thisTime, lowerBounds, upperBounds) {
			t.Error("Election Timer Randoms Generated time out of bounds.")
		}
		if !upper5 && inRange(thisTime, upperBounds-threshold, upperBounds) {
			fmt.Println("Upper timer bounds Passed. set @", thisTime)
			upper5 = true
		}
		if !lower5 && inRange(thisTime, lowerBounds, lowerBounds+threshold) {
			fmt.Println("Lower timer bounds Passed. set @", thisTime)
			lower5 = true
		}
	}
	if !lower5 {
		t.Error("no randoms found in lower threshold. try re-running test")
	}
	if !upper5 {
		t.Error("no randoms found in upper threshold. Try re-running test")
	}
}

func inRange(test, lowerBounds, upperBounds int64) bool {
	return (test <= upperBounds && test >= lowerBounds)
}

func Test_Timer(t *testing.T) {
	thisTimer := getTimer()
	<-thisTimer.C
	fmt.Println("Timer Limit Initially Reached!")
}

func Test_ElectionTimer(t *testing.T) {
	testingTimer := NewElectionTimer()
	if testingTimer.Expired() {
		t.Error("Timer Expired Prematurely")
	}
	time.Sleep(time.Duration(lowerBounds-1) * time.Millisecond)
	if testingTimer.Expired() {
		t.Error("Timer Expired Prematurely")
	}
	testingTimer.Renew()
	time.Sleep(time.Duration(lowerBounds-1) * time.Millisecond)
	if testingTimer.Expired() {
		t.Error("Timer Expired Prematurely After Renew... RENEW FAILED")
	}
	time.Sleep(time.Duration(upperBounds-lowerBounds+2) * time.Millisecond)
	if !testingTimer.Expired() {
		t.Error("Timer Failed to Expire")
	}
}

func Test_heartrate(t *testing.T) {
	expire = make(chan bool)
	NewHeartrate(printExpire) //calls print expire after the timer expires
	time.Sleep(time.Duration(heartrate-1) * time.Millisecond)
	select {
	case <-expire:
		t.Error("HR Timer Expired Prematurely")
	default:
		time.Sleep(time.Millisecond * 10)

	}
	select {
	case <-expire:
		break
	default:
		t.Error("HR timer did not expire!")
	}
}

func printExpire() {
	expire <- true
	close(expire)
	fmt.Println("HeartRate Expired!")
}
