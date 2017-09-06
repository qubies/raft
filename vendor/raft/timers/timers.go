package timers

/*
The Election Timer is here. lowerBounds should be set to be greater than
the approximate latency between servers. If the timeout is less, then the server will always transition to
candidate status, while too long will hurt response times. Values too close together will cause excessive split
votes.
*/

import (
	"crypto/rand"
	"fmt"
	"io"
	"math/big"
	"time"
)

const heartrate int64 = 100    //the repetition rate of a heartbeat
const lowerBounds int64 = 600  //the lower limit of wait time in milliseconds
const upperBounds int64 = 1200 //the upper limit of wait time in milliseconds

//ElectionTimer represents the timer before a follower will transition to candidacy
type ElectionTimer struct {
	*time.Timer
}

//HRTimer holds the heartbeat repeat timer
type HRTimer struct {
	*time.Timer
}

//NewElectionTimer returns an ElectionTimer pointer
func NewElectionTimer() *ElectionTimer {
	return &ElectionTimer{getTimer()}
}

//NewHeartrate returns a timer to repeat heartbeats
//f is called on expiry
func NewHeartrate(f func()) *HRTimer {
	return &HRTimer{time.AfterFunc((time.Duration(heartrate) * time.Millisecond), f)}
}

//Renew resets an existing election timer
func (E *ElectionTimer) Renew() {
	*E = ElectionTimer{getTimer()}
}

//Expired checks if an election timer has expired
func (E *ElectionTimer) Expired() bool {
	select {
	case <-E.C:
		return true
	default:
		return false
	}
}

func getTimer() *time.Timer {
	return time.NewTimer(time.Duration(getLength(rand.Reader)) * time.Millisecond)
}

//GetRandomSecondsTimer returns a timer of random length between the lower and upper bounds
func GetRandomSecondsTimer(low, high int64) *time.Timer {
	return time.NewTimer(time.Duration(getVarLen(low, high, rand.Reader)) * time.Second)
}

func getVarLen(low, high int64, Reader io.Reader) int64 {
	start := new(big.Int)
	start.SetInt64(high - low + 1)
	temp, err := rand.Int(Reader, start)
	if err != nil {
		fmt.Println("error in random generation:", err)
		return 0 //returns a 0 length, intended to force retry
	}
	return (temp.Int64() + low)
}

/*
generates a cryptographically seceure random number for the timer.
*/
func getLength(Reader io.Reader) int64 {
	start := new(big.Int)
	start.SetInt64(upperBounds - lowerBounds + 1)
	temp, err := rand.Int(Reader, start)
	if err != nil {
		fmt.Println("error in random generation:", err)
		return 0 //returns a 0 length, intended to force retry
	}
	return (temp.Int64() + lowerBounds)
}
