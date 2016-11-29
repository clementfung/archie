package main

import (
	"fmt"
	"strings"
	"strconv"

	"os" 
	"os/exec"

	"time"
	"math/rand" // tests

)

const reset = "\033[m"

func main() {



	// init

	// seed a RNG with current time in nanoseconds
	s1 := rand.NewSource(time.Now().UnixNano())
    r1 := rand.New(s1)

    var cal []string

    // generate some random slots
	for i := 0; i < 8; i++ {
		var state string
		coinflip := r1.Float64()

		if coinflip < 0.1 {
			state = "R"
		} else if coinflip < 0.3 {
			state = "M"
		} else if coinflip < 0.6 {
			state = "B"
		} else {
			state = "A"
		}

		cal = append(cal, state)
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

    // how to reset after???






	selected_slot := 0



	// end init


	// draw
	draw_slots(0, selected_slot, cal)

	key_chan := make(chan string)

	go handle_keys(key_chan)

	for {
		select {
		case key := <- key_chan :

			switch key {
			case "up" :
				if selected_slot > 0 {
					selected_slot--
					draw_slots(0, selected_slot, cal)
				}
			case "down" :
				if selected_slot < (len(cal) - 1) {
					selected_slot++
					draw_slots(0, selected_slot, cal)
				}
			}


		}
	}

}

func draw_slots(start_row int, selected_slot int, cal []string) {
	move_cursor(start_row, 0)

	for i := 0; i < 8; i++ {
		draw_slot(0, 0, i, cal[i], selected_slot == i)
	}
}

func draw_slot(row int, col int, time int, state string, selected bool) {

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
		fmt.Printf("      " + esc("1", fg, bg) + "╭─────────────┒" + reset + "\n")
		fmt.Printf("  " + esc("1") + time_str + " ▶" + esc(fg, bg) + "│  " + label + "┃" + reset + "◀" + "\n")
		fmt.Printf("      " + esc("1", fg, bg) + "┕━━━━━━━━━━━━━┛" + reset + "\n")

	} else {
		fmt.Printf("      " + esc("1", fg, bg) + "╭─────────────╮" + reset + "\n")
		fmt.Printf("  " + time_str + "  " + esc("1", fg, bg) + "│  " + label + "│" + reset + "  \n")
		fmt.Printf("      " + esc("1", fg, bg) + "╰─────────────╯" + reset + "\n")
	}


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

    }

}