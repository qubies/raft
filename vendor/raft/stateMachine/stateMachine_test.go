package stateMachine

import (
	"fmt"
	"os"
	"testing"
)

var testMachine *StateMachine

func Test_InitialScore(t *testing.T) {
	pwd, _ := os.Getwd()
	testMachine = CreateStateMachine(pwd+"/stateMachine.sm", 48)
	fmt.Println("Intial Scores:")
	testMachine.GetScore()
}
func Test_StateMachine(t *testing.T) {
	if testMachine.GuessNumber(-1) {
		t.Error("Negative Guess Issues")
	}
	if testMachine.GuessNumber(0) {
		t.Error("0 Guess successful, state machine empty?")
	}
	ID1 := "1D76BBE2847"
	ID2 := "29D93EB939F"
	for i := 0; i < 100000; i++ {
		thisInt, resp := testMachine.Guess()
		if resp {
			testMachine.CommitGuess(thisInt, ID1)
		}
		thisInt, resp = testMachine.Guess()
		if resp {
			testMachine.CommitGuess(thisInt, ID2)
		}
	}
	fmt.Println("Final Scores:")
	testMachine.GetScore()
}
