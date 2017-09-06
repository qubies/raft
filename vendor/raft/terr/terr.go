package terr

/*
Generic error handlers.
*/
import (
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
)

var file *os.File
var mainLog string
var relPath string
var err error
var logger *log.Logger

func init() {
	relPath = ".raftTL"
	//look up for the file until it exists.
	for !FileExists(relPath) {
		relPath = "../" + relPath
	}
	var temp string
	temp, err = filepath.Abs(relPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Absolute File Path Creation", err)
	}
	dir := path.Dir(temp)
	mainLog = dir + "/logs/main.log"

	if !FileExists(dir + "/logs/") {
		os.Mkdir(dir+"/logs", 0744)
		fmt.Println("makein")
	}

	file, err = os.OpenFile(mainLog, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Creating initial log file", err)
	}
	logger = log.New(file, "log: ", log.Ldate|log.Ltime|log.Lmicroseconds|log.Llongfile)
	LogInfo("Log Started " + mainLog)
}

//LogInfo just writes some info to the log
func LogInfo(s string) {
	logger.Println(" INFO:", s)
}

//LogError Just writes the error to the logfile.
func LogError(service string, err error) {
	logger.Println("ERROR:", service+":", err)
}

//logPanic logs the error and then calls Panic.
func logPanic(service string, err error) {
	logger.Panicln("PANIC:", service+":", err)
}

//ClearLog clears and recreates the logfile
func ClearLog() {
	file.Close()
	os.Remove(mainLog)
	file, err = os.OpenFile(mainLog, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0755)
	PrintError("Deleting Log File", err)
	logger = log.New(file, "log: ", log.Ldate|log.Ltime|log.Lmicroseconds|log.Llongfile)
}

//BreakError panics, halting execution and reports an error
func BreakError(service string, err error) {
	if err != nil {
		fmt.Println(service, err.Error())
		logPanic(service, err)
	}
}

//PrintError does not halt execution, it just reports to std.err
func PrintError(service string, err error) {
	if err != nil {
		//fmt.Println("ErrorTest ThisError")
		fmt.Println(service, err.Error())
		LogError(service, err)
	}
}

//DidPanic is a testing function that expects code to panic and handles around it
func DidPanic(f func()) bool {
	success := true
	func() {
		defer func() {
			if r := recover(); r == nil {
				success = false
			}
		}()
		f()
	}()
	return success
}

//FileExists checks if the given file path exists.
func FileExists(path string) bool {
	if _, err := os.Stat(path); err == nil {
		return true
	}
	return false
}

//VerbPrint is a verbose level printing system, 0 will always print, higher levels are set through the program.
func VerbPrint(outputTo io.Writer, level, verb int, iface ...interface{}) {
	if level == 0 || level <= verb {
		fmt.Fprintln(outputTo, iface...)
	}
}
