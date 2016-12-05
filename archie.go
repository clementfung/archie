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
  Name string
  Address string
  IsActive *bool
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
  SelfID int

  AddressLookup map[int]Peer
  MyMeetings []Meeting
  MyCalendar Calendar
  MyCache map[int]Calendar
  NumNodes int
  RepFactor int
}

type Meeting struct {
  MeetingID string // used for proposed
  NumRepliesNeeded *int 
  Attendees []int
  RequestedTimeSlots []int
  TimeSlotsMap map[int][]int
}

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
  Attendees []string
  TimeSlots []int
}

func (t* CalendarHandler) UserPropose(req UserPropose, reply *int) error {

  var timeslots []int

  theMeeting := initMeeting()
  for _, meeting := range t.MyMeetings {
    if meeting.MeetingID == req.MeetingID {
      theMeeting.MeetingID = "Taken"
    }
  }

  if theMeeting.MeetingID != "Taken" {

    attendeeIDs := make([]int, 0)
    attendeeNames := sliceToMap(req.Attendees)

    for id, peer := range t.AddressLookup {
      _, contains := attendeeNames[peer.Name]
      if contains {
        attendeeIDs = append(attendeeIDs, id)
      } 
    }
    
    fmt.Println(req.Attendees)
    fmt.Println(attendeeIDs)
    numRepliesNeeded := len(attendeeIDs)
    theMeeting = Meeting{req.MeetingID, &numRepliesNeeded, attendeeIDs, req.TimeSlots, make(map[int][]int)} 
    t.MyMeetings = append(t.MyMeetings, theMeeting)

    for _, time := range req.TimeSlots {
      if t.MyCalendar.Slots[time].Status == "A" {
        fmt.Println("At time " + strconv.Itoa(time) + " try it.")
        t.MyCalendar.Slots[time] = Booking{"R", req.MeetingID, t.SelfID, attendeeIDs, req.TimeSlots}
        timeslots = append(timeslots, time)
      }
    }

    for _, attendeeID := range attendeeIDs {

      sent := false
      isProxy := false

      for i := 0; i <= t.RepFactor; i++ {
        
        contactID := (attendeeID + i) % t.NumNodes
        
        if (*t.AddressLookup[contactID].IsActive) {        
          
          client, err := rpc.DialHTTP("tcp", t.AddressLookup[contactID].Address)

          if err != nil {
            fmt.Println("Detected a failure on " + t.AddressLookup[contactID].Name + " fallback to next person")
            *t.AddressLookup[contactID].IsActive = false
            continue
          }

          args := Propose{dinvRT.PackM(nil, "Sending PROPOSE to " + t.AddressLookup[contactID].Name), 
              theMeeting.MeetingID, t.SelfID, attendeeIDs, timeslots, attendeeID, isProxy}

          subreply := 0
          err = client.Call("CalendarHandler.Propose", args, &subreply)
          
          if err != nil {
            fmt.Println("Detected a failure on " + t.AddressLookup[contactID].Name + " fallback to next person")
            *t.AddressLookup[contactID].IsActive = false
            continue
          }

          err = client.Close()
          check(err)
          sent = true  
          
          break  

        } else {
          fmt.Println(t.AddressLookup[contactID].Name + " is logged as inactive.")
          isProxy = true
        }
      
      }

      if (!sent) {
        fmt.Println("Not enough replicas alive!")
        os.Exit(1)
      }

    }

  } else {
    fmt.Println("You are already trying a meeting, fool.")
    return errors.New("Already attempting meetingID " + req.MeetingID)
  }

  *reply = 1
  return nil

}

// Message Type 1. Proposer sends to acceptors
type Propose struct {
  Buffer []byte
  MeetingID string
  ProposerID int
  Attendees []int
  TimeSlots []int
  AcceptorID int
  IsProxy bool 
}

func (t* CalendarHandler) Propose(req Propose, reply *int) error {

  fmt.Println(t.AddressLookup[req.ProposerID].Name + ", I accept you!")
  dinvRT.UnpackM(req.Buffer, nil, "Got PROPOSE from " + t.AddressLookup[req.ProposerID].Name)

  var timeslots []int

  if (req.IsProxy) {

    _, contains := t.MyCache[req.AcceptorID]
    
    if contains {

      for _, time := range req.TimeSlots {
        if t.MyCache[req.AcceptorID].Slots[time].Status == "A" {
          fmt.Println("At time " + strconv.Itoa(time) + " is okay!")
          booking := Booking{"R", req.MeetingID, req.ProposerID, req.Attendees, req.TimeSlots}
          t.MyCache[req.AcceptorID].Slots[time] = booking
          timeslots = append(timeslots, time)
        }
      }    
    }

  } else {

    for _, time := range req.TimeSlots {
      if t.MyCalendar.Slots[time].Status == "A" {
        fmt.Println("At time " + strconv.Itoa(time) + " is okay!")
        booking := Booking{"R", req.MeetingID, req.ProposerID, req.Attendees, req.TimeSlots}
        t.MyCalendar.Slots[time] = booking
        timeslots = append(timeslots, time)
      }
    }

  }

  sent := false
  isProxy := false

  for i := 0; i <= t.RepFactor; i++ {

    contactID := (req.ProposerID + i) % t.NumNodes
      
    if (*t.AddressLookup[contactID].IsActive) {        
      
      client, err := rpc.DialHTTP("tcp", t.AddressLookup[contactID].Address)

      if err != nil {
        fmt.Println("Detected a failure on " + t.AddressLookup[contactID].Name + " fallback to next person")
        *t.AddressLookup[contactID].IsActive = false
        continue
      }

      args := Reserve{dinvRT.PackM(nil, "Sending RESERVE to " + t.AddressLookup[contactID].Name), req.MeetingID, t.SelfID, timeslots, isProxy}
      subreply := 0
      err = client.Call("CalendarHandler.Reserve", args, &subreply)
      
      if err != nil {
        fmt.Println("Detected a failure on " + t.AddressLookup[contactID].Name + " fallback to next person")
        *t.AddressLookup[contactID].IsActive = false
        continue
      }

      err = client.Close()
      check(err)  

      sent = true  
      break  

    } else {
      fmt.Println(t.AddressLookup[contactID].Name + " is logged as inactive.")
      isProxy = true
    }
    
  }

  if (!sent) {
    fmt.Println("Not enough replicas alive!")
    os.Exit(1)
  }
  
  *reply = 1
  return nil

}

// Message type 2. Acceptors send RESERVE to proposer
type Reserve struct {
  Buffer []byte
  MeetingID string
  AcceptorID int
  TimeSlots []int
  IsProxy bool
}

func (t* CalendarHandler) Reserve(req Reserve, reply *int) error {

  fmt.Println("Got RESERVE from " + t.AddressLookup[req.AcceptorID].Name)
  dinvRT.UnpackM(req.Buffer, nil, "Got RESERVE from " + t.AddressLookup[req.AcceptorID].Name)

  meetingIdx := -1
  for i, myMeeting := range t.MyMeetings {
    if (myMeeting.MeetingID == req.MeetingID) {
      *t.MyMeetings[i].NumRepliesNeeded = *t.MyMeetings[i].NumRepliesNeeded - 1
      t.MyMeetings[i].TimeSlotsMap[req.AcceptorID] = req.TimeSlots
      meetingIdx = i
      break
    }
  }

  if (meetingIdx == -1) {
    fmt.Println("This isn't my Meeting?" + req.MeetingID)
    fmt.Println(t.MyMeetings)
    os.Exit(1)
  }

  bestTime := -1
  myMeeting := t.MyMeetings[meetingIdx] 

  if (*t.MyMeetings[meetingIdx].NumRepliesNeeded == 0) {   
    
    bestTime = findIntersectingTime(myMeeting.TimeSlotsMap, myMeeting.RequestedTimeSlots, len(myMeeting.Attendees))
    
    if (bestTime == -1) {
      fmt.Println("No times work!")
    } else {
      fmt.Println("The best time is at " + strconv.Itoa(bestTime))
    }

    for _, attendeeID := range t.MyMeetings[meetingIdx].Attendees {

      sent := false
      isProxy := false

      for i := 0; i <= t.RepFactor; i++ {

        contactID := (attendeeID + i) % t.NumNodes
          
        if (*t.AddressLookup[contactID].IsActive) {        

          client, err := rpc.DialHTTP("tcp", t.AddressLookup[contactID].Address)

          if err != nil {
            fmt.Println("Detected a failure on " + t.AddressLookup[contactID].Name + " fallback to next person")
            *t.AddressLookup[contactID].IsActive = false
            continue
          }

          fmt.Println("Sending SELECT to " + t.AddressLookup[contactID].Name)
          args := Select{dinvRT.PackM(nil, "Sending SELECT to " + t.AddressLookup[contactID].Name), myMeeting.MeetingID, t.SelfID, bestTime, isProxy}
          subreply := 0
          err = client.Call("CalendarHandler.Select", args, &subreply)
          
          if err != nil {
            fmt.Println("Detected a failure on " + t.AddressLookup[contactID].Name + " fallback to next person")
            *t.AddressLookup[contactID].IsActive = false
            continue
          }

          err = client.Close()
          check(err)

          fmt.Println("ROGER DONE")          

          sent = true  
          break  

        } else {
          fmt.Println(t.AddressLookup[contactID].Name + " is inactive. Falling back...")
          isProxy = true
        }
        
      }

      if (!sent) {
        fmt.Println("Not enough replicas alive!")
        os.Exit(1)
      }

    }

    fmt.Println(myMeeting)
    for _, time := range myMeeting.RequestedTimeSlots {
      
      booking := t.MyCalendar.Slots[time]

      if booking.MeetingID == myMeeting.MeetingID {

        if (booking.Status == "R") { 
          if time == bestTime {
            t.MyCalendar.Slots[time] = Booking{"M", booking.MeetingID, booking.ProposerID, booking.Attendees, booking.Alternates}
          } else {
            t.MyCalendar.Slots[time] = Booking{"A", "", -1, make([]int, 0), make([]int, 0)}  
          } 
        }
      }

    }

    t.MyMeetings[meetingIdx] = t.MyMeetings[len(t.MyMeetings)-1]
    t.MyMeetings = t.MyMeetings[:len(t.MyMeetings)-1]
    fmt.Println(t.MyCalendar)
    fmt.Println(t.MyMeetings)
  }

  *reply = 1
  return nil

}

type Select struct {
  Buffer []byte
  MeetingID string
  ProposerID int
  BestTime int
  IsProxy bool
}

// Message type 3. Proposer SELECTS to acceptors
func (t* CalendarHandler) Select(req Select, reply *int) error {

  fmt.Println("Got SELECT from " + t.AddressLookup[req.ProposerID].Name)
  dinvRT.UnpackM(req.Buffer, nil, "Got SELECT from " + t.AddressLookup[req.ProposerID].Name)

  for time, booking := range t.MyCalendar.Slots {
    
    if booking.MeetingID == req.MeetingID {

      if (booking.Status == "R") { 
        if time == req.BestTime {
          t.MyCalendar.Slots[time] = Booking{"M", booking.MeetingID, booking.ProposerID, booking.Attendees, booking.Alternates}
        } else {
          t.MyCalendar.Slots[time] = Booking{"A", "", -1, make([]int, 0), make([]int, 0)}  
        } 
      }
    }

  }

  fmt.Println(t.MyCalendar)

  *reply = 1
  return nil
}

type UserBusy struct {
  Time int
}

func (t* CalendarHandler) UserBusy(req UserBusy, reply *int) error {

  theBooking := t.MyCalendar.Slots[req.Time]
  fmt.Println("Oops! I'm busy at " + strconv.Itoa(req.Time))
  t.MyCalendar.Slots[req.Time] = Booking{"B", "", -1, make([]int, 0), make([]int, 0)}  

  if theBooking.Status == "M" {
    
    if (theBooking.ProposerID != t.SelfID) {
    
      fmt.Println("Need to let " + t.AddressLookup[theBooking.ProposerID].Name + " know to reschedule!")

      sent := false
      isProxy := false

      for i := 0; i <= t.RepFactor; i++ {

        contactID := (theBooking.ProposerID + i) % t.NumNodes
          
        if (*t.AddressLookup[contactID].IsActive) {        

          client, err := rpc.DialHTTP("tcp", t.AddressLookup[contactID].Address)

          if err != nil {
            fmt.Println("Detected a failure on " + t.AddressLookup[contactID].Name + " fallback to next person")
            *t.AddressLookup[contactID].IsActive = false
            continue
          }

          fmt.Println("Requesting a RESCHEDULE from " + t.AddressLookup[contactID].Name)    
          args := Reschedule{dinvRT.PackM(nil, "Requesting a RESCHEDULE from " + t.AddressLookup[contactID].Name), 
              t.SelfID, theBooking.ProposerID, theBooking.MeetingID, req.Time, theBooking.Attendees, theBooking.Alternates, isProxy}
          
          subreply := 0
          err = client.Call("CalendarHandler.RequestReschedule", args, &subreply)

          if err != nil {
            fmt.Println("Detected a failure on " + t.AddressLookup[contactID].Name + " fallback to next person")
            *t.AddressLookup[contactID].IsActive = false
            continue
          }

          err = client.Close()
          check(err)

          sent = true  
          break  

        } else {
          fmt.Println(t.AddressLookup[contactID].Name + " is logged as inactive.")
          isProxy = true
        }
        
      }

      if (!sent) {
        fmt.Println("Not enough replicas alive!")
        os.Exit(1)
      }

    } else {

      fmt.Println("Need to let everyone know to cancel it!")

      for _, attendeeID := range theBooking.Attendees {
    
        sent := false
        isProxy := false

        for i := 0; i <= t.RepFactor; i++ {

          contactID := (attendeeID + i) % t.NumNodes
            
          if (*t.AddressLookup[contactID].IsActive) {        

            client, err := rpc.DialHTTP("tcp", t.AddressLookup[contactID].Address)

            if err != nil {
              fmt.Println("Detected a failure on " + t.AddressLookup[contactID].Name + " fallback to next person")
              *t.AddressLookup[contactID].IsActive = false
              continue
            }

            args := Cancel{dinvRT.PackM(nil, "Sending CANCEL to " + t.AddressLookup[contactID].Name), 
                theBooking.MeetingID, t.SelfID, req.Time, isProxy}

            subreply := 0
            err = client.Call("CalendarHandler.Cancel", args, &subreply)

            if err != nil {
              fmt.Println("Detected a failure on " + t.AddressLookup[contactID].Name + " fallback to next person")
              *t.AddressLookup[contactID].IsActive = false
              continue
            }

            err = client.Close()
            check(err)

            fmt.Println("Got CANCEL ROGER")

            sent = true  
            break  

          } else {
            fmt.Println(t.AddressLookup[contactID].Name + " is logged as inactive.")
            isProxy = true
          }
          
        }

        if (!sent) {
          fmt.Println("Not enough replicas alive!")
          os.Exit(1)
        }

      }
    }
  }

  fmt.Println(t.MyCalendar.Slots) 
  *reply = 1
  return nil

} 

type Reschedule struct {
  Buffer []byte
  Rescheduler int
  ProposerID int
  MeetingID string
  Time int
  Attendees []int
  Alternates []int
  IsProxy bool
}

func (t* CalendarHandler) RequestReschedule(req Reschedule, reply *int) error {
  
  fmt.Println("Got a RESCHEDULE from " + t.AddressLookup[req.Rescheduler].Name + " for " + req.MeetingID)
  dinvRT.UnpackM(req.Buffer, nil, "Got RESCHEDULE from " + t.AddressLookup[req.Rescheduler].Name + " for " + req.MeetingID)
  theBooking := t.MyCalendar.Slots[req.Time]
  attendees := theBooking.Attendees

  numRepliesNeeded := len(theBooking.Attendees)

  var timeslots []int
  for _, time := range req.Alternates {
    if t.MyCalendar.Slots[time].Status == "A" {
      timeslots = append(timeslots, time)
      fmt.Println("At time " + strconv.Itoa(time) + " try it.")
    }
  }

  timeslots = append(timeslots, req.Time)
  fmt.Println("At time " + strconv.Itoa(req.Time) + " try it.")

  theMeeting := Meeting{req.MeetingID, &numRepliesNeeded, req.Attendees, timeslots, make(map[int][]int)} 
  t.MyMeetings = append(t.MyMeetings, theMeeting)

  if theBooking.MeetingID == req.MeetingID && theBooking.Status == "M" {
    t.MyCalendar.Slots[req.Time] = Booking{"R", theBooking.MeetingID, theBooking.ProposerID, theBooking.Attendees, timeslots}  
  }

  for _, time := range req.Alternates {
    if (t.MyCalendar.Slots[time].Status == "A") {
      t.MyCalendar.Slots[time] = Booking{"R", theBooking.MeetingID, theBooking.ProposerID, theBooking.Attendees, timeslots}     
    }
  }

  for _, attendeeID := range attendees {

    sent := false
    isProxy := false

    for i := 0; i <= t.RepFactor; i++ {

      contactID := (attendeeID + i) % t.NumNodes
        
      if (*t.AddressLookup[contactID].IsActive) {        

        client, err := rpc.DialHTTP("tcp", t.AddressLookup[contactID].Address)

        if err != nil {
          fmt.Println("Detected a failure on " + t.AddressLookup[contactID].Name + " fallback to next person")
          *t.AddressLookup[contactID].IsActive = false
          continue
        }

        fmt.Println("Sending RESCHEDULE to " + t.AddressLookup[contactID].Name)
        args := Reschedule{dinvRT.PackM(nil, "Sending RESCHEDULE to " + t.AddressLookup[contactID].Name), 
          req.Rescheduler, req.ProposerID, req.MeetingID, req.Time, req.Attendees, timeslots, isProxy}
        subreply := 0
        err = client.Call("CalendarHandler.Reschedule", args, &subreply)

        if err != nil {
          fmt.Println("Detected a failure on " + t.AddressLookup[contactID].Name + " fallback to next person")
          *t.AddressLookup[contactID].IsActive = false
          continue
        }

        err = client.Close()
        check(err)

        sent = true  
        break  

      } else {
        fmt.Println(t.AddressLookup[contactID].Name + " is logged as inactive.")
        isProxy = true
      }
      
    }

    if (!sent) {
      fmt.Println("Not enough replicas alive!")
      os.Exit(1)
    }
  
  }

  *reply = 1
  return nil

}

func (t* CalendarHandler) Reschedule(req Reschedule, reply *int) error {

  fmt.Println("Got a RESCHEDULE from " + t.AddressLookup[req.ProposerID].Name + " for " + req.MeetingID)
  dinvRT.UnpackM(req.Buffer, nil, "Got a RESCHEDULE from " + t.AddressLookup[req.ProposerID].Name + " for " + req.MeetingID)

  theBooking := t.MyCalendar.Slots[req.Time]
  if theBooking.MeetingID == req.MeetingID || req.Rescheduler == t.SelfID {
    if theBooking.Status == "M" {
      fmt.Println("Rescheduling at " + strconv.Itoa(req.Time))
      t.MyCalendar.Slots[req.Time] = Booking{"R", req.MeetingID, req.ProposerID, req.Attendees, req.Alternates}
    }
  
    var timeslots []int
    for _, time := range req.Alternates {
      if t.MyCalendar.Slots[time].Status == "A" {
        fmt.Println("At time " + strconv.Itoa(time) + " is okay!")
        booking := Booking{"R", req.MeetingID, req.ProposerID, req.Attendees, req.Alternates}
        t.MyCalendar.Slots[time] = booking
        timeslots = append(timeslots, time)
      }
    }

    sent := false
    isProxy := false

    for i := 0; i <= t.RepFactor; i++ {

      contactID := (req.ProposerID + i) % t.NumNodes
        
      if (*t.AddressLookup[contactID].IsActive) {        

        client, err := rpc.DialHTTP("tcp", t.AddressLookup[contactID].Address)

        if err != nil {
          fmt.Println("Detected a failure on " + t.AddressLookup[contactID].Name + " fallback to next person")
          *t.AddressLookup[contactID].IsActive = false
          continue
        }

        args := Reserve{dinvRT.PackM(nil, "Sending RESERVE to " + t.AddressLookup[contactID].Name), req.MeetingID, t.SelfID, timeslots, isProxy}
        subreply := 0
        err = client.Call("CalendarHandler.Reserve", args, &subreply)

        if err != nil {
          fmt.Println("Detected a failure on " + t.AddressLookup[contactID].Name + " fallback to next person")
          *t.AddressLookup[contactID].IsActive = false
          continue
        }

        err = client.Close()
        check(err) 

        sent = true
        break

      } else {
        fmt.Println(t.AddressLookup[contactID].Name + " is logged as inactive.")
        isProxy = true
      }
      
    }

    if (!sent) {
      fmt.Println("Not enough replicas alive!")
      os.Exit(1)
    }
  
  }

  *reply = 1
  return nil
}

type UserCancel struct {
  Time int
}

type Cancel struct {
  Buffer []byte
  MeetingID string 
  ProposerID int 
  Time int
  IsProxy bool
}

func (t* CalendarHandler) UserCancel(req Cancel, reply *int) error {

  theBooking := t.MyCalendar.Slots[req.Time]
  if theBooking.ProposerID != t.SelfID {
    fmt.Println("Trying to cancel a meeting that isn't mine!!")
    return nil
  }

  if theBooking.Status == "M" {
    t.MyCalendar.Slots[req.Time] = Booking{"A", "", -1, make([]int, 0), make([]int, 0)}  
  }
  
  for _, attendeeID := range theBooking.Attendees {

    sent := false
    isProxy := false

    for i := 0; i <= t.RepFactor; i++ {

      contactID := (attendeeID + i) % t.NumNodes
        
      if (*t.AddressLookup[contactID].IsActive) {        

        client, err := rpc.DialHTTP("tcp", t.AddressLookup[contactID].Address)

        if err != nil {
          fmt.Println("Detected a failure on " + t.AddressLookup[contactID].Name + " fallback to next person")
          *t.AddressLookup[contactID].IsActive = false
          continue
        }

        fmt.Println("Sending CANCEL to " + t.AddressLookup[contactID].Name)
        args := Cancel{dinvRT.PackM(nil, "Sending CANCEL to " + t.AddressLookup[contactID].Name), theBooking.MeetingID, 
            theBooking.ProposerID, req.Time, isProxy}

        subreply := 0
        err = client.Call("CalendarHandler.Cancel", args, &subreply)
        
        if err != nil {
          fmt.Println("Detected a failure on " + t.AddressLookup[contactID].Name + " fallback to next person")
          *t.AddressLookup[contactID].IsActive = false
          continue
        }

        err = client.Close()
        check(err)

        fmt.Println("Got CANCEL ROGER")

        sent = true  
        break  

      } else {
        fmt.Println(t.AddressLookup[contactID].Name + " is logged as inactive.")
        isProxy = true
      }
      
    }

    if (!sent) {
      fmt.Println("Not enough replicas alive!")
      os.Exit(1)
    }  

  }

  fmt.Println(t.MyCalendar.Slots)
  *reply = 1
  return nil
}

func (t* CalendarHandler) Cancel(req Cancel, reply *int) error {
  
  fmt.Println("Got a CANCEL from " + t.AddressLookup[req.ProposerID].Name + " for " + req.MeetingID)
  dinvRT.UnpackM(req.Buffer, nil, "Got a CANCEL from " + t.AddressLookup[req.ProposerID].Name + " for " + req.MeetingID)

  if t.MyCalendar.Slots[req.Time].MeetingID == req.MeetingID && t.MyCalendar.Slots[req.Time].Status == "M" {
    t.MyCalendar.Slots[req.Time] = Booking{"A", "", -1, make([]int, 0), make([]int, 0)}  
    *reply = 1
  } 
  
  fmt.Println(t.MyCalendar.Slots)
  return nil

}

type CachePush struct {
  Buffer []byte
  CacheOwner int
  Push Calendar 
}

func (t* CalendarHandler) CachePush(req CachePush, reply *int) error {
  
  dinvRT.UnpackM(req.Buffer, nil, "Got CACHE from " + t.AddressLookup[req.CacheOwner].Name)

  _, contains := t.MyCache[req.CacheOwner]

  if contains {
    t.MyCache[req.CacheOwner] = req.Push
    fmt.Println("Updated Cache of " + t.AddressLookup[req.CacheOwner].Name)
  } else {
    fmt.Println("Illegal Cache Update detected...")
    os.Exit(1)
  }

  fmt.Println(t.MyCache)
  *reply = 1
  return nil

}

func (t* CalendarHandler) GetCalendar(args int, reply *Calendar) error {
  *reply = t.MyCalendar
  return nil
}

/* UTILITY FUNCTIONS */
func sliceToMap(theSlice []string) map[string]struct{} {

  returnMap := make(map[string]struct{}, len(theSlice))
  for _, item := range theSlice {
    returnMap[item] = struct{}{}
  }
  return returnMap
}

func findIntersectingTime(timeslotMap map[int][]int, requested []int, requestedNum int) int {

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

func initCalendar(owner int) Calendar {
  
  bookings := make(map[int]Booking) 
  for i := 0; i < 24; i++ {
    newBooking := Booking{"A", "", -1, make([]int, 0), make([]int, 0)}
    bookings[i] = newBooking
  }

  aCalendar := Calendar{owner, bookings}
  fmt.Println(aCalendar)
  return aCalendar
}

func initMeeting() Meeting {
  numRepliesNeeded := 0
  initialMeeting := Meeting{"InitialID", &numRepliesNeeded, make([]int, 0), make([]int, 0), make(map[int][]int)}
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
    myNum := 0

    repFactor := 2
    numNodes := 0
    address := os.Args[1]
    peersfile := os.Args[2]

    fmt.Println("Address: " + address)
    fmt.Println("Peersfile: " + peersfile)

    Logger := govec.Initialize("Master " + address, "log.txt")
    dinvRT.GetLogger().LogLocalEvent("Starting...")

    var peers []Peer
    lookup := make(map[int]Peer)

    // Open the peersfile
    file, err := os.Open(peersfile)
    check(err)
    defer file.Close()
    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
      
      numNodes++
      peerCombo := scanner.Text()
      peerSplit := strings.Split(peerCombo, ",")
      peerNumStr := peerSplit[0]
      peerName := peerSplit[1]
      peerAddress := peerSplit[2]

      peerNum, err := strconv.Atoi(peerNumStr)
      check(err)

      // don't add yourself
      if peerAddress != address {
        peer := Peer{peerName, peerAddress, new(bool)}
        peers = append(peers, peer)
        lookup[peerNum] = peer
        fmt.Println("Peer found at: " + peerAddress)
      } else {
        myName = peerName
        myNum = peerNum
      }
      
    }

    // Set up for BEAT messages
    heartbeatHandler := HeartbeatHandler{Logger, peers}
    rpc.Register(&heartbeatHandler)

    myCache := make(map[int]Calendar)
    for i := 1; i <= repFactor; i++ {
      myCache[(myNum + i) % numNodes] = initCalendar((myNum + i) % numNodes)  
    }

    calendarHandler := CalendarHandler{Logger, myNum, lookup, make([]Meeting, 0), initCalendar(myNum), myCache, numNodes, repFactor}
    rpc.Register(&calendarHandler)

    // Export the RPC functions
    rpc.HandleHTTP()
    listener,err := net.Listen("tcp", address)
    check(err)
    go http.Serve(listener, nil)

    status := heartbeatPhase(Logger, listener, address, myName, peers)
    if status == 1 {
      fmt.Println("Ready to start Archie!")
      archieMain(Logger, lookup, &calendarHandler.MyCalendar, myNum, numNodes, repFactor)
    }

    fmt.Println("I FAILED")
    os.Exit(1)
}

func archieMain(Logger *govec.GoLog, addressLookup map[int]Peer, myCalendar *Calendar, myNodeNum int, numNodes int, repFactor int) {

  for {

    time.Sleep(time.Duration(5000) * time.Millisecond)
    for i := 1; i <= repFactor; i++ {
      
      nodeNum := (myNodeNum + i) % numNodes
      fmt.Println(nodeNum)
      fmt.Println(addressLookup)

      if (*addressLookup[nodeNum].IsActive) {

        client, err := rpc.DialHTTP("tcp", addressLookup[nodeNum].Address)

        if err != nil {
          fmt.Println("Detected a failure on " + addressLookup[nodeNum].Name + " fallback to next person")
          *addressLookup[nodeNum].IsActive = false
          continue
        }

        args := CachePush{dinvRT.PackM(nil, "Sending CACHE to " + addressLookup[nodeNum].Address), myNodeNum, *myCalendar}
        reply := 0
        err = client.Call("CalendarHandler.CachePush", args, &reply)

        if err != nil {
          fmt.Println("Detected a failure on " + addressLookup[nodeNum].Name + " fallback to next person")
          *addressLookup[nodeNum].IsActive = false
          continue
        }

        err = client.Close()
        check(err)

      } else {
        fmt.Println("Inactive")
        os.Exit(1)
      }

    }

  }

}

func heartbeatPhase(Logger *govec.GoLog, listener net.Listener, address string, name string, peers []Peer) int{

  allPeersFound := false
    
  for {
    for _, peer := range(peers) {
      
      fmt.Println(peer.Address)
      fmt.Println(*peer.IsActive)
      if peer.Address != address && !*peer.IsActive {
        
        client, err := rpc.DialHTTP("tcp", peer.Address)
        if err != nil {
          // Just wait for them to start up
          fmt.Println(err)
          fmt.Println("Couldn't contact " + peer.Name)
          time.Sleep(time.Duration(1000) * time.Millisecond)
          continue
        }

        args := Heartbeat{dinvRT.PackM(nil, "Beating to " + peer.Address), name, address}
        reply := 0
        err = client.Call("HeartbeatHandler.Beat", args, &reply)
        check(err)

        if reply == 1 {
          fmt.Println("Got reply = 1")
          *peer.IsActive = true
          fmt.Println(peer)
        }

        err = client.Close()
        check(err)

      }
    }

    // Check if everyone is alive
    allPeersFound = true
    for _, peer := range(peers) {
      if !*peer.IsActive {
        allPeersFound = false        
      }
    }

    if allPeersFound {
      return 1
    }
  }

}
