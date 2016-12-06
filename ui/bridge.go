package main

import (
	"fmt"
	"os"
	"bufio"
	"strings"

	"net/rpc"
)


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
	fmt.Println("bitch\n")

	peers := get_peers("../peersfile.txt")

	var kuba_addr string

	for _, peer_str := range peers {
		peer := strings.Split(peer_str, ",")
		kuba_addr = peer[2]
	}

	

	client, err := rpc.DialHTTP("tcp", kuba_addr)
	
	reply := Calendar{2, make(map[int]Booking)}

	err = client.Call("CalendarHandler.GetCalendar", 0, &reply)
	handle_err(err)

	err = client.Close()
	handle_err(err)

	fmt.Println(reply)

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