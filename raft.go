package main

import (
	"bytes"
	"flag"
	"fmt"
	"raft/command"
	"raft/member"
	"raft/timers"
	"runtime"
	"time"
)

var threads = runtime.NumCPU() * 2 // gets the number of CPUS from the host machine.
var expireTime time.Time
var quickTime = 0 //the number of minutes for a quicktime expiry
const verb = 0

func main() {
	/**************** GENERAL STARTUP CONFIG ************************************/
	runtime.GOMAXPROCS(threads)
	stop := make(chan bool)
	pause := make(chan bool)
	resume := make(chan bool)
	myOutput := new(bytes.Buffer) //this is not threadsafe, but it doesnt really matter.
	expire := flag.String("e", "", "set a date and time to exipre the raft member format is: mm/dd/yyyy-hh:mm:am")
	quick := flag.Int("q", 0, "quickTime interval set to "+string(quickTime)+" seconds")
	contactIP := flag.String("i", "", "Ip:port to contact for raft membership")
	port := flag.String("p", "", "port for connection listener")
	ID := flag.String("d", "", "AutoID")
	chaos := flag.Bool("c", false, "Mini Chaos Monkey, randomly pauses the raft")

	flag.Parse()

	//Check for an expiry time, and set timed expiry takes precedence over quick set
	if *expire != "" {
		setExpire(*expire)
	} else if *quick != 0 {
		quickTime = *quick
		quickExpire()
	}
	IP := member.GetIP()
	Port := "33333"
	if *contactIP == "" {
		fmt.Println("you must enter a contact address for the member to join, please enter an ip:port using the -i flag")
		return
	}
	if *port != "" {
		Port = *port
	}
	var me *member.RaftMember
	if *ID != "" {
		me = member.New(IP+":"+Port, *contactIP, *ID, myOutput)
	} else {
		me = member.New(IP+":"+Port, *contactIP, "", myOutput)
	}
	go command.Begin(me, myOutput, stop)         //startup the commander to read choices
	fmt.Println("Raft Built at:", me.CM.Address) //print out the details
	//make me!!!

	go checkExpire(stop)
	if *chaos {
		go runMonkeyRun(pause, resume)
	}
	for {
		select {
		case <-stop:
			goto end
		case <-pause:
			me.Suspend()
		case <-resume:
			me.Resume()
		}
	}
end:
	if expireTime.Before(time.Now()) && !expireTime.IsZero() {
		fmt.Println("Raft Session Expired from GetOpt timer")
	}
}

//quickExpire is a shortcut to make a raft last for the nearest round time of the quickTime constant
//Currently this will set the raft to expire at 5 minute rounds, ie 5:45 or 5:50 or 5:55 etc.
func quickExpire() {
	mins := quickTime - time.Now().Minute()%quickTime
	expireTime = time.Now().Add(time.Minute * time.Duration(mins)).Truncate(time.Minute)
	fmt.Printf("Raft will stop after %s\n", expireTime.Local())
}

func runMonkeyRun(pause, resume chan bool) {
	for {
		shouldPause := timers.GetRandomSecondsTimer(30, 120)
		<-shouldPause.C
		pause <- true
		shouldResume := timers.GetRandomSecondsTimer(10, 25)
		<-shouldResume.C
		resume <- true
	}
}
func setExpire(dateTime string) error {
	const longForm = "Jan/02/2006-3:04pm"
	var err error
	t := time.Now().Local() //this is needed to get the current location timezone
	expireTime, err = time.ParseInLocation(longForm, dateTime, t.Location())
	fmt.Printf("Raft will stop after %s\n", expireTime.Local())

	return err
}

func checkExpire(meKill chan bool) {
	for expireTime.IsZero() || expireTime.After(time.Now()) {
		time.Sleep(time.Second) //avoids wasting cycles (faster sleeps more responsive to timers.)
	}
	fmt.Println("Closing!")
	close(meKill) // close the kill channel
}
