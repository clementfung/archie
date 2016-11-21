package main

import (
  "fmt"
  "os"
  "net"
  "net/http"
  "net/rpc"
  "time"
  "bufio"
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

type Heartbeat struct {
  Buffer []byte
  SourceAddress string
}

func (t *HeartbeatHandler) Beat(req Heartbeat, reply *int) error {  
  fmt.Println("Received BEAT from " + req.SourceAddress)
  *reply = 1
  return nil
}

type Peer struct {
  address string
  active *bool
}

/* UTILITY FUNCTIONS */
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

    address := os.Args[1]
    peersfile := os.Args[2]

    fmt.Println("Address: " + address)
    fmt.Println("Peersfile: " + peersfile)

    Logger := govec.Initialize("Master " + address, "log.txt")
    dinvRT.GetLogger().LogLocalEvent("Starting...")

    var peers []Peer

    // Open the peersfile
    file, err := os.Open(peersfile)
    check(err)
    defer file.Close()
    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
      peerAddress := scanner.Text()
      
      // don't add yourself
      if peerAddress != address {
        peers = append(peers, Peer{peerAddress, new(bool)})
        fmt.Println("Peer found at: " + peerAddress)
      }
      
    }

    status := heartbeatPhase(Logger, address, peers)
    
    if status == 1 {
      fmt.Println("I MADE IT")
      for {
        
      }
    }

    fmt.Println("I FAILED")
    os.Exit(1)
}

func heartbeatPhase(Logger *govec.GoLog, address string, peers []Peer) int{

  // Set up for BEAT messages
  heartbeatHandler := HeartbeatHandler{Logger, peers}
  rpc.Register(&heartbeatHandler)

  // Export the RPC functions
  rpc.HandleHTTP()
  listener,err := net.Listen("tcp", address)
  check(err)
  go http.Serve(listener, nil)

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
          fmt.Println("Couldn't contact " + peer.address)
          time.Sleep(time.Duration(3000) * time.Millisecond)
          continue
        }

        args := Heartbeat{dinvRT.PackM(nil, "Beating to " + peer.address), address}
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
