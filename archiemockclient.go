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
  MeetingID string
  Attendees []string
  TimeSlots []int
}

type UserBusy struct {
  Time int
}

type UserCancel struct {
  Time int
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

    commandFlag := os.Args[1]

    if commandFlag == "-p" {
      
      address := os.Args[2]
      peers := os.Args[3]
      timeRange := os.Args[4]
      meetingID := os.Args[5]
    
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
      args := UserPropose{meetingID, peerSplit, timeslots}
      reply := 0
      err = client.Call("CalendarHandler.UserPropose", args, &reply)
      check(err)

      err = client.Close()
      check(err)

    } else if commandFlag == "-b" {

      address := os.Args[2]
      time := os.Args[3]

      intTime, err := strconv.Atoi(time)
      check(err)

      client, err := rpc.DialHTTP("tcp", address)
      args := UserBusy{intTime}
      reply := 0
      err = client.Call("CalendarHandler.UserBusy", args, &reply)
      check(err)

      err = client.Close()
      check(err)

    } else if commandFlag == "-c" {

      address := os.Args[2]
      time := os.Args[3]

      intTime, err := strconv.Atoi(time)
      check(err)

      client, err := rpc.DialHTTP("tcp", address)
      args := UserCancel{intTime}
      reply := 0
      err = client.Call("CalendarHandler.UserCancel", args, &reply)
      check(err)

      err = client.Close()
      check(err)

    }

}