// +build !race
//racy tests are racy for the sake of testing. They will trigger improper data races in the race detector.

package member

import (
	"fmt"
	"testing"
)

func Test_ProcessBook(t *testing.T) {
	printHeader("Testing Process book")
	//we first process a book with 4 clients, checking that they are all added
	R := MockFollowerWithCM("R1    ", "localhost:20000")
	mockBook := mockBook()
	R.processBook(mockBook)
	if R.ClientCount != 4 {
		t.Error("ClientCount Incorrect After book processing")
	}
	if _, ok := R.nextIndex["S1    "]; !ok {
		t.Error("Client 1 not added to list")
	}
	if _, ok := R.nextIndex["S2    "]; !ok {
		t.Error("Client 2 not added to list")
	}
	if _, ok := R.nextIndex["S3    "]; !ok {
		t.Error("Client 3 not added to list")
	}
	if _, ok := R.nextIndex["S4    "]; !ok {
		t.Error("Client 4 not added to list")
	}
	//we check if a 5th phantom client can be added
	if _, ok := R.nextIndex["S5    "]; ok {
		t.Error("Client 5 in list")
	}
	fmt.Println("Initial testing complete")
	// we process the book again to see if any adds are created
	// the book state should be the same as above.
	R.processBook(mockBook)
	if R.ClientCount != 4 {
		t.Error("ClientCount Incorrect After second processing")
	}
	if _, ok := R.nextIndex["S1    "]; !ok {
		t.Error("Client 1 not added to list")
	}
	if _, ok := R.nextIndex["S2    "]; !ok {
		t.Error("Client 2 not added to list")
	}
	if _, ok := R.nextIndex["S3    "]; !ok {
		t.Error("Client 3 not added to list")
	}
	if _, ok := R.nextIndex["S4    "]; !ok {
		t.Error("Client 4 not added to list")
	}
	if _, ok := R.nextIndex["S5    "]; ok {
		t.Error("Client 5 in list")
	}
	fmt.Println("Repeat Adding testing complete")
	printComplete()
}
func Test_ElectionTimerExpired(t *testing.T) {
	printHeader("Expiration of Election Timer")
	R := MockFollowerWithCM("S1    ", "localhost:20001")
	R.electionTimerExpired()
	if R.Status != Candidate {
		t.Error("promotion did not occurr")
	}
	R.electionTimerExpired()
	if R.Status == Candidate {
		t.Error("Automatic Demotion did not occur")
	}
	R.Status = Leader
	R.electionTimerExpired()
	if R.Status != Leader {
		t.Error("Leader demoted by expiry")
	}

	printComplete()
}
func Test_ApplyEntries(t *testing.T) {
	printHeader("Apply Entries")
	R := MockFollowerWithCM("S1    ", "localhost:20010")
	//R.run(nil)
	EA := R.applyEntries(R.LastApplied)
	if EA != 0 {
		t.Error("Entries applied incorrectly")
	}
	fmt.Println("Apply None Complete")

	R.Status = Leader
	R.Log = append(R.Log, LogEntry{Term: 1, Entry: "Append 1"})
	R.commitIndex = 1
	if EA := R.applyEntries(1); EA == 0 {
		t.Error("Entry Not Applied")
	}
	if R.LastApplied != 1 {
		t.Error("LastApplied not set correctly")
	}
	//try with multiple appends in same session
	R.Log = append(R.Log, LogEntry{Term: 1, Entry: "Append 2"})
	R.Log = append(R.Log, LogEntry{Term: 1, Entry: "Append 3"})
	R.Log = append(R.Log, LogEntry{Term: 1, Entry: "Append 4"})
	R.commitIndex = 4
	if EA := R.applyEntries(4); EA != 3 {
		t.Error(EA, "Entries applied. Expecting 3.")
	}
	if R.LastApplied != 4 {
		t.Error("LastApplied after batch was", R.LastApplied)
	}
	//try with a commitIndex that was set too high
	R.Log = append(R.Log, LogEntry{Term: 1, Entry: "Append 5"})
	R.commitIndex = 6
	if EA := R.applyEntries(5); EA != 1 {
		t.Error(EA, "Entries applied. Expecting 3.")
	}
	if R.LastApplied != 5 {
		t.Error("LastApplied after batch was", R.LastApplied)
	}
	printComplete()
}
func Test_AppendEntries(t *testing.T) {
	printHeader("Testing Append Entries")
	R, AE := setupAppend("Follo1")

	if R.leader != "S1    " {
		t.Error("Leader not set")
	}
	if AE.Reply.ID != "Follo1" {
		t.Error("Reply ID not set")
	}
	R.needWrite = false
	R.processAppend(AE) //this should be a normal heartbeat and not trigger needWrite

	if R.needWrite {
		t.Error("Write triggered on hearbeat")
	}

	//check if a heartbeat demotes
	R.Status = Leader
	R.leader = R.ID
	R.Term = 2
	R.processAppend(AE)
	if R.Status != Leader {
		t.Error("Leader incorrectly demoted")
	}
	R.Term = 1 //the terms are now equal, and demotion back to follower should occur
	R.processAppend(AE)
	if R.Status == Leader {
		t.Error("Leader not demoted")
	}
	AE.Term = 2
	R.needWrite = false
	R.processAppend(AE)
	if R.Term != 2 {
		t.Error("Term not set through append")
	}
	if !R.needWrite {
		t.Error("Write not triggered on term update")
	}

	///// reset
	R, AE = setupAppend("Follo1")
	AE.Entries = append(AE.Entries, LogEntry{Term: R.Term, Entry: "Entry 1"})
	R.processAppend(AE)
	if R.Log[R.commitIndex].Entry != "Entry 1" {
		t.Error("Entry not appended, or commit index incorrect")
	}
	if R.LastApplied != 0 {
		t.Error("Entry applied prematurely")
	}
	AE.Entries = nil
	AE.PrevLogIndex = 1
	AE.PrevLogTerm = 1
	AE.CommitIndex = 2
	AE.LastApplied = 1
	R.processAppend(AE)
	if R.LastApplied != 1 {
		t.Error("commit not applied. LastApplied was", R.LastApplied)
	}

	///// reset
	//sets up a leader that is ahead, and watches the log sync between them
	R = MockFollowerWithCM("Leader", "localhost:10020")
	R2 := MockFollowerWithCM("Follo1", "localhost:10021")
	R.addClient("Follo1")
	R2.addClient("Leader")
	R.Status = Leader
	R.commitIndex = 3
	R.LastApplied = 3
	R.Log = append(R.Log, LogEntry{Term: 1, Entry: "Entry 1"})
	R.Log = append(R.Log, LogEntry{Term: 1, Entry: "Entry 2"})
	R.Log = append(R.Log, LogEntry{Term: 1, Entry: "Entry 3"})

	AE = R.buildClientPayload(R2.ID)
	R.nextIndex[R2.ID] = len(R.Log)

	R2.processAppend(AE)
	R.processAppended(AE)
	AE = R.buildClientPayload("Follo1")
	R2.processAppend(AE)
	R.processAppended(AE)

	for i, entry := range R.Log {
		if R2.Log[i] != entry {
			t.Error("Entry did not match at index:", i)
		}
	}
	if R2.LastApplied != 3 {
		t.Error("entries not applied")
	}
	if R.nextIndex[R2.ID] != 4 {
		t.Error("next index not correct")
	}
	printComplete()
}
