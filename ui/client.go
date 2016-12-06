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


  	"net/rpc"

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

func main() {



	// init

	/*
	// seed a RNG with current time in nanoseconds
	s1 := rand.NewSource(time.Now().UnixNano())
    r1 := rand.New(s1)

    
    
    slots := make(map[int]Booking)
    
    // generate some random slots
	for i := 0; i < 24; i++ {
		coinflip := r1.Float64()

		if coinflip < 0.3 {
			slots[i] = Booking{"M", "surprise party", 2, []int{0, 1, 2}, make([]int, 0)}
		} else if coinflip < 0.6 {
			slots[i] = Booking{"B", "", -1, make([]int, 0), make([]int, 0)}
		} else {
			slots[i] = Booking{"A", "", -1, make([]int, 0), make([]int, 0)}
		}
	}

	// kuba's calendar
    cal := Calendar{2, slots}
	*/

    node_num := os.Args[1]
    peersfile := os.Args[2]

    cal := get_cal(node_num, peersfile)




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

	max_row := len(cal.Slots) * 3



	draw_chan := make(chan int)
	go draw (draw_chan, &scroll_row, &selected_slot, &cal, &rows, &cols)
	go scroller(draw_chan, &scroll_row, &selected_slot, rows, max_row)



	// draw
	draw_chan <- 1

	key_chan := make(chan string)
	go handle_keys(key_chan)

	for {
		select {
		case key := <- key_chan :

			switch key {
			case "up" :
				if selected_slot > 0 {
					selected_slot--
					draw_chan <- 1
				}
			case "down" :
				if selected_slot < (len(cal.Slots) - 1) {
					selected_slot++
					draw_chan <- 1
				}


			case "b" : // toggle "A"/"B"
				state := cal.Slots[selected_slot].Status

				if state == "A" || state == "B" {




				}
				
			}


		}
	}

}

func get_cal(node_str string, peersfile string) Calendar {
	peers := get_peers(peersfile)

	var my_server_addr string
	node, err := strconv.Atoi(node_str)
	handle_err(err)

	for i, peer_str := range peers {
		if i == node {
			peer := strings.Split(peer_str, ",")
			my_server_addr = peer[2]
		}
	}
	

	client, err := rpc.DialHTTP("tcp", my_server_addr)
	
	reply := Calendar{node, make(map[int]Booking)}

	err = client.Call("CalendarHandler.GetCalendar", 0, &reply)
	handle_err(err)

	err = client.Close()
	handle_err(err)

	return reply
}



func draw(draw_chan chan int, scroll_row *int, selected_slot *int, cal *Calendar, rows *int, cols *int) {
	for {
		select {
		case <- draw_chan :
			draw_slots(*scroll_row, *selected_slot, *cal, *rows)
			draw_sidebar(selected_slot, cal, *rows, *cols)

		}

	}
}

func draw_sidebar(selected_slot *int, cal *Calendar, rows int, cols int) {

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

	if label == "" {

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


	// if available/busy
	// press b to toggle

	move_cursor(infobox_height + 3, sidebar_col + 5)

	switch state {
	case "A" :
		fmt.Printf("b : toggle available/busy")
	case "B" :
		fmt.Printf("b : toggle available/busy")
	default :
		fmt.Printf("                         ")
	}



	// 






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

func draw_slots(scroll_row int, selected_slot int, cal Calendar, rows int) {
	move_cursor(1, 1) // change for header

	// current slot
	slot_i := scroll_row / 3

	remaining_rows := rows


	// draw the cutoff first slot if needed
	switch scroll_row % 3 {
	case 1 :
		// need to draw last two rows of first slot
		draw_slot(slot_i, cal.Slots[slot_i].Status, selected_slot == slot_i, []int{1, 2}, true)
		remaining_rows -= 2
		slot_i++

	case 2 :
		// need to draw last row of first slot
		draw_slot(slot_i, cal.Slots[slot_i].Status, selected_slot == slot_i, []int{2}, true)
		remaining_rows -= 1
		slot_i++
	}

	

	// maximum number of slots that will fit
	max_slots := min(remaining_rows / 3, len(cal.Slots) - slot_i)
	//debug("max slots: %v", max_slots)

	// draw a bunch of full slots
	for j := 0; j < max_slots - 1; j++ {
		draw_slot(slot_i, cal.Slots[slot_i].Status, selected_slot == slot_i, []int{0, 1, 2}, true)
		remaining_rows -= 3
		slot_i++
	}

	// draw the last full slot:
	//   if remaining_rows % 3 == 0
	//   then this fits on to the last line, don't print a new line
	//   if not, then print a new line
	draw_slot(slot_i, cal.Slots[slot_i].Status, selected_slot == slot_i, []int{0, 1, 2}, remaining_rows % 3 != 0)
	remaining_rows -= 3
	slot_i++


	// draw the final cutoff slot if needed

	if slot_i < len(cal.Slots) {

		switch remaining_rows {
		case 1 :
			// can draw one row of last slot
			draw_slot(slot_i, cal.Slots[slot_i].Status, selected_slot == slot_i, []int{0}, false)

		case 2 :
			// can draw two rows of last slot
			draw_slot(slot_i, cal.Slots[slot_i].Status, selected_slot == slot_i, []int{0, 1}, false)

		}
	}


}

func draw_slot(time int, state string, selected bool, visible_rows []int, new_line bool) {

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

	if selected {
		fg = fgcolor("W")
	}

	for i, row := range visible_rows {
		if i < len(visible_rows) - 1 {
			draw_slot_row(row, fg, bg, time_str, label, selected, true)
		} else {
			draw_slot_row(row, fg, bg, time_str, label, selected, new_line)
		}

	}
}

func draw_slot_row(row int, fg string, bg string, time_str string, label string, selected bool, new_line bool) {
	
	if selected {
		switch row {
		case 0 :
			fmt.Printf("      " + esc("1", fg, bg) + "╭─────────────┒" + RESET + "  ")
		case 1 :
			fmt.Printf("  " + esc("1") + time_str + " ▶" + esc(fg, bg) + "│  " + label + "┃" + RESET + "◀")
		case 2 :
			fmt.Printf("      " + esc("1", fg, bg) + "┕━━━━━━━━━━━━━┛" + RESET + "  ")
		}

	} else {
		switch row {
		case 0 :
			fmt.Printf("      " + esc("1", fg, bg) + "╭─────────────╮" + RESET + "  ")
		case 1 :
			fmt.Printf("  " + time_str + "  " + esc("1", fg, bg) + "│  " + label + "│" + RESET + "  ")
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
                case 66 :
                    key_chan <- "down"
                case 67 :
                    key_chan <- "right"
                case 68 :
                    key_chan <- "left"
                }
                
            }
        }

        key := string(b[0])

        if key == "b" || key == "B" {
        	key_chan <- "b"
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