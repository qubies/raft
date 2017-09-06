package terr

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"testing"
)

func Test_Logging(t *testing.T) {
	LogInfo("Welcome to testing!")
	LogError("Creating a new Log error", errors.New("This is an error to log"))
	if !DidPanic(func() { logPanic("PANIC TEST", errors.New("This is a panic Error")) }) {
		t.Error("Logger did not panic")
	}
	ClearLog()
	LogInfo("Welcome to testing!")
}

func Test_terr(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("The code did not panic")
		}
	}()
	BreakError("ErrorTest", errors.New("ThisError"))
}

func Example_err() {
	PrintError("ErrorTest", errors.New("ThisError"))
}

func Test_DidPanic(t *testing.T) {
	if !DidPanic(func() {
		panic("o noes!")
	}) {
		t.Error("Code did not panic")
	}
	if DidPanic(func() {
		fmt.Println("hello!")
	}) {
		t.Error("shouldnt have paniced")
	}
}

//PrintFile reads in a file from a path and dumps it to stdout
func PrintFile(path string) {
	// open input file
	fi, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	// close fi on exit and check for its returned error
	defer func() {
		if err := fi.Close(); err != nil {
			panic(err)
		}
	}()
	// make a read buffer
	r := bufio.NewReader(fi)

	// make a buffer to keep chunks that are read
	buf := make([]byte, 1024)
	for {
		// read a chunk
		n, err := r.Read(buf)
		if err != nil && err != io.EOF {
			panic(err)
		}
		if n == 0 {
			break
		}

		// write a chunk
		fmt.Println(string(buf[:n]))
	}
}
