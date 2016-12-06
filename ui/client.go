package main

import (
	"fmt"
	"bufio"

	"strings"
	"strconv"

	"os" 
	"os/exec"

	"time"
	//"math/rand" // tests

	"net"
  	"net/rpc"
  	"net/http"

)

const RESET = "\033[m"
const SCROLL_TICK = 35 //ms for updating scroll

type Calendar struct {
	Owner int
	Slots map[int]Booking
}

type Booking struct {
	Status string // A, M, B, R
	MeetingID string
	ProposerID int // node that originated the request
	Attendees []int
	Alternates []int
}

// From the UI to the proposer
type UserPropose struct {
	MeetingID string
	Attendees []int
	MinTime int
	MaxTime int
}

/* A handler for calendars */
type ClientHandler struct {
	MyCalendar Calendar
	GotFirstPush bool
	draw_chan chan int
}

func main() {

	// init

    node_num := os.Args[1]
    peersfile := os.Args[2]

    my_name, server_addr := get_my_info(node_num, peersfile)
    client_addr := increment_addr(server_addr)


    // set up push endpoint for server
    // server will update local calendar with UpdateClient

    clientHandler := ClientHandler{}

   	clientHandler.GotFirstPush = false
   	clientHandler.draw_chan = make(chan int)

    rpc.Register(&clientHandler)

    // Export the RPC functions
    rpc.HandleHTTP()
    listener,err := net.Listen("tcp", client_addr)
    handle_err(err)
    go http.Serve(listener, nil)


	screen_clear()

	fmt.Printf("I am the client for %v\n!", my_name)
    fmt.Printf("My server address is: %v\n", server_addr)
    fmt.Printf("My client address is: %v\n", client_addr)

	fmt.Printf("Waiting on server to push calendar for the first time...\n")

	for !clientHandler.GotFirstPush {
		// only proceed once server has pushed calendar
	}

	screen_clear()


	rows, cols := screen_size()

	// need to constrain rows to workable size for scroll

	// i.e. subtract rows for header, footer, etc.
	

	if rows + cols == 0 {

	}

	// disable input buffering
    exec.Command("stty", "-F", "/dev/tty", "cbreak", "min", "1").Run()
    // do not display entered characters on the screen
    exec.Command("stty", "-F", "/dev/tty", "-echo").Run()

    // hide cursor (linux only)
	fmt.Printf("\033[?25l")

    // how to RESET after???



	selected_slot := 0
	scroll_row := 0 // highest row we see

	// end init

	//fmt.Printf("%v", rows)


	max_row := len(clientHandler.MyCalendar.Slots) * 3


	// meeting propose

	propose_ui := false
	var my_proposal UserPropose


	go draw (clientHandler.draw_chan, &scroll_row, &selected_slot, &clientHandler.MyCalendar, &propose_ui, &my_proposal, &rows, &cols)
	go scroller(clientHandler.draw_chan, &scroll_row, &selected_slot, rows, max_row)


	key_chan := make(chan string)
	go handle_keys(key_chan)

	for {
		select {
		case key := <- key_chan :
			state := clientHandler.MyCalendar.Slots[selected_slot].Status

			if propose_ui {

				switch key {
				case "up" :
					if selected_slot > my_proposal.MinTime {
						selected_slot--
						my_proposal.MaxTime--
						clientHandler.draw_chan <- 1
					}
				case "down" :
					if selected_slot < (len(clientHandler.MyCalendar.Slots) - 1) {
						selected_slot++
						my_proposal.MaxTime++
						clientHandler.draw_chan <- 1
					}
				case "q" : // begin propose
					propose_ui = false
					draw_chan <- 1
				}

			} else {

				switch key {
				case "up" :
					if selected_slot > 0 {
						selected_slot--
						clientHandler.draw_chan <- 1
					}
				case "down" :
					if selected_slot < (len(clientHandler.MyCalendar.Slots) - 1) {
						selected_slot++
						clientHandler.draw_chan <- 1
					}


				case "t" : // toggle "A"/"B"
				
					client, err := rpc.DialHTTP("tcp", server_addr)
					reply := 0

					if state == "A" {

						err = client.Call("CalendarHandler.UserBusy", selected_slot, &reply)
						handle_err(err)

					} else if state == "B" {

						err = client.Call("CalendarHandler.UserAvailable", selected_slot, &reply)
						handle_err(err)
					}

					err = client.Close()
					handle_err(err)

				case "m" : // begin propose
					if state == "A" {

						propose_ui = true
						my_proposal = UserPropose{}
						my_proposal.MinTime = selected_slot
						my_proposal.MaxTime = selected_slot

						draw_chan <- 1

					}
				}
				
			}


		}
	}

}

func (t *ClientHandler) UpdateClient(input Calendar, reply *int) error {
	t.MyCalendar = input 
	*reply = 1

	if !t.GotFirstPush {
		t.GotFirstPush = true
	}

	t.draw_chan <- 1 // tell client to draw

	return nil
}

func get_my_info(node_str string, peersfile string) (string, string) {
	// return an address with port 1 greater than the server's port

	peers := get_peers(peersfile)

	node, err := strconv.Atoi(node_str)
	handle_err(err)

	for i, peer_str := range peers {
		if i == node {
			peer := strings.Split(peer_str, ",")
			return peer[1], peer[2]
		}
	}
	
	return "",""
}

func increment_addr(addr_str string) string {
	addr := strings.Split(addr_str, ":")
	port, err := strconv.Atoi(addr[1])
	handle_err(err)

	port += 1

	addr[1] = strconv.Itoa(port)

	return strings.Join(addr, ":")
}



func draw(draw_chan chan int, scroll_row *int, selected_slot *int, cal *Calendar, propose_ui *bool, my_proposal *UserPropose, rows *int, cols *int) {
	for {
		select {
		case <- draw_chan :
			draw_slots(propose_ui, my_proposal, *scroll_row, *selected_slot, *cal, *rows)
			draw_sidebar(propose_ui, my_proposal, selected_slot, cal, *rows, *cols)

		}

	}
}

func draw_sidebar(propose_ui *bool, my_proposal *UserPropose, selected_slot *int, cal *Calendar, rows int, cols int) {

	// requires at least 30+ cols?

	if cols < 40 {
		return
	}

	sidebar_col := 28 // col sidebar starts on
	sidebar_width := cols - sidebar_col

	hor_border := strings.Repeat("┄", sidebar_width - 2)
	hor_space  := strings.Repeat(" ", sidebar_width - 2)
	hor_fill   := strings.Repeat("▒", sidebar_width - 2)

	// height of infobox is larger of 12, rows/3
	infobox_height := max(12, rows/3)


	var label string
	//var time_str string
	var bg string
	var fg string
	state := cal.Slots[*selected_slot].Status

	switch state {
	case "A" :
		label = "AVAILABLE  "
		bg = bgcolor("K")
		fg = fgcolor("G")

	case "B" :
		label = "BUSY       "
		bg = bgcolor("K")
		fg = fgcolor("R")

	case "R" :
		label = "RESERVED   "
		bg = bgcolor("K")
		fg = fgcolor("Y")

	case "M" :
		label = "MEETING    "
		bg = bgcolor("K")
		fg = fgcolor("C")

	}


	fmt.Printf(esc("1", bg, fg))

	move_cursor(1, sidebar_col)
	fmt.Printf("╓" + hor_border + "┄" + "\n")

	for i := 2; i < infobox_height; i++ {
		move_cursor(i, sidebar_col)
		fmt.Printf("║" + hor_space + "▒" + "\n")
	}

	move_cursor(infobox_height, sidebar_col)
	fmt.Printf("║" + hor_fill + "▒" + "\n")

/*
	move_cursor(2, sidebar_col + 3)
	fmt.Printf(" █████╗ ██╗   ██╗ █████╗ ██╗██╗      █████╗ ██████╗ ██╗     ███████╗")
	move_cursor(3, sidebar_col + 3)
	fmt.Printf("██╔══██╗██║   ██║██╔══██╗██║██║     ██╔══██╗██╔══██╗██║     ██╔════╝")
	move_cursor(4, sidebar_col + 3)
	fmt.Printf("███████║██║   ██║███████║██║██║     ███████║██████╔╝██║     █████╗  ")
	move_cursor(5, sidebar_col + 3)
	fmt.Printf("██╔══██║╚██╗ ██╔╝██╔══██║██║██║     ██╔══██║██╔══██╗██║     ██╔══╝  ")
	move_cursor(6, sidebar_col + 3)
	fmt.Printf("██║  ██║ ╚████╔╝ ██║  ██║██║███████╗██║  ██║██████╔╝███████╗███████╗")
	move_cursor(7, sidebar_col + 3)
	fmt.Printf("╚═╝  ╚═╝  ╚═══╝  ╚═╝  ╚═╝╚═╝╚══════╝╚═╝  ╚═╝╚═════╝ ╚══════╝╚══════╝")
*/

	move_cursor(2, sidebar_col + 3)
	fmt.Printf(label)


	fmt.Printf(RESET)


	if *propose_ui {
		move_cursor(infobox_height + 3, sidebar_col + 5)
		fmt.Printf("q : quit meeting proposal")

		move_cursor(infobox_height + 5, sidebar_col + 5)
		fmt.Printf("enter : book meeting between %v and %v   ", my_proposal.MinTime, my_proposal.MaxTime)



	} else {

		move_cursor(infobox_height + 3, sidebar_col + 5)

		switch state {
		case "A" :
			fmt.Printf("t : toggle available/busy")
		case "B" :
			fmt.Printf("t : toggle available/busy")
		default :
			fmt.Printf("                         ")
		}

		// if available
		// press m to propose

		move_cursor(infobox_height + 5, sidebar_col + 5)
		switch state {
		case "A" :
			fmt.Printf("m : schedule a meeting                ")
		default :
			fmt.Printf("                                      ")
		}

	}


}

// handles scrolling
func scroller(draw_chan chan int, scroll_row *int, selected_slot *int, rows int, max_row int) {


	c := time.Tick(SCROLL_TICK * time.Millisecond)

	for _ = range c {
		// if the calendar takes up the whole window, then
		// the ideal location for the selected slot
		// is in between 1/3 and 2/3 of the screen
		ideal_top := *scroll_row + rows*1/3
		ideal_bot := *scroll_row + rows*2/3

		selected_row := *selected_slot * 3



		if selected_row < ideal_top && *scroll_row > 0 {
			// if our selected slot is in the top third
			// scroll UP if we can

			*scroll_row -= 1
			draw_chan <- 1 // draw

		} else if selected_row > ideal_bot && (*scroll_row + rows) < max_row {
			// if our selected slot is in the bot third
			// scroll DOWN if we can

			*scroll_row += 1
			draw_chan <- 1 // draw

		}

	}


}

func draw_slots(propose_ui *bool, my_proposal *UserPropose, scroll_row int, selected_slot int, cal Calendar, rows int) {
	move_cursor(1, 1) // change for header

	// current slot
	slot_i := scroll_row / 3

	remaining_rows := rows

	min_slot := my_proposal.MinTime

	// draw the cutoff first slot if needed
	switch scroll_row % 3 {
	case 1 :
		// need to draw last two rows of first slot
		draw_slot(slot_i, cal.Slots[slot_i].Status, selected_slot, min_slot, *propose_ui, []int{1, 2}, true)
		remaining_rows -= 2
		slot_i++

	case 2 :
		// need to draw last row of first slot
		draw_slot(slot_i, cal.Slots[slot_i].Status, selected_slot, min_slot, *propose_ui, []int{2}, true)
		remaining_rows -= 1
		slot_i++
	}

	

	// maximum number of slots that will fit
	max_slots := min(remaining_rows / 3, len(cal.Slots) - slot_i)
	//debug("max slots: %v", max_slots)

	// draw a bunch of full slots
	for j := 0; j < max_slots - 1; j++ {
		draw_slot(slot_i, cal.Slots[slot_i].Status, selected_slot, min_slot, *propose_ui, []int{0, 1, 2}, true)
		remaining_rows -= 3
		slot_i++
	}

	// draw the last full slot:
	//   if remaining_rows % 3 == 0
	//   then this fits on to the last line, don't print a new line
	//   if not, then print a new line
	draw_slot(slot_i, cal.Slots[slot_i].Status, selected_slot, min_slot, *propose_ui, []int{0, 1, 2}, remaining_rows % 3 != 0)
	remaining_rows -= 3
	slot_i++


	// draw the final cutoff slot if needed

	if slot_i < len(cal.Slots) {

		switch remaining_rows {
		case 1 :
			// can draw one row of last slot
			draw_slot(slot_i, cal.Slots[slot_i].Status, selected_slot, min_slot, *propose_ui, []int{0}, false)

		case 2 :
			// can draw two rows of last slot
			draw_slot(slot_i, cal.Slots[slot_i].Status, selected_slot, min_slot, *propose_ui, []int{0, 1}, false)

		}
	}


}

func draw_slot(time int, state string, selected_slot int, min_slot int, propose_ui bool, visible_rows []int, new_line bool) {

	var label string
	var time_str string
	var bg string
	var fg string

	time_str = strconv.Itoa(time)

	// prepend 0 to timeslots 00 - 09
	if time < 10 {
		time_str = "0" + time_str
	}

	switch state {
	case "A" :
		label = "AVAILABLE  "
		bg = bgcolor("G")
		fg = fgcolor("G")

	case "B" :
		label = "BUSY       "
		bg = bgcolor("R")
		fg = fgcolor("R")

	case "R" :
		label = "RESERVED   "
		bg = bgcolor("Y")
		fg = fgcolor("Y")

	case "M" :
		label = "MEETING    "
		bg = bgcolor("C")
		fg = fgcolor("C")

	}

	if propose_ui {
		if min_slot <= time && time <= selected_slot {
			fg = fgcolor("Y")

			if state == "A" { // also colour bg if it's available
				bg = bgcolor("Y")
			}
		}
	} else if time == selected_slot {
		fg = fgcolor("W")
	}

	for i, row := range visible_rows {
		if i < len(visible_rows) - 1 {
			draw_slot_row(row, fg, bg, time_str, label, time == selected_slot, time == min_slot, propose_ui, true)
		} else {
			draw_slot_row(row, fg, bg, time_str, label, time == selected_slot, time == min_slot, propose_ui, new_line)
		}

	}
}

func draw_slot_row(row int, fg string, bg string, time_str string, label string, selected bool, is_min bool, propose_ui bool, new_line bool) {
	
	if selected {
		switch row {
		case 0 :
			fmt.Printf("      " + esc("1", fg, bg) + "╭─────────────┒" + RESET + "  ")
		case 1 :

			if propose_ui {
				if is_min {
					fmt.Printf("  " + esc("1") + time_str + esc(fg) + " ▶" + esc(bg) + "│  " + label + "┃" + RESET + esc("1", fg) + "◀" + RESET)
				} else {
					fmt.Printf("  " + esc("1") + time_str + "  " + esc(fg, bg) + "│  " + label + "┃" + RESET + esc("1", fg) + "◀" + RESET)
				}	

			} else {
				fmt.Printf("  " + esc("1") + time_str + " ▶" + esc(fg, bg) + "│  " + label + "┃" + RESET + "◀")
			}


		case 2 :
			fmt.Printf("      " + esc("1", fg, bg) + "┕━━━━━━━━━━━━━┛" + RESET + "  ")
		}

	} else {
		switch row {
		case 0 :
			fmt.Printf("      " + esc("1", fg, bg) + "╭─────────────╮" + RESET + "  ")
		case 1 :
			
			if propose_ui && is_min {
				fmt.Printf("  " + time_str + esc(fg) + " ▶" + esc("1", bg) + "│  " + label + "│" + RESET + "  ")
			} else {
				fmt.Printf("  " + time_str + "  " + esc("1", fg, bg) + "│  " + label + "│" + RESET + "  ")
			}


		case 2 :
			fmt.Printf("      " + esc("1", fg, bg) + "╰─────────────╯" + RESET + "  ")
		}

	}

	if new_line {
		fmt.Printf ("\n")
	}
}

func min(a int, b int) int {
	if a <= b {
		return a
	}

	return b
}

func max(a int, b int) int {
	if a >= b {
		return a
	}

	return b
}

func screen_size() (int, int) {
	cmd := exec.Command("stty", "size")
  	cmd.Stdin = os.Stdin
  	out, _ := cmd.Output()

  	out_str := strings.TrimSpace(string(out))

  	space_index := strings.Index(out_str, " ")

  	rows, _ := strconv.Atoi(out_str[:space_index])
  	cols, _ := strconv.Atoi(out_str[space_index + 1:])

  	return rows, cols
}

func screen_clear() {
	cmd := exec.Command("clear") //linux
	cmd.Stdout = os.Stdout
	cmd.Run()
}


func move_cursor(row int, col int) {
	row_str := strconv.Itoa(row)
	col_str := strconv.Itoa(col)
	fmt.Printf("\033[" + row_str + ";" + col_str + "H") 
}

func esc(options ...string) string {

	options_str := strings.Join(options, ";")

	return "\033[" + options_str + "m"
}

func fgcolor(c string) string {
	return "3" + color(c)
}

func bgcolor(c string) string {
	return "4" + color(c)
}

func color(c string) string {
	switch c {
	case "K" :
		return "0"
	case "R" :
		return "1"
	case "G" :
		return "2"
	case "Y" :
		return "3"
	case "B" :
		return "4"
	case "M" :
		return "5" 
	case "C" :
		return "6" 
	case "W" :
		return "7"
	}

	return ""
}

func handle_keys(key_chan chan string) {

    var b []byte = make([]byte, 1)
    for {
        os.Stdin.Read(b)
        //fmt.Println("\nI got the byte", b, "("+string(b)+")")
        //fmt.Println("also: "+ string([]byte("27")))


	    // 27, 91
	    //  U: 65 
	    //  D: 66
	    //  R: 67
	    //  L: 68

        if b[0] == 27 {
            os.Stdin.Read(b)
            if b[0] == 91 {
                os.Stdin.Read(b)
                switch b[0] {
                case 65 :
                    key_chan <- "up"
                    continue
                case 66 :
                    key_chan <- "down"
                    continue
                case 67 :
                    key_chan <- "right"
                    continue
                case 68 :
                    key_chan <- "left"
                    continue
                }
                
            }
        }



        if b[0] >= 65 && b[0] <= 90 {
        	key := string(b[0] + 32)
        	key_chan <- key
        	continue
        }

    	if b[0] >= 97 && b[0] <= 122 {
        	key := string(b[0])
        	key_chan <- key
    		continue
    	}

    }

}


// debug, print str to very bottom
func debug(format string, args ...interface{}) {
	fmt.Printf("\033[s") // save cursor location

	rows, _ := screen_size()

	move_cursor(rows - 2, 1)

	fmt.Printf(format, args...)

	fmt.Printf("\033[u") // restore cursor location
}

// read a file by lines
func get_peers(peersfile string) []string {
    file, err := os.Open(peersfile)
    if err != nil {
        //
    }
    defer file.Close()

    scanner := bufio.NewScanner(file)
    peers := make([]string, 0)
    for scanner.Scan() {
        peers = append(peers, scanner.Text())
    }

    return peers

}

// handles an error by printing it
func handle_err(err error) {
    if err != nil {
        panic(err)
    }
}





