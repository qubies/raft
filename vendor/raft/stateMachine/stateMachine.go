package stateMachine

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"raft/terr"
	"strconv"
	"strings"
	"time"
)

//the StateMachine structure holds the critical data for the machine
type StateMachine struct {
	FilePath        string
	file            *os.File
	TargetNumber    int
	ScoreBoard      map[string]int
	ContentionLevel uint
}

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

//CreateStateMachine builds a new instance of the stateMachine on a filePath
func CreateStateMachine(FilePath string, ContentionLevel uint) *StateMachine {
	if ContentionLevel < 0 {
		ContentionLevel = 0
	} else if ContentionLevel > 60 {
		ContentionLevel = 60
	}
	var err error
	temp := StateMachine{FilePath: FilePath, TargetNumber: 0, ContentionLevel: ContentionLevel}
	temp.ScoreBoard = make(map[string]int)
	temp.file, err = os.OpenFile(temp.FilePath, os.O_RDWR|os.O_CREATE, 0755)

	terr.BreakError("state machine file open/create problem", err)
	scanner := bufio.NewScanner(temp.file)
	for scanner.Scan() {
		R := strings.Split(scanner.Text(), ",")
		if len(R) > 1 { // make sure there is a score with the split
			temp.ScoreBoard[R[0]]++ //increment the score for the server
			terr.BreakError("Error parsing TargetNumber", err)
		}
	}
	temp.TargetNumber = rand.Int() >> temp.ContentionLevel
	return &temp
}

//GuessNumber returns a boolean to indicate if the guess was successful
func (S *StateMachine) GuessNumber(guess int) bool {
	if guess != S.TargetNumber {
		return false
	}
	return true
}

func (S *StateMachine) GetNumber() int {
	return rand.Int() >> S.ContentionLevel
}
func (S *StateMachine) Guess() (int, bool) {
	thisInt := rand.Int() >> S.ContentionLevel
	return thisInt, S.GuessNumber(thisInt)
}

//commit guess checks again that the guess is successful, and then writes the guess into the file.
func (S *StateMachine) CommitGuess(guess int, ID string) error {
	S.ScoreBoard[ID]++ // increment the score
	S.file.WriteString(ID + "," + strconv.Itoa(guess) + "\n")
	S.TargetNumber = rand.Int() >> S.ContentionLevel // generate a new TargetNumber
	return nil
}

func (S *StateMachine) GetScore() map[string]int {
	S.PrintScore()
	return S.ScoreBoard
}

func (S *StateMachine) PrintScore() {
	for player, score := range S.ScoreBoard {
		fmt.Println("Player:", player, "Score:", score)
	}
}

//SetTarget is for a leader to commit its guessed results to a recipient.
//SetTarget deliberately allows injection of a result to the state machine.
func (S *StateMachine) SetTarget(target int) {
	S.TargetNumber = target
}
