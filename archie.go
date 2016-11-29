package main

import (
  "fmt"
  "os"
  "net"
  "net/http"
  "net/rpc"
  "time"
  "strconv"
  "strings"
  "bufio"
  "errors"
  "github.com/arcaneiceman/GoVector/govec"
  "bitbucket.org/bestchai/dinv/dinvRT"
)

/*
  ARCHIE
  by
  Clement Fung
  Amon Ge
*/

/* A Handler for the HEARTBEAT messages */
type HeartbeatHandler struct {
  Logger *govec.GoLog
  Peers []Peer
}

type Peer struct {
  name string
  address string
  active *bool
}

type Heartbeat struct {
  Buffer []byte
  SourceName string
  SourceAddress string
}

func (t *HeartbeatHandler) Beat(req Heartbeat, reply *int) error {  
  fmt.Println("Received BEAT from " + req.SourceName)
  *reply = 1
  return nil
}

/* A handler for calendars */
type CalendarHandler struct {
  Logger *govec.GoLog
  SelfID string
  AddressLookup map[string]string
  MyMeeting Meeting
  MyCalendar Calendar
}

type Meeting struct {
  MeetingID string // used for proposed
  NumRepliesNeeded *int 
  Attendees []string
  RequestedTimeSlots []int
  TimeSlotsMap map[string][]int
}

type Calendar struct {
  Owner string
  Slots map[int]Booking
}

type Booking struct {
  Status string // A, M, B, R
  MeetingID string
  ProposerID string // node that originated the request
  Attendees []string
}

// From the UI to the proposer
type UserPropose struct {
  MeetingID string
  Attendees []string
  TimeSlots []int
}

func (t* CalendarHandler) UserPropose(req UserPropose, reply *int) error {

  var timeslots []int

  if t.MyMeeting.MeetingID == "InitialID" {
    
    numRepliesNeeded := len(req.Attendees)
    t.MyMeeting = Meeting{req.MeetingID, &numRepliesNeeded, req.Attendees, req.TimeSlots, make(map[string][]int)} 

    for _, time := range req.TimeSlots {
      if t.MyCalendar.Slots[time].Status == "A" {
        fmt.Println("At time " + strconv.Itoa(time) + " try it.")
        t.MyCalendar.Slots[time] = Booking{"R", req.MeetingID, t.SelfID, req.Attendees}
        timeslots = append(timeslots, time)
      }
    }

    for _, attendeeID := range req.Attendees {

      client, err := rpc.DialHTTP("tcp", t.AddressLookup[attendeeID])
      check(err) // TODO handle node failure

      args := Propose{dinvRT.PackM(nil, "Sending PROPOSE to " + attendeeID), t.MyMeeting.MeetingID, t.SelfID, req.Attendees, timeslots}
      reply := 0
      err = client.Call("CalendarHandler.Propose", args, &reply)
      check(err)

      err = client.Close()
      check(err)

    }

  } else {
    fmt.Println("You are already trying a meeting, fool.")
    return errors.New("Already attempting meetingID " + t.MyMeeting.MeetingID)
  }

  return nil

}

// Message Type 1. Proposer sends to acceptors
type Propose struct {
  Buffer []byte
  MeetingID string
  ProposerID string
  Attendees []string
  TimeSlots []int 
}

func (t* CalendarHandler) Propose(req Propose, reply *int) error {

  fmt.Println(req.ProposerID + ", I accept you!")
  dinvRT.UnpackM(req.Buffer, nil, "Got PROPOSE from " + req.ProposerID)

  var timeslots []int

  for _, time := range req.TimeSlots {
    if t.MyCalendar.Slots[time].Status == "A" {
      fmt.Println("At time " + strconv.Itoa(time) + " is okay!")
      booking := Booking{"R", req.MeetingID, req.ProposerID, req.Attendees}
      t.MyCalendar.Slots[time] = booking
      timeslots = append(timeslots, time)
    }
  }

  fmt.Println(t.AddressLookup)
  fmt.Println(req.ProposerID)
  fmt.Println(t.MyCalendar)
  client, err := rpc.DialHTTP("tcp", t.AddressLookup[req.ProposerID])
  check(err) // TODO handle node failure

  args := Reserve{dinvRT.PackM(nil, "Sending RESERVE to " + req.ProposerID), req.MeetingID, t.SelfID, timeslots}
  subreply := 0
  err = client.Call("CalendarHandler.Reserve", args, &subreply)
  check(err)

  err = client.Close()
  check(err)  

  return nil

}

// Message type 2. Acceptors send RESERVE to proposer
type Reserve struct {
  Buffer []byte
  MeetingID string
  AcceptorID string
  TimeSlots []int
}

func (t* CalendarHandler) Reserve(req Reserve, reply *int) error {

  fmt.Println("Got RESERVE from " + req.AcceptorID)
  dinvRT.UnpackM(req.Buffer, nil, "Got RESERVE from " + req.AcceptorID)

  if (t.MyMeeting.MeetingID == req.MeetingID) {
    *t.MyMeeting.NumRepliesNeeded = *t.MyMeeting.NumRepliesNeeded - 1
    t.MyMeeting.TimeSlotsMap[req.AcceptorID] = req.TimeSlots
  }

  bestTime := -1
  if (*t.MyMeeting.NumRepliesNeeded == 0) {   
    
    bestTime = findIntersectingTime(t.MyMeeting.TimeSlotsMap, t.MyMeeting.RequestedTimeSlots, len(t.MyMeeting.Attendees))
    fmt.Println("The best time is at " + strconv.Itoa(bestTime))

    for _, attendeeID := range t.MyMeeting.Attendees {

      client, err := rpc.DialHTTP("tcp", t.AddressLookup[attendeeID])
      check(err) // TODO handle node failure

      args := Select{dinvRT.PackM(nil, "Sending SELECT to " + attendeeID), t.MyMeeting.MeetingID, t.SelfID, bestTime}
      reply := 0
      err = client.Call("CalendarHandler.Select", args, &reply)
      check(err)

      if reply != 1 {
        fmt.Println("ROGER FAILED?")
        os.Exit(1)
      }

      err = client.Close()
      check(err)

      fmt.Println("ROGER DONE")

    }

    for _, time := range t.MyMeeting.RequestedTimeSlots {
      
      booking := t.MyCalendar.Slots[time]

      if booking.MeetingID == t.MyMeeting.MeetingID {

        if (booking.Status == "R") { 
          if time == bestTime {
            t.MyCalendar.Slots[time] = Booking{"M", booking.MeetingID, booking.ProposerID, booking.Attendees}
          } else {
            t.MyCalendar.Slots[time] = Booking{"A", "", "", make([]string, 0)}  
          } 
        }
      }

    }

    t.MyMeeting = initMeeting()
    fmt.Println(t.MyMeeting)
    fmt.Println(t.MyCalendar)

  }

  return nil
}

type Select struct {
  Buffer []byte
  MeetingID string
  ProposerID string
  BestTime int
}

func (t* CalendarHandler) Select(req Select, reply *int) error {

  for time, booking := range t.MyCalendar.Slots {
    
    if booking.MeetingID == req.MeetingID {

      if (booking.Status == "R") { 
        if time == req.BestTime {
          t.MyCalendar.Slots[time] = Booking{"M", booking.MeetingID, booking.ProposerID, booking.Attendees}
        } else {
          t.MyCalendar.Slots[time] = Booking{"A", "", "", make([]string, 0)}  
        } 
      }
    }

  }

  fmt.Println(t.MyCalendar)

  *reply = 1
  return nil
}

/* UTILITY FUNCTIONS */
func findIntersectingTime(timeslotMap map[string][]int, requested []int, requestedNum int) int {

  acceptCountMap := make(map[int]int)
  for _, time := range requested {
    acceptCountMap[time] = 0
  }

  for _, goodTimes := range timeslotMap {
    for _, time := range goodTimes {
      acceptCountMap[time]++       
    }
  }

  for time, count := range acceptCountMap {
    if count == requestedNum {
      return time
    }
  } 

  return -1
}

func initCalendar(owner string) Calendar {
  
  bookings := make(map[int]Booking) 
  for i := 0; i < 24; i++ {
    newBooking := Booking{"A", "", "", make([]string, 0)}
    bookings[i] = newBooking
  }

  aCalendar := Calendar{owner, bookings}
  fmt.Println(aCalendar)
  return aCalendar
}

func initMeeting() Meeting {
  numRepliesNeeded := 0
  initialMeeting := Meeting{"InitialID", &numRepliesNeeded, make([]string, 0), make([]int, 0), make(map[string][]int) }
  return initialMeeting
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

    fmt.Println("Starting.....")

    myName := ""
    address := os.Args[1]
    peersfile := os.Args[2]

    fmt.Println("Address: " + address)
    fmt.Println("Peersfile: " + peersfile)

    Logger := govec.Initialize("Master " + address, "log.txt")
    dinvRT.GetLogger().LogLocalEvent("Starting...")

    var peers []Peer
    lookup := make(map[string]string)

    // Open the peersfile
    file, err := os.Open(peersfile)
    check(err)
    defer file.Close()
    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
      
      peerCombo := scanner.Text()
      peerSplit := strings.Split(peerCombo, ",")
      peerName := peerSplit[0]
      peerAddress := peerSplit[1]

      // don't add yourself
      if peerAddress != address {
        peers = append(peers, Peer{peerName, peerAddress, new(bool)})
        lookup[peerName] = peerAddress
        fmt.Println("Peer found at: " + peerAddress)
      } else {
        myName = peerName
      }
      
    }

    // Set up for BEAT messages
    heartbeatHandler := HeartbeatHandler{Logger, peers}
    rpc.Register(&heartbeatHandler)

    calendarHandler := CalendarHandler{Logger, myName, lookup, initMeeting(), initCalendar(myName)}
    rpc.Register(&calendarHandler)

    // Export the RPC functions
    rpc.HandleHTTP()
    listener,err := net.Listen("tcp", address)
    check(err)
    go http.Serve(listener, nil)

    status := heartbeatPhase(Logger, listener, address, myName, peers)
    if status == 1 {
      fmt.Println("Ready to start Archie!")
      archieMain(Logger)
    }

    fmt.Println("I FAILED")
    os.Exit(1)
}

func archieMain(Logger *govec.GoLog) {
  
  for {

  }

}

func heartbeatPhase(Logger *govec.GoLog, listener net.Listener, address string, name string, peers []Peer) int{

  allPeersFound := false
    
  for {
    for _, peer := range(peers) {
      
      fmt.Println(peer.address)
      fmt.Println(*peer.active)
      if peer.address != address && !*peer.active {
        
        client, err := rpc.DialHTTP("tcp", peer.address)
        if err != nil {
          // Just wait for them to start up
          fmt.Println(err)
          fmt.Println("Couldn't contact " + peer.name)
          time.Sleep(time.Duration(1000) * time.Millisecond)
          continue
        }

        args := Heartbeat{dinvRT.PackM(nil, "Beating to " + peer.address), name, address}
        reply := 0
        err = client.Call("HeartbeatHandler.Beat", args, &reply)
        check(err)

        if reply == 1 {
          fmt.Println("Got reply = 1")
          *peer.active = true
          fmt.Println(peer)
        }

        err = client.Close()
        check(err)

      }
    }

    // Check if everyone is alive
    allPeersFound = true
    for _, peer := range(peers) {
      if !*peer.active {
        allPeersFound = false        
      }
    }

    if allPeersFound {
      return 1
    }
  }

}
