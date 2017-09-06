package member

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"raft/atomicGob"
	CM "raft/connectionManager"
	"raft/lockables"
	"raft/stateMachine"
	"raft/terr"
	"raft/timers"
	"sort"
	"strconv"
	"strings"
	"time"
)

//port and address consts
const (
	verb            int = 3
	hrInterval          = 50 //number of milliseconds between heartbeats
	contentionLevel     = 35
)

//states for raft Members
const (
	Follower  = iota //0
	Candidate        //1
	Leader           //2
)

//RaftMember is a server intended to be a member of the raft.
//Exported Members are passed around by gob functions and written to files.
type RaftMember struct {
	Persistent
	volatile
}

type Persistent struct {
	ElectionState
	ID string
	Raft
	ClientList  map[string]bool
	ClientCount int
	Target      int
}
type volatile struct {
	Machine        *stateMachine.StateMachine //?
	needStatus     chan bool
	needWrite      bool
	giveStatus     chan StatusBall
	needLog        chan bool
	giveLog        chan string
	appendRequests chan string
	needScoreBoard chan bool
	GiveScoreboard chan map[string]int
	outputTo       io.Writer
	alreadyGuessed bool
	CM             *CM.ConnectionManager
}
type AddMemberPayload struct {
	term int
	book map[string]string
}

type Raft struct {
	Log         []LogEntry
	commitIndex int
	LastApplied int
	nextIndex   map[string]int // the status of the member's commits
	matchIndex  map[string]int // the status of the member's commits
	suspend     chan int
}

type strInt struct { // for passing a string and an int in interface{}
	tString string
	tInt    int
}

type VoteRecord struct {
	Votes    map[string]bool
	VotedFor string
}

type RequestVote struct {
	Term         int
	ID           string
	LastLogIndex int
	LastLogTerm  int
	Reply
}

type Reply struct {
	Result      bool
	Term        int
	CommitIndex int
	LastApplied int
	ID          string
}

type Guess struct {
	ID    string
	Guess int
}

//the TermState keeps pertinenet information regarding the current state of the member
type ElectionState struct {
	VoteLog []VoteRecord // the amount of votes this member recieved in the indexed term
	Term    int          // the term of the raft
	Status  int          // leader candidate or follower
	leader  string
	et      *timers.ElectionTimer
	hrt     *time.Timer
}

type LogEntry struct {
	Term  int
	Entry string
}

type AppendEntry struct {
	Term         int
	PrevLogIndex int
	PrevLogTerm  int
	Entries      []LogEntry
	CommitIndex  int
	LastApplied  int
	ID           string
	Reply
}

type StatusBall struct {
	Term      int
	Leader    string
	Members   []string
	LogLength int
	Address   string
	ID        string
	Status    int
}

func init() {
	//need to register all the structs with the gob
	gob.Register(make(map[string]string))
	gob.Register(lockables.NewStrMap())
	gob.Register(*new(RequestVote))
	gob.Register(*new(AppendEntry))
	gob.Register(new(Guess))
}

//New builds a new raft member, and starts its internal processes.
//It returns a Pointer to its member for controls
func New(IP, firstContact, ID string, OutputTo io.Writer) *RaftMember {
	//try to get the existing data from file
	os.Mkdir("/tmp/raft/", 0777)
	R, PStore, success := readPersist()
	//build the stateMachine
	pwd := "/tmp"
	os.Mkdir(pwd+"/machines", 0777) //make the machines directory

	R.hrt = new(time.Timer) //initialize our timer.
	R.giveStatus = make(chan StatusBall)
	R.outputTo = OutputTo
	R.needStatus = make(chan bool)
	R.giveLog = make(chan string)
	R.needLog = make(chan bool)
	R.nextIndex = make(map[string]int)
	R.matchIndex = make(map[string]int)
	R.appendRequests = make(chan string, 100)
	R.needScoreBoard = make(chan bool)
	R.suspend = make(chan int)
	R.GiveScoreboard = make(chan map[string]int)
	if !success {
		//load failed or files dont exist
		fmt.Println("Creating New Member")
		tid, err := makeID()
		R.VoteLog = append(R.VoteLog, VoteRecord{Votes: make(map[string]bool), VotedFor: "NIL"}) //bootstrap the vote record for the 0 term
		terr.BreakError("ID Creation", err)
		R.ID = tid
		R.Log = append(R.Log, LogEntry{Term: 0, Entry: "init"}) // make the first entry an initialize for counting purposes.
		R.Target = 1
	}
	R.Machine = stateMachine.CreateStateMachine(pwd+"/machines/"+R.ID[:6]+".sm", contentionLevel) //build or recreate the state machine.
	terr.VerbPrint(R.outputTo, 1, verb, "Target Set to:", R.Target)
	R.Machine.SetTarget(R.Target) //restores a target after a reboot
	if ID != "" {
		R.ID = ID
	}

	R.CM = CM.New(R.ID, IP, R.outputTo)
	if firstContact != "" {
		terr.VerbPrint(R.outputTo, 3, verb, "connecting to", firstContact)
		R.Connect(firstContact)
	}
	R.writePersist(PStore)
	R.et = timers.NewElectionTimer()
	R.run(PStore) //spawns a go routine to handle incomming messages
	return R
}

func (R *RaftMember) reply(msg CM.Message, response int, payload interface{}) {
	Nmsg := R.CM.BuildMessage(response, payload)
	R.CM.PM(msg.SenderAddress, Nmsg)
}

func (L *LogEntry) compare(comp *LogEntry) bool {
	if L.Entry == comp.Entry && L.Term == comp.Term {
		return true
	}
	return false
}

func (R *RaftMember) handleCommit(commit string) {
	args := strings.Split(commit, ",") //look at the arguments
	if len(args) < 1 {
		terr.VerbPrint(R.outputTo, 1, verb, "No arguments converted for handleCommit")
		return
	}
	switch args[0] {
	//the first argument is the command type
	case "PointScored":
		if len(args) != 4 {
			terr.VerbPrint(R.outputTo, 1, verb, "improper args length for PointScored")
			return
		}
		guess, err := strconv.Atoi(args[2])
		if err != nil {
			terr.VerbPrint(R.outputTo, 1, verb, "Unable to convert guess to integer")
		}
		err = R.Machine.CommitGuess(guess, args[1])
		if err != nil {
			//this should not happen
			terr.VerbPrint(R.outputTo, 1, verb, "Unsuccessful commit inside PointScored")
		} else {
			terr.VerbPrint(R.outputTo, 3, verb, args[1][:6], "Scored on number:", args[2])
		}
		target, err := strconv.Atoi(args[3])
		if err != nil {
			terr.VerbPrint(R.outputTo, 1, verb, "Unable to convert target to integer")
		}
		R.Target = target
		R.Machine.SetTarget(target)
		R.alreadyGuessed = false

	default:
		terr.VerbPrint(R.outputTo, 3, verb, R.ID[:6], "Committing", commit)
	}
}
func (R *RaftMember) Suspend() {
	terr.VerbPrint(R.outputTo, 0, verb, R.ID[:6], "SUSPENDED")
	R.suspend <- 1
}
func (R *RaftMember) Resume() {
	R.suspend <- 0
	terr.VerbPrint(R.outputTo, 0, verb, R.ID[:6], "Resumed")
}
func (R *RaftMember) run(PStore *atomicGob.GobFile) {
	go func() {
		for {
			select {
			case x := <-R.suspend:
				for x != 0 {
					select {
					case x = <-R.suspend:
						if x == 0 {
							R.et.Renew()
						}
					case <-R.CM.Messages:
						continue
					}
				}
			case <-R.et.C:
				R.electionTimerExpired()
			case <-R.hrt.C:
				R.heartrateExpired()
			case M := <-R.CM.Messages:
				switch M.Type {
				case CM.Append:
					terr.VerbPrint(R.outputTo, 5, verb, R.ID[:6], "Recieved Append from", M.SenderID[:6])
					if AR, ok := M.Payload.(AppendEntry); ok {
						R.processAppend(&AR)        //process the request
						R.reply(M, CM.Appended, AR) //send the response
					} else {
						terr.VerbPrint(R.outputTo, 0, verb, "Unable to cast entry in Append")
					}
				case CM.Appended:
					AR := M.Payload.(AppendEntry)
					R.processAppended(&AR)
				case CM.Print:
					terr.VerbPrint(R.outputTo, 1, verb, R.ID[:6], "Recieved Print:", M.Payload)
				case CM.Connect:
					R.CM.PM(M.SenderAddress, R.CM.BuildMessage(CM.AddMembers, R.CM.Clients.Get()))
					R.nextIndex[M.SenderID] = len(R.Log)
					R.ClientCount = len(R.nextIndex)
				case CM.AddMembers:
					newBook := (M.Payload.(map[string]string))
					R.processBook(newBook)
				case CM.VoteRequest:
					VR, ok := M.Payload.(RequestVote)
					if !ok {
						terr.BreakError("Vote Request Not Convertible", errors.New("Vote Request Cast Fail"))
					}
					R.processVoteRequest(&VR) //process the request
					R.reply(M, CM.Vote, VR)   // send the processed result
				case CM.Vote:
					VR := M.Payload.(RequestVote)
					R.processRecievedVote(&VR) // process the request
				case CM.RA:
					new := M.Payload.(string)
					R.Append(new) //try to append the requet, will send to leader if member is not leader.
				case CM.Guess:
					terr.VerbPrint(R.outputTo, 5, verb, "Guess Recieved")
					theGuess := M.Payload.(*Guess)
					R.processGuess(theGuess)
				}
			case <-R.needStatus:
				NSB := new(StatusBall)
				NSB.Address = R.CM.Address
				NSB.Leader = R.leader
				NSB.LogLength = len(R.Log)
				NSB.Term = R.Term
				NSB.ID = R.ID
				NSB.Status = R.Status
				for ID, Address := range R.CM.Clients.Get() {
					NSB.Members = append(NSB.Members, ID[:6]+" | "+Address)
				}
				R.giveStatus <- *NSB
			case <-R.needLog:
				R.giveLog <- R.printLog()
			case new := <-R.appendRequests:
				if R.Status == Leader {
					R.Log = append(R.Log, LogEntry{Entry: new, Term: R.Term})
					terr.VerbPrint(R.outputTo, 4, verb, "Entry Appended:", new)
				} else {
					R.sendLeader(R.CM.BuildMessage(CM.RA, new))
				}
			case <-R.needScoreBoard:
				R.GiveScoreboard <- R.Machine.GetScore()
			default:
				R.takeAGuess()
			} //end of message select
			R.writeNeed(PStore) // checks if a write needs to occur and does it.
		} //end of for loop
	}() //call of internal goRoutine
}

func (R *RaftMember) processGuess(theGuess *Guess) {
	if R.Status == Leader { // only the leader can append a point
		if ok := R.Machine.GuessNumber(theGuess.Guess); ok && !R.alreadyGuessed {
			//the guess was ok, try to append it
			R.alreadyGuessed = true
			R.Append("PointScored," + theGuess.ID + "," + strconv.Itoa(theGuess.Guess) + "," + strconv.Itoa(R.Machine.GetNumber()))
		}
	}
}
func (R *RaftMember) takeAGuess() (int, bool) {
	//If you have nothing else to do, guess at the next point.
	var ok bool
	var guess int
	if guess, ok = R.Machine.Guess(); ok {
		//yay, you think you scored. (but maybe someone beat you to it... sigh)
		//we need to ask the leader if we were right.
		theGuess := new(Guess)
		theGuess.ID = R.ID
		theGuess.Guess = guess

		if R.Status == Leader {
			R.processGuess(theGuess)
		} else {
			msg := *new(CM.Message)
			msg.Payload = theGuess
			msg.SenderAddress = R.CM.Address
			msg.SenderID = R.ID
			msg.Type = CM.Guess
			R.sendLeader(msg)
		}
	}
	return guess, ok
}

func (R *RaftMember) processAppend(AR *AppendEntry) {
	//first we fill ouy the basics of our response:
	AR.Reply.CommitIndex = R.commitIndex
	AR.Reply.ID = R.ID
	AR.Reply.LastApplied = R.LastApplied
	AR.Reply.Term = R.Term
	AR.Reply.Result = false
	//if the leader's term is incorrect, we fire our response back
	if AR.Term < R.Term {
		return
	}
	//from here we have a valid leader

	R.et.Renew() //renew the countdown

	//update the state to match the leader's information
	if R.Term != AR.Term {
		R.Term = AR.Term
		R.needWrite = true
	}
	if R.leader != AR.ID {
		R.leader = AR.ID
		R.needWrite = true
	}
	if R.Status != Follower {
		R.Status = Follower
		R.needWrite = true
	}

	//previous entry doesnt exist or doesnt match
	if len(R.Log) <= AR.PrevLogIndex || R.Log[AR.PrevLogIndex].Term != AR.PrevLogTerm {
		terr.VerbPrint(R.outputTo, 1, verb, R.ID[:6], "Recievd append from valid leader of prevIndex:", AR.PrevLogIndex, "and term:", AR.PrevLogTerm, "that did not match")
		return
	}

	AR.Result = true //we now set the result to true, since the append will be accepted.

	//a hearbeat has no entries.
	//We also account for a non-matching commit index, whcih will fall throuhg and get updated.
	if len(AR.Entries) == 0 && R.commitIndex == AR.CommitIndex && AR.LastApplied == R.LastApplied {
		return
	}
	//everything below modifies the state and needs a permanent write.
	R.needWrite = true
	//if the last entry in the log is greater than the previous index sent, we slice our log back to the previous size.
	if len(R.Log)-1 > AR.PrevLogIndex {
		R.Log = R.Log[:AR.PrevLogIndex+1]
	}

	//stack any new entries on the end
	if len(AR.Entries) > 0 && len(R.Log)-1 == AR.PrevLogIndex { // required in case of non-matching commit indexes
		R.Log = append(R.Log, AR.Entries...)
	}

	R.commitIndex = len(R.Log) - 1 //we set the new commit index
	AR.Reply.CommitIndex = R.commitIndex
	//apply any outstanding entries.
	R.applyEntries(AR.LastApplied)
	//update our lastApplied
	AR.Reply.LastApplied = R.LastApplied
}

func (R *RaftMember) processAppended(AR *AppendEntry) {
	if AR.Result && R.Status == Leader {
		terr.VerbPrint(R.outputTo, 4, verb, R.ID[:6], "Recieved Appended from", AR.Reply.ID, "of index:", AR.Reply.CommitIndex+1)
		R.nextIndex[AR.Reply.ID] = AR.Reply.CommitIndex + 1 //the next entry we send
		R.matchIndex[AR.Reply.ID] = AR.Reply.LastApplied
		R.commitIndex = max(R.commitIndex, min(len(R.Log), R.AssessCommits()-1)) // makes sure that the commit index cannot go backwards
		R.applyEntries(R.commitIndex)
	} else {
		if AR.Reply.Term > R.Term {
			R.Status = Follower
			R.Term = AR.Reply.Term
			terr.VerbPrint(R.outputTo, 1, verb, R.ID[:6], "was demoted by append reply from", AR.Reply.ID[:6])
			R.needWrite = true
		} else {
			//if the commitINdexes arent the saem, try again with the appendReply index, otherwise, decrement the next by 1 looking for a match
			if AR.CommitIndex != AR.Reply.CommitIndex {
				R.nextIndex[AR.Reply.ID] = AR.Reply.CommitIndex + 1 //we correct the position of our append
			} else {
				R.nextIndex[AR.Reply.ID]--
			}
		}
	}
}

func (R *RaftMember) buildClientPayload(ID string) *AppendEntry {
	//initialize the response
	payload := new(AppendEntry)
	payload.Term = R.Term
	payload.CommitIndex = R.commitIndex
	payload.PrevLogIndex, payload.PrevLogTerm = R.getLLILLT()
	payload.ID = R.ID
	payload.LastApplied = R.LastApplied

	if memberNextIndex, ok := R.nextIndex[ID]; ok {
		//the member exists... good.
		if len(R.Log) == memberNextIndex { //this member needs no append...
			//we need no appends
			return payload
		}
		payload.PrevLogIndex = min(memberNextIndex-1, len(R.Log)-1) // the previous index is their current index
		payload.PrevLogTerm = R.Log[payload.PrevLogIndex].Term
		payload.Entries = R.Log[payload.PrevLogIndex+1:] //the entries are a slice from the member's next index to the end
		return payload
	}
	payload.PrevLogIndex = 0         //point to init
	payload.PrevLogTerm = 0          //init term
	payload.Entries = R.Log[1:]      //everything except init
	R.nextIndex[ID] = len(R.Log)     //add it to the client list for next time
	R.ClientCount = len(R.nextIndex) //reset the client count
	return payload
}
func (R *RaftMember) heartrateExpired() (int, int) {
	messagesSent := 0
	entriesSent := 0
	if R.Status == Leader {
		terr.VerbPrint(R.outputTo, 5, verb, R.ID[:6], "Sending Heartbeat")

		for ID, Address := range R.CM.Clients.Get() { // get the client list
			terr.VerbPrint(R.outputTo, 5, verb, "Clients Are:", R.nextIndex)
			payload := R.buildClientPayload(ID)
			if len(payload.Entries) > 0 {
				terr.VerbPrint(R.outputTo, 4, verb, "Member:", ID[:6], "Payload Term:", payload.Term, "Payload Index:", payload.PrevLogIndex, "Payload Entries:", payload.Entries)
				entriesSent += len(payload.Entries)
			}
			R.CM.PM(Address, R.CM.BuildMessage(CM.Append, *payload)) //send the payload to the lucky recipient
			messagesSent++
		}
		//reset the heartrate timer.
		R.hrt = time.NewTimer(time.Duration(hrInterval) * time.Millisecond)
	}
	return messagesSent, entriesSent
}
func (R *RaftMember) applyEntries(toIndex int) int {
	entriesApplied := 0
	if R.commitIndex >= len(R.Log) {
		terr.VerbPrint(R.outputTo, 0, verb, "R.commitIndex reset because it was too large.")
		R.commitIndex = len(R.Log) - 1
	}
	target := min(R.commitIndex, toIndex)
	for target > R.LastApplied {
		R.handleCommit(R.Log[R.LastApplied+1].Entry) // commit the next entry
		R.LastApplied++
		entriesApplied++
		terr.VerbPrint(R.outputTo, 5, verb, R.ID[:6], "Applying Index:", R.LastApplied, "With value:", R.Log[R.commitIndex].Entry)
		R.needWrite = true
	}
	return entriesApplied
}
func (R *RaftMember) electionTimerExpired() {
	if R.Status == Follower {
		terr.VerbPrint(R.outputTo, 3, verb, R.ID[:6], "ElectionTimer Expired, moving to candidate.")
		R.Term++             //increase our term
		R.Status = Candidate //move to candidate
		R.vote(R.Term, R.ID) //vote for myself
		R.recieveVote(R.Term, R.ID)
		VR := new(RequestVote)
		VR.ID = R.ID
		VR.LastLogIndex, VR.LastLogTerm = R.getLLILLT()
		VR.Term = R.Term
		msg := R.CM.BuildMessage(CM.VoteRequest, VR)
		R.Broadcast(msg)
		R.et.Renew()
	} else if R.Status == Candidate {
		terr.VerbPrint(R.outputTo, 3, verb, R.ID[:6], "ElectionTimer Expired, demoting back to Follower.")
		R.et.Renew()
		R.Status = Follower
	}
	R.needWrite = true
}
func (R *RaftMember) processBook(newBook map[string]string) {
	terr.VerbPrint(R.outputTo, 5, verb, R.ID[:6], "recieved Book", newBook)
	for k, v := range newBook {
		if R.ID != k && !R.CM.Clients.Exists(k) {
			R.CM.Connect(v)
		}
		R.nextIndex[k] = len(R.Log)
		R.ClientCount = len(R.nextIndex)
	}
}
func (R *RaftMember) processVoteRequest(VR *RequestVote) {
	if VR.Term > R.Term {
		R.Term = VR.Term
		R.Status = Follower
		R.et.Renew()
		terr.VerbPrint(R.outputTo, 3, verb, "Vote Request Not in Sync. Term Adjusted")
	}
	if R.Status == Follower {
		myLastIndex, myLastTerm := R.getLLILLT()
		if VR.Term < R.Term || (VR.LastLogIndex < myLastIndex) || VR.LastLogTerm != myLastTerm {
			VR.Reply.Result = false
			VR.Reply.Term = R.Term
			VR.Reply.ID = R.ID
		} else { // we have a valid request for votes
			if R.vote(VR.Term, VR.ID) {
				VR.Reply.ID = R.ID
				R.et.Renew()
				R.Term = VR.Term
				VR.Reply.Result = true
				VR.Reply.Term = R.Term
				terr.VerbPrint(R.outputTo, 3, verb, R.ID[:6], "voted for", VR.Reply.ID[:6], "in term", VR.Term)
				R.needWrite = true
			} else {
				VR.Reply.ID = R.ID
				VR.Reply.Result = false
				VR.Reply.Term = R.Term
			}
		}
	}
}
func (R *RaftMember) processRecievedVote(VR *RequestVote) {
	if VR.Reply.Result == true {
		if VR.Reply.Term == R.Term && R.Status == Candidate {
			terr.VerbPrint(R.outputTo, 3, verb, R.ID[:6], "recieved vote from", VR.ID[:6], "in term", VR.Term)
			R.et.Renew()
			//now we record the vote
			R.recieveVote(VR.Term, VR.Reply.ID)
			R.checkForMajority()
		}
	} else {
		if VR.Reply.Term > R.Term {
			terr.VerbPrint(R.outputTo, 1, verb, R.ID[:6], "Recieved Demote Message in vote from", VR.Reply.ID[:6])
			R.Status = Follower
			R.Term = VR.Reply.Term // set my term to match
		}
	}
	R.et.Renew()
}

func min(nums ...int) int {
	cMin := nums[0]
	for _, x := range nums {
		if x < cMin {
			cMin = x
		}
	}
	return cMin
}

func (R *RaftMember) writeNeed(PStore *atomicGob.GobFile) {
	if R.needWrite {
		R.writePersist(PStore)
		R.needWrite = false
		terr.VerbPrint(R.outputTo, 5, verb, R.ID[:6], "Writing")
	}
}
func (R *RaftMember) checkForMajority() {
	numMembers := R.ClientCount + 1
	numVotes := R.getVotes(R.Term)
	if numVotes > numMembers-numVotes {
		//we have a winner!
		R.Status = Leader
		terr.VerbPrint(R.outputTo, 1, verb, R.ID[:6], "Became Leader in Term:", R.Term, "With", numVotes, "Member votes.")
		R.hrt = time.NewTimer(time.Duration(hrInterval) * time.Millisecond)
		for ID := range R.nextIndex {
			R.nextIndex[ID] = len(R.Log)
			R.matchIndex[ID] = 0
		}
	}
}

func (R *RaftMember) getLLILLT() (int, int) {
	LLT := 0 // a 0 term is impossible, this indicates there is no LLT
	LLI := max(len(R.Log)-1, 0)
	if LLI > 0 {
		LLT = R.Log[LLI].Term
	}
	return LLI, LLT
}

func max(nums ...int) int {
	x := nums[0]
	for _, y := range nums {
		if y > x {
			x = y
		}
	}
	return x
}
func (R *RaftMember) LogOk(LastLogIndex, LastLogTerm int) bool {
	LLI := len(R.Log) - 1
	return LastLogIndex == LLI && R.Log[LLI].Term == LastLogTerm
}

func (R *RaftMember) AssessCommits() int {
	//NOTE This could be better. Uses a copy of a map for a relatively static number
	if R.ClientCount < 1 {
		return 1
	}
	size := len(R.nextIndex)
	arr := make([]int, size)
	i := 0
	for _, v := range R.nextIndex {
		arr[i] = v
		i++
	}
	sort.Ints(arr)
	return arr[(size)/2]
}

func (R *RaftMember) Append(N string) {
	R.appendRequests <- N
}
func (R *RaftMember) sendLeader(msg CM.Message) {
	if R.Status != Leader && R.leader != "" {
		CL := R.CM.Clients.Get()
		R.CM.PM(CL[R.leader], msg)
	}
}
func (R *RaftMember) GetStatus() *StatusBall {
	R.needStatus <- true
	x := <-R.giveStatus
	return &(x)
}
func (R *RaftMember) GetScore() map[string]int {
	R.needScoreBoard <- true
	x := <-R.GiveScoreboard
	//clone the scoreboard, and send it out.
	NM := make(map[string]int)
	for k, v := range x {
		NM[k] = v
	}
	return NM
}
func (R *RaftMember) GetLog() string {
	R.needLog <- true
	return <-R.giveLog
}

func (R *RaftMember) Connect(IP string) {
	if IP != R.CM.Address && IP != "" {
		R.CM.Connect(IP)
	}
}

func (R *RaftMember) haveVoted(term int) bool {
	if len(R.VoteLog)-1 >= term { // the term is inside the array
		if R.VoteLog[term].VotedFor != "" { // check the term for a vote cast
			return true
		}
		return false
	}
	return false
}

func (R *RaftMember) vote(term int, candidate string) bool {
	if R.haveVoted(term) {
		if R.VoteLog[term].VotedFor == candidate { // we are being asked to vote for the same guy again
			terr.VerbPrint(R.outputTo, 4, verb, R.ID[:6], "Vote Request Repeated For", candidate[:6])
			return true
		}
		return false
	}
	//fix the length of the array
	for len(R.VoteLog)-1 < term {
		NVR := *new(VoteRecord)
		NVR.Votes = make(map[string]bool)
		R.VoteLog = append(R.VoteLog, NVR)
	}
	//cast our vote
	R.VoteLog[term].VotedFor = candidate
	return true
}

func (R *RaftMember) recieveVote(term int, ID string) {
	R.VoteLog[term].Votes[ID] = true
}

func (R *RaftMember) getVotes(term int) int {
	if !R.haveVoted(term) {
		return 0
	}
	return len(R.VoteLog[term].Votes)
}

func readPersist() (*RaftMember, *atomicGob.GobFile, bool) {
	var tempI interface{}
	RM := new(RaftMember)
	if terr.FileExists("/tmp/raft/persist.raft") { //the raft has history, load it
		//R := new(RaftMember)
		myPersist := atomicGob.NewGobFile("/tmp/raft/persist.raft", RM)
		myPersist.Read(&tempI)
		myPersist.Wait()
		if tempI != nil { //read the persist into temp
			R, ok := tempI.(*RaftMember) // Type assert that the returned gob is a Raft.
			if !ok {
				terr.PrintError("Gob Load On Server", errors.New("Gob Load Failed"))
			} else {
				return R, myPersist, true
			}
		}
		return RM, myPersist, false
	}
	//it didnt exist, so make it
	myPersist := atomicGob.NewGobFile("/tmp/raft/persist.raft", RM)
	return RM, myPersist, false
}
func (R *RaftMember) Broadcast(M CM.Message) {
	R.CM.Broadcast(M)
}

func (R RaftMember) writePersist(P *atomicGob.GobFile) {
	if P != nil {
		P.Lock()
		R.CM.Clients.Lock()
		defer P.Unlock()
		defer R.CM.Clients.Unlock()
		P.Data = R
		P.Write()
		P.Wait() // wait for the write to finish
	}
}

//SHA2 returns a random sha-256 string
func makeID() (string, error) {
	data := make([]byte, 32)
	_, err := rand.Read(data)
	x := sha256.Sum256(data)
	return hex.EncodeToString(x[:]), err
}

//GetIP returns the ip address of the local machine (not localhost)
func GetIP() string {
	ifaces, err := net.Interfaces()
	terr.PrintError("problem collecting local ip address", err)
	// handle err
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		terr.PrintError("problem collecting local ip address", err)
		// handle err
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			//if the address is not loopback return it
			if ip.String() != "127.0.0.1" && ip.String() != "::1" {
				return ip.String()
			}
		}
	}
	return ""
}

func (S StatusBall) String() string {
	ans := "\nAddress:" + S.Address + " ID:" + S.ID[:6] + " Number of Members: " + strconv.Itoa(len(S.Members)+1)
	if S.Status == Follower {
		ans += "\nMy Status: Follower"
		if S.Leader == "" {
			ans += "\nMy Leader: NIL"
		} else {
			ans += "\nMy Leader: " + S.Leader
		}
	} else if S.Status == Candidate {
		ans += "\nMy Status: Candidate"
	} else if S.Status == Leader {
		ans += "\nMy Status: Leader"
	}

	ans += "\nMy Term: " + strconv.Itoa(S.Term)
	ans += "\nMy Log Length: " + strconv.Itoa(S.LogLength)
	i := 1
	for _, ID := range S.Members {
		ans = ans + ("\n\tMember:" + strconv.Itoa(i) + " " + ID[:6])
		i++
	}
	return ans
}

func printState(R *RaftMember) {
	if R.Status == Leader {
		fmt.Println(R, "-->", redString("is a Leader"))
	} else if R.Status == Candidate {
		fmt.Println(R, "-->", yellowString("is a Candidate"))
	} else if R.Status == Follower {
		fmt.Println(R, "-->", greenString("is a Follower"))
	}
}

//PrintLog outputs the log of the member for debugging
func (R *RaftMember) printLog() string {
	numEntries := 5
	if R.Log == nil {
		return ""
	}
	var top, mid, bottom string
	stopper := "|"
	top = stopper
	bottom = stopper
	mid = stopper
	start := max(0, len(R.Log)-numEntries) // make the start 10 back from the end
	for i, v := range R.Log[start:] {
		args := strings.Split(v.Entry, ",")
		if args[0] == "Guess" {
			v.Entry = args[1][:6] + " Guessed:" + args[2]

		} else if args[0] == "PointScored" {
			v.Entry = args[1][:6] + ": " + args[2]
		}
		l1 := len(strconv.Itoa(i))
		l2 := len(strconv.Itoa(v.Term))
		l3 := len(v.Entry)
		var padNum int
		if l1 >= l2 && l1 >= l3 {
			padNum = l1
		} else if l2 > l1 && l2 >= l3 {
			padNum = l2
		} else {
			padNum = l3
		}
		top = top + pad(strconv.Itoa(i+start), padNum) + stopper
		mid = mid + pad(strconv.Itoa(v.Term), padNum) + stopper
		bottom = bottom + pad(v.Entry, padNum) + stopper
	}
	top = "Log Index:" + top + "\n"
	top += ("          ") //offset the headings...
	sep := ""
	for j := 0; j < len(bottom); j++ {
		sep += "-"
	}
	top += sep
	top += "\n     Term:" + mid + "\n          "
	top += sep
	top += "\n    Entry:" + bottom
	top += "\n"
	return top
}
func pad(strIn string, padTo int) string {
	for len(strIn) < padTo+2 {
		strIn = strIn + " "
		if len(strIn) >= padTo+2 {
			continue
		}
		strIn = " " + strIn
	}
	return strIn
}
func redString(in string) string {
	return "\033[1;31m" + in + "\033[0m"
}

func greenString(in string) string {
	return "\033[1;32m" + in + "\033[0m"
}

func yellowString(in string) string {
	return "\033[1;33m" + in + "\033[0m"
}
func highlightString(in string) string {
	return "\033[1;45m" + in + "\033[0m"
}
