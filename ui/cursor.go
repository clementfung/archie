package main

import (
	"fmt"
	"strconv"
)

func main() {
	fmt.Printf("hi\n")

	fmt.Printf("\033[s") // save cursor location

	move_cursor(2, 2)

	fmt.Printf("hdingleberrk\n")

	fmt.Printf("\033[u") // restore cursor location

	fmt.Printf("bye\n")



}

func move_cursor(row int, col int) {
	row_str := strconv.Itoa(row)
	col_str := strconv.Itoa(col)
	fmt.Printf("\033[" + row_str + ";" + col_str + "H") 
}
