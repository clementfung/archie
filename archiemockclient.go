package main

import (
  "fmt"
  "os"
  "net/rpc"
  "strconv"
  "strings"
)

// From the UI to the proposer
type UserPropose struct {
  Attendees []string
  TimeSlots []int
}

func check(err error) {
  if err != nil {
    panic(err)
  }
}

/*************
  START OF MAIN METHOD
***************/
func main() {

    fmt.Println("Calling.....")

    address := os.Args[1]
    peers := os.Args[2]
    timeRange := os.Args[3]
  
    peerSplit := strings.Split(peers, ",")
    timeSplit := strings.Split(timeRange, ",")

    minTime, err := strconv.Atoi(timeSplit[0])
    check(err)

    maxTime, err := strconv.Atoi(timeSplit[1])
    check(err)

    timeslots := make([]int, 0)

    for i := minTime; i <= maxTime; i++ {
      timeslots = append(timeslots, i)
    }

    client, err := rpc.DialHTTP("tcp", address)
    args := UserPropose{peerSplit, timeslots}
    reply := 0
    err = client.Call("CalendarHandler.UserPropose", args, &reply)
    check(err)

    err = client.Close()
    check(err)

}