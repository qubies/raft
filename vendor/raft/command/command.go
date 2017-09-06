package command

import (
	"bytes"
	"log"
	"raft/lockables"
	"raft/member"
	"strconv"
	"time"

	. "github.com/rthornton128/goncurses"
)

const (
	HEIGHT = 20
	WIDTH  = 20
)

func Begin(R *member.RaftMember, outputTo *bytes.Buffer, stop chan bool) {
	var active int
	var append int
	menu := []string{"View Output", "Print Score", "Print Status", "Print Log", "Append New Entry", "Suspend", "Resume", "Exit"}

	stdscr, err := Init()
	if err != nil {
		log.Fatal(err)
	}
	defer End()

	//color initialization
	if err := StartColor(); err != nil {
		log.Fatal(err)
	}
	if err := InitPair(1, C_RED, C_BLACK); err != nil {
		log.Fatal("InitPair failed: ", err)
	}
	if err := InitPair(2, C_YELLOW, C_BLACK); err != nil {
		log.Fatal("InitPair failed: ", err)
	}
	if err := InitPair(3, C_GREEN, C_BLACK); err != nil {
		log.Fatal("InitPair failed: ", err)
	}
	if err := InitPair(4, C_BLUE, C_WHITE); err != nil {
		log.Fatal("InitPair failed: ", err)
	}
	//end color

	Raw(true)
	Echo(false)
	Cursor(0)
	stdscr.Clear()
	stdscr.Keypad(true)
	_, mx := stdscr.MaxYX()
	y := 1
	x := 1
	win, _ := NewWindow(HEIGHT, WIDTH, y, x)
	win.Keypad(true)
	win2, _ := NewWindow(HEIGHT, mx-WIDTH-5, y, x+WIDTH+2)
	win2.ScrollOk(true)
	stdscr.Print("Use arrow keys to go up and down, Press enter to select")
	stdscr.Refresh()
	//printout the menu
	printmenu(win, menu, active)

	//setup the scroll printer
	var outputPrint lockables.Bool
	outputPrint.Set(false)
	win3, _ := NewWindow(HEIGHT-3, mx-WIDTH-7, y+2, x+WIDTH+2+1)
	win3.ScrollOk(true)
	go outputPrinter(outputTo, &outputPrint, win3)

	for {
		select {
		case <-stop:
			outputPrint.Set(false)
			close(stop)
			win3.Delete()
			win2.Delete()
			win.Delete()
			return
		default:
			ch := stdscr.GetChar()
			switch Key(ch) {
			case KEY_UP:
				if active == 0 {
					active = len(menu) - 1
				} else {
					active -= 1
				}
			case KEY_DOWN:
				if active == len(menu)-1 {
					active = 0
				} else {
					active += 1
				}
			case KEY_RETURN, KEY_ENTER, Key('\r'):
				switch active {
				case 0:
					//draw the border
					win2.Clear()
					win2.Box(0, 0)
					win2.Refresh()
					//set the locker
					outputPrint.Set(true)
				case 1:
					win2.Clear()
					win2.Box(0, 0)
					outputPrint.Set(false)
					win3.Clear()
					scoreBoard := R.GetScore()
					win3.ColorOn(4)
					for ID, Score := range scoreBoard {
						win3.Println(ID[:6], ":", Score)
					}
					win3.ColorOff(4)
					win2.Refresh()
					win3.Refresh()
				case 2:
					win2.Clear()
					win2.Box(0, 0)
					outputPrint.Set(false)
					NSB := R.GetStatus()
					win2.MovePrint(1, 2, "")
					win2.MovePrint(2, 2, "        ID: ", NSB.ID[:6])
					win2.MovePrint(3, 2, "   Address: ", NSB.Address)
					win2.MovePrint(4, 2, "    Status: ")
					if NSB.Status == member.Leader {
						printColor(4, 14, win2, 1, "Leader")
					} else if NSB.Status == member.Candidate {
						printColor(4, 14, win2, 2, "Candidate")
					} else if NSB.Status == member.Follower {
						printColor(4, 14, win2, 3, "Follower")
					}
					if NSB.Leader == "" {
						win2.MovePrint(5, 2, "    Leader: ", "NONE")
					} else {
						win2.MovePrint(5, 2, "    Leader: ", NSB.Leader[:6])
					}
					win2.MovePrint(6, 2, "      Term: ", NSB.Term)
					win2.MovePrint(7, 2, "Log Length: ", NSB.LogLength)
					win2.MovePrint(8, 2, "   Members: ")
					for i, x := range NSB.Members {
						win2.MovePrint(9+i, 14, x)
					}
					win2.Refresh()
				case 3:
					win2.Clear()
					win2.Box(0, 0)
					outputPrint.Set(false)
					win2.Refresh()
					win3.Clear()
					printColor(0, 0, win3, 4, R.GetLog())
					win3.Refresh()
				case 4:
					win2.Clear()
					win2.Box(0, 0)
					outputPrint.Set(false)
					R.Append(strconv.Itoa(append))
					printColor(1, 2, win2, 4, "Appending:"+strconv.Itoa(append))
					append++
					win2.Refresh()
				case 5:
					R.Suspend()
				case 6:
					R.Resume()
				case 7:
					outputPrint.Set(false)
					close(stop)
					win3.Delete()
					win2.Delete()
					win.Delete()
					return
				}
				if !outputPrint.Get() {
					stdscr.ClearToEOL()
					stdscr.Refresh()
				}
			}
			printmenu(win, menu, active)
		}
	}
}

func outputPrinter(outputTo *bytes.Buffer, shouldPrint *lockables.Bool, window *Window) {
	//scroller := make([]string, 15)
	for {
		theOutput, err := outputTo.ReadString('\n')
		if shouldPrint.Get() {
			window.Touch()
			if err == nil && theOutput != "" {
				window.Print(theOutput)
				window.Refresh()
			}
		}
		time.Sleep(time.Millisecond * 50)
	}
}

func printmenu(w *Window, menu []string, active int) {
	y, x := 2, 2
	w.Box(0, 0)
	for i, s := range menu {
		if i == active {
			w.AttrOn(A_REVERSE)
			w.MovePrint(y+i, x, s)
			w.AttrOff(A_REVERSE)
		} else {
			w.MovePrint(y+i, x, s)
		}
	}
	w.Refresh()
}

func printColor(y, x int, w *Window, colorNum int16, str string) {
	w.ColorOn(colorNum)
	w.MovePrint(y, x, str)
	w.ColorOff(colorNum)
}
