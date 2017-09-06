package member

import (
	"errors"
	"fmt"
	"os"
	CM "raft/connectionManager"
	"raft/stateMachine"
	"raft/terr"

	"raft/timers"
	"strings"
	"testing"
	"time"
)

func centre100(in string) string {
	startingPosition := 50 - len(in)/2
	endingPosition := startingPosition + len(in)
	reply := make([]byte, 100)
	for x := 0; x < 100; x++ {
		if x >= startingPosition && x < endingPosition {
			reply[x] = in[x-startingPosition]
		} else {
			reply[x] = ' '
		}
	}
	return string(reply)

}
func overline() {
	for x := 0; x < 100; x++ {
		fmt.Print("_")
	}
	fmt.Println()
}
func underline() {
	for x := 0; x < 100; x++ {
		fmt.Print("\u203E")
	}
	fmt.Println()
}
func printComplete() {
	overline()
	fmt.Println(centre100("COMPLETE"))
	underline()
}
func printHeader(header string) {
	header = strings.ToUpper(header)
	fmt.Println()
	overline()
	fmt.Println(centre100(header))
	underline()
}

func Test_SKELETON(t *testing.T) {
	printHeader("SKELETON")

	printComplete()
}

func (R *RaftMember) addClient(ID string) {
	R.CM.Clients.Add(ID, "localhost:99999")
	R.nextIndex[ID] = len(R.Log)
	R.ClientCount = len(R.nextIndex)
}

func Test_Appended(t *testing.T) {
	printHeader("Testing appended")
	R := MockFollowerWithCM("S1    ", "localhost:10013")
	R.Status = Leader
	//add a single Client
	R.addClient("Follo1")
	R.addClient("Follo2")
	R.addClient("Follo3")

	AE := MockAppendEntry("S1    ", "Follo1")
	R.commitIndex = 0
	R.LastApplied = 0
	R.processAppended(AE)

	//basic feature test
	if R.nextIndex["Follo1"] != 1 {
		t.Error("Initial Index Incorrect:", R.nextIndex["Follo1"])
	}
	if R.matchIndex["Follo1"] != 0 {
		t.Error("Initial Match index incorrect:", R.matchIndex["Follo1"])
	}
	if R.commitIndex != 0 {
		t.Error("Commit index not intialized to 0")
	}
	if R.LastApplied != 0 {
		t.Error("last applied not initialized to 0")
	}

	//We append some entries, modify the reply's commit index.
	//This means that the raftmember is indicating it has appended the first and second entries
	//CommitIndex of 2 means log len is 3, and last index is 2.
	R.Log = append(R.Log, LogEntry{Term: 1, Entry: "Append 1"})
	R.Log = append(R.Log, LogEntry{Term: 1, Entry: "Append 2"})

	AE.Reply.CommitIndex = 1
	AE.Result = true

	R.processAppended(AE)
	if R.commitIndex != 0 {
		t.Error("Commit index changed by append:", R.commitIndex)
	}
	if R.LastApplied != 0 {
		t.Error("R Applied uncommited entry:", R.LastApplied)
	}
	if R.nextIndex["Follo1"] != 2 {
		t.Error("F1 Nextindex not updated", R.nextIndex["Follo1"])
	}
	//Follo2 has not commited the entries yet, and still has a next index of 1.
	if R.nextIndex["Follo2"] != 1 {
		t.Error("F2 index changed", R.nextIndex["Follo2"])
	}

	//now when Follo2 commits, the leader will update since this is now a 3/4 majority.
	AE.Reply.CommitIndex = 2
	AE.Reply.ID = "Follo2"

	R.processAppended(AE)
	if R.commitIndex != 1 { //matches the majority.
		t.Error("Commit index not increased by appended", R.commitIndex)
	}
	if R.LastApplied != 1 {
		t.Error("lastApplied not correct", R.LastApplied)
	}
	if R.nextIndex["Follo1"] != 2 {
		t.Error("F1 Nextindex changed")
	}
	if R.nextIndex["Follo2"] != 3 {
		t.Error("F2 next index not changed")
	}

	//now we check how a demotion is handled by resetting the commit index of Follo2
	AE.Reply.CommitIndex = 0
	R.processAppended(AE)

	if R.commitIndex != 1 { //shouldn't change
		t.Error("Commit index changed", R.commitIndex)
	}
	if R.LastApplied != 1 { //cant Change
		t.Error("lastApplied changed", R.LastApplied)
	}
	if R.nextIndex["Follo1"] != 2 {
		t.Error("F1 Nextindex changed")
	}
	if R.nextIndex["Follo2"] != 1 {
		t.Error("F2 next index not changed")
	}
	//Now we add a new member to see how the commits are handled.
	R.addClient("Follo4")
	AE.Reply.CommitIndex = 2
	AE.Reply.ID = "Follo4"
	R.processAppended(AE)
	AE.Reply.ID = "Follo3"
	R.processAppended(AE)
	if R.commitIndex != 2 { //shouldn't change
		t.Error("Commit index incorrect", R.commitIndex)
	}
	if R.LastApplied != 2 { //cant Change
		t.Error("lastApplied incorrect", R.LastApplied)
	}
	if R.nextIndex["Follo1"] != 2 {
		t.Error("F1 Next index changed")
	}
	if R.nextIndex["Follo2"] != 1 {
		t.Error("F2 next index changed")
	}
	if R.nextIndex["Follo3"] != 3 {
		t.Error("F3 next index not changed")
	}
	if R.nextIndex["Follo4"] != 3 {
		t.Error("F4 next index not changed")
	}

	//test failures
	//first an index adjustment
	AE.Result = false
	AE.Reply.CommitIndex = 0
	AE.CommitIndex = R.commitIndex
	R.processAppended(AE)
	if R.commitIndex != 2 { //shouldn't change
		t.Error("Commit index chagned", R.commitIndex)
	}
	if R.LastApplied != 2 { //cant Change
		t.Error("lastApplied changed", R.LastApplied)
	}
	if R.nextIndex["Follo1"] != 2 {
		t.Error("F1 Next index changed")
	}
	if R.nextIndex["Follo2"] != 1 {
		t.Error("F2 next index changed")
	}
	if R.nextIndex["Follo3"] != 1 {
		t.Error("F3 next index not changed", R.nextIndex["Follo3"] != 1)
	}
	if R.nextIndex["Follo4"] != 3 {
		t.Error("F4 next index changed")
	}

	AE = MockAppendEntry("S1    ", "Follo4")
	AE.Reply.Term = 1
	AE.Reply.Result = false
	AE.Reply.LastApplied = 0
	AE.Term = R.Term
	AE.Reply.CommitIndex = 2 //the commit indexes need to match to make sure that the decrement happens instead of a replace.
	AE.CommitIndex = 2
	R.processAppended(AE) //single imcrement demote
	if R.nextIndex["Follo4"] != 2 {
		t.Error("F4 next index not decremented", R.nextIndex["Follo4"])
	}
	R.processAppended(AE)
	if R.nextIndex["Follo4"] != 1 { //repeat single increment demote
		t.Error("F4 next index not decremented a second time", R.nextIndex["Follo4"])
	}
	//make sure that a demotion has effect,
	AE.Reply.Term = 2
	R.processAppended(AE)
	if R.Status == Leader {
		t.Error("Demotion failed")
	}
	if R.Term != 2 {
		t.Error("Term not updated at demotion")
	}

	printComplete()
	//need to test double singe demotion
}
func Test_buildClientPayload(t *testing.T) {
	printHeader("Payload Builder")
	R := MockFollowerWithCM("S1    ", "localhost:20011")
	R.Status = Leader
	thePayload := R.buildClientPayload("S2    ")
	if thePayload.CommitIndex != R.commitIndex {
		t.Error("commit index not set in payload")
	}
	if len(thePayload.Entries) > 0 {
		t.Error("Payload entries not nil")
	}
	if thePayload.LastApplied != R.LastApplied {
		t.Error("Last Applied incorrect")
	}
	if thePayload.PrevLogIndex != 0 || thePayload.PrevLogTerm != 0 {
		t.Error("initial payload previous not init")
	}
	if thePayload.Reply.Result {
		t.Error("initial result should be false")
	}
	if thePayload.Term != R.Term {
		t.Error("payload term incorrect")
	}

	//add a client
	R.CM.Clients.Add("S2    ", "localhost:99999")

	R.Log = append(R.Log, LogEntry{Term: 1, Entry: "Append 1"})
	R.Log = append(R.Log, LogEntry{Term: 1, Entry: "Append 2"})
	//set its next index to the appended entry index
	//This should send both entries (1 and 2)
	R.nextIndex["S2    "] = 1
	thePayload = R.buildClientPayload("S2    ")
	if len(thePayload.Entries) != 2 {
		t.Error("did not send correct number of entries. Sent", len(thePayload.Entries))
	}

	//this should only send Append 2
	R.nextIndex["S2    "] = 2
	thePayload = R.buildClientPayload("S2    ")
	if len(thePayload.Entries) != 1 {
		t.Error("did not send correct number of entries. Sent", len(thePayload.Entries))
	}

	//this should send nothing
	R.nextIndex["S2    "] = 3
	thePayload = R.buildClientPayload("S2    ")
	if len(thePayload.Entries) != 0 {
		t.Error("did not send correct number of entries. Sent", len(thePayload.Entries))
	}
	printComplete()
}
func Test_HeartrateExpired(t *testing.T) {
	R := MockFollowerWithCM("S1    ", "localhost:20012")
	R.CM.Clients.Add("S2    ", "localhost:99999")
	R.CM.Clients.Add("S3    ", "localhost:99999")
	R.Log = append(R.Log, LogEntry{Term: 1, Entry: "Append 1"})
	R.Log = append(R.Log, LogEntry{Term: 1, Entry: "Append 2"})
	MS, AE := R.heartrateExpired()
	if MS != 0 || AE != 0 {
		t.Error("Messages sent from nonLeader")
	}
	R.Status = Leader
	MS, AE = R.heartrateExpired()
	if MS != 2 {
		t.Error("Hearbeats Not sent")
	}
	if AE != 4 { //the process attempts to send the entire log to unknown Members.
		t.Error("4 Entries should have been appended, instead recived", AE)
	}

	//Now set the nextIndex for the Members
	R.nextIndex["S2    "] = 1
	R.nextIndex["S3    "] = 3
	MS, AE = R.heartrateExpired()
	if MS != 2 {
		t.Error("Hearbeats Not sent")
	}
	if AE != 2 {
		t.Error("2 Entries should have been appended, instead recived", AE)
	}

	R.nextIndex["S2    "] = 2
	R.nextIndex["S3    "] = 3
	MS, AE = R.heartrateExpired()
	if MS != 2 {
		t.Error("Hearbeats Not sent")
	}
	if AE != 1 {
		t.Error("1 Entry should have been appended, instead recived", AE)
	}
	R.nextIndex["S2    "] = 3
	R.nextIndex["S3    "] = 3
	MS, AE = R.heartrateExpired()
	if MS != 2 {
		t.Error("Hearbeats Not sent")
	}
	if AE != 0 {
		t.Error("No entries should have been sent", AE)
	}
	//test large numbers
	R.nextIndex["S2    "] = 989
	R.nextIndex["S3    "] = 3898989
	MS, AE = R.heartrateExpired()
	if MS != 2 {
		t.Error("Hearbeats Not sent")
	}
	if AE != 0 {
		t.Error("No entries should have been sent", AE)
	}
}

func Test_VotingSystem(t *testing.T) {
	printHeader("Testing Voting Internals")
	R := RaftMember{}
	R.VoteLog = append(R.VoteLog, VoteRecord{Votes: make(map[string]bool), VotedFor: "NIL"}) //bootstrap the vote record for the 0 term
	R.Status = Candidate
	R.ID, _ = makeID()
	R.outputTo = os.Stdout
	R.Term++
	R.vote(R.Term, R.ID) //vote for yourself
	R.vote(R.Term, R.ID) //vote for yourself
	R.recieveVote(R.Term, R.ID)
	numVotes := R.getVotes(R.Term)

	if numVotes != 1 {
		t.Error("vote not recorded properly.")
	}
	R.Term++
	numVotes = R.getVotes(R.Term)
	//fmt.Println(numVotes, "Votes in term", R.Term)
	if numVotes > 0 {
		t.Error("vote count incorrect.")
	}
	printComplete()
}

func Test_RecieveVote(t *testing.T) {
	printHeader("Testing Vote Reception")
	R, VR, err := setCan("TEST01")
	if err != nil {
		t.Error("VoteFor Self Incorrect")
	}

	//make sure that an individual ID is only counted once.
	R.processRecievedVote(VR)
	R.processRecievedVote(VR)

	if R.getVotes(R.Term) != 2 {
		t.Error("Vote 1 Not Correct. Term was", R.Term)
	}

	//Check if a denied request is correctly processed
	R, VR, _ = setCan("TEST02")
	VR.Result = false      //the result is false.
	VR.Reply.Term = R.Term // the terms match

	R.processRecievedVote(VR)
	if R.getVotes(R.Term) != 1 {
		t.Error("Vote 2 Not Correct. Recieved", R.getVotes(R.Term), "votes")
	}
	if R.Status != Candidate {
		t.Error("incorrect demotion for denied vote")
	}

	//Check if a false request sets the term properly.
	R, VR, _ = setCan("TEST03")
	VR.Result = false
	VR.Reply.Term = 290
	R.processRecievedVote(VR)
	if R.getVotes(R.Term) != 0 || R.Term != 290 || R.Status != Follower {
		t.Error("NewTerm not Adopted Properly after Demotion")
	}
	//Check an incorrect term, and see if it is accepted
	R, VR, _ = setCan("TEST04")
	VR.Reply.Term = 2
	R.processRecievedVote(VR)
	if R.getVotes(R.Term) != 1 {
		t.Error("Registered vote for A different term")
	}

	//Check a second increment, and see if leader is set (Members are 4)
	R, VR, _ = setCan("TEST05")
	R.processRecievedVote(VR)
	if R.Status == Leader || R.getVotes(R.Term) != 2 {
		t.Error("Leader Declared Prematurely, or vote count incorrect")
	}
	//Count another vote
	VR.Reply.ID = "VOTER2"
	R.processRecievedVote(VR)
	R.ClientCount = 3
	if R.getVotes(R.Term) != 3 {
		t.Error("TEST04 Vote Not Correct")
	}
	if R.Status != Leader { // it should be the leader with 3/4 votes
		t.Error("Leader Not Declared.")
	}

	//Test the majority calculation (3/4 needed or 3/5 --> 2 external + vote for self)
	R, VR, _ = setCan("TEST06")
	R.ClientCount = 4
	R.processRecievedVote(VR)
	//Count another vote, from a different voter
	VR.Reply.ID = "VOTER2"
	R.processRecievedVote(VR)
	if R.Status != Leader { //it should still be the leader with 3/5 votes
		t.Error("Fail on 5 member check for majority")
	}

	printComplete()
}

func Test_processVoteRequest(t *testing.T) {
	//Checks a normal passing vote request
	printHeader("Testing Vote Request Processing")
	R, VR := setFollow("TEST01")

	R.processVoteRequest(VR)
	if VR.Result != true {
		t.Error("Initial Vote Request Rejected")
	}

	//Checks a VoteReqest of Lower Term
	R, VR = setFollow("TEST02")
	R.Term = 2
	VR.Result = false

	R.processVoteRequest(VR)
	if VR.Result == true && VR.Reply.Term != 2 {
		t.Error("Vote Request of lesser term not rejected")
	}

	//Checks for a VoteRequest of higher term
	R, VR = setFollow("TEST03")
	R.Term = 0

	R.processVoteRequest(VR)
	if VR.Result != true && R.Term != 1 {
		t.Error("Vote Request of Greater term not Accepted")
	}

	//Check if a second vote can be cast in the same term
	R, VR = setFollow("TEST04")
	VR.ID = "Voter1"
	R.processVoteRequest(VR)
	if VR.Result != true {
		t.Error("Vote Request replied incorrectly")
	}
	VR.ID = "Voter2"
	R.processVoteRequest(VR)
	if VR.Result == true {
		t.Error("Vote Request replied twice")
	}
	//Check if a second request is met with a true for the same candidate.
	VR.ID = "Voter1"
	R.processVoteRequest(VR)
	if VR.Result != true {
		t.Error("Vote Request not replied a second time")
	}

	//make sure a candidate or leader will not process a vote request.
	R, VR = setFollow("TEST05")
	R.Status = Candidate
	R.processVoteRequest(VR)
	if VR.Result == true {
		t.Error("Candidate Processed Vote Request")
	}
	R.Status = Leader
	if VR.Result == true {
		t.Error("Leader Processed Vote Request")
	}
	//check that a leader is demoted from a vote request of greater term
	R, VR = setFollow("TEST05")
	VR.Term = 2
	R.Status = Leader
	R.processVoteRequest(VR)
	if R.Status != Follower {
		t.Error("No Demotion in Vote Request")
	}
	printComplete()
}

func Test_Guess(t *testing.T) {
	printHeader("GuessTest")
	var guess int
	var ok bool
	Lead := MockMemberWithMachine("Leader")
	Lead.Status = Leader
	Follow := MockMemberWithMachine("Follo1")
	for guess, ok = Follow.takeAGuess(); !ok; guess, ok = Follow.takeAGuess() {
	} // while we have no good guess, we loop
	if guess != 1 {
		t.Error("Guess wrong")
	}
	for guess, ok = Lead.takeAGuess(); !ok; guess, ok = Lead.takeAGuess() {
	} // while we have no good guess, we loop
	if guess != 1 {
		t.Error("Guess wrong")
	}
	printComplete()
}

func MockMemberWithMachine(ID string) *RaftMember {
	R := MockFollower(ID)
	pwd, _ := os.Getwd()
	R.CM = new(CM.ConnectionManager)
	R.CM.ID = ID
	R.Machine = stateMachine.CreateStateMachine(pwd+"/machines/"+R.ID[:6]+".sm", 50) //build or recreate the state machine.
	terr.VerbPrint(R.outputTo, 1, verb, R.ID[:6], "Target Set to:", 1)
	R.Machine.SetTarget(1) //restores a target after a reboot
	return R
}

func setupAppend(ID string) (*RaftMember, *AppendEntry) {
	R := MockFollower(ID)
	AE := MockAppendEntry("S1    ", "xxxxxx") // we declare a non-sense id, to ensure it is replaced.
	R.processAppend(AE)                       //the first heartbeat will set the leader
	return R, AE
}
func MockAppendEntry(senderID, replyID string) *AppendEntry {
	AE := new(AppendEntry)
	AE.CommitIndex = 0
	AE.ID = senderID
	AE.Reply.ID = replyID
	AE.LastApplied = 0
	AE.PrevLogIndex = 0
	AE.PrevLogTerm = 0
	AE.Result = true
	AE.Term = 1
	AE.Reply.CommitIndex = 0
	AE.Reply.LastApplied = 0
	AE.Reply.Term = 1
	return AE
}
func mockBook() map[string]string {
	mockBook := make(map[string]string)
	mockBook["S1    "] = "localhost:20001"
	mockBook["S2    "] = "localhost:20002"
	mockBook["S3    "] = "localhost:20003"
	mockBook["S4    "] = "localhost:20004"
	return mockBook
}
func setCan(ID string) (*RaftMember, *RequestVote, error) {
	R := MockCandidate(ID)
	result := R.vote(R.Term, R.ID)
	if !result {
		return nil, nil, errors.New("No Candidate Vote")
	}
	R.recieveVote(R.Term, R.ID) //register your own vote.
	VR := MockVoteRequest()
	VR.Reply.Result = true
	VR.Reply.Term = 1
	VR.ID = ID
	return R, VR, nil
}

func setFollow(ID string) (*RaftMember, *RequestVote) {
	R := MockFollower(ID)
	VR := MockVoteRequest()
	return R, VR
}

func MockVoteRequest() *RequestVote {
	VR := new(RequestVote)
	VR.CommitIndex = 1
	VR.LastApplied = 1
	VR.Reply.Term = 1
	VR.Term = 1
	VR.Reply.ID = "VOTER1"
	return VR
}

func MockLeader(ID string) *RaftMember {
	R := MockFollower(ID)
	R.Status = Leader
	return R
}
func MockCandidate(ID string) *RaftMember {
	R := MockFollower(ID)
	R.Status = Candidate
	return R
}
func MockFollowerWithCM(ID, Address string) *RaftMember {
	R := MockFollower(ID)
	R.CM = CM.New(R.ID, Address, R.outputTo)
	return R
}

func MockFollower(ID string) *RaftMember {
	//try to get the existing data from file
	R := new(RaftMember)
	R.hrt = new(time.Timer) //initialize our timer.
	R.giveStatus = make(chan StatusBall)
	R.outputTo = os.Stdout
	R.needStatus = make(chan bool)
	R.giveLog = make(chan string)
	R.needLog = make(chan bool)
	R.nextIndex = make(map[string]int)
	R.matchIndex = make(map[string]int)
	R.appendRequests = make(chan string, 100)
	R.needScoreBoard = make(chan bool)
	R.GiveScoreboard = make(chan map[string]int)
	R.nextIndex = make(map[string]int)
	R.VoteLog = append(R.VoteLog, VoteRecord{Votes: make(map[string]bool), VotedFor: "NIL"}) //bootstrap the vote record for the 0 term
	R.ID = ID
	R.Log = append(R.Log, LogEntry{Term: 0, Entry: "init"}) // make the first entry an initialize for counting purposes.
	R.et = timers.NewElectionTimer()
	R.Term = 1
	R.Status = Follower
	R.ClientCount = 4
	R.LastApplied = 0
	R.commitIndex = 0
	return R
}
