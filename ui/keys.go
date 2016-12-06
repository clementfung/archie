package main

import (
    "fmt"
    "os"
    "os/exec"
)

func main() {
    // disable input buffering
    exec.Command("stty", "-F", "/dev/tty", "cbreak", "min", "1").Run()
    // do not display entered characters on the screen
    exec.Command("stty", "-F", "/dev/tty", "-echo").Run()

    var b []byte = make([]byte, 1)
    for {
        os.Stdin.Read(b)
        fmt.Println("\nI got the byte", b, "("+string(b)+")")
        fmt.Println("also: "+ string([]byte("27")))

        if b[0] == 27 {
            os.Stdin.Read(b)
            if b[0] == 91 {
                os.Stdin.Read(b)
                switch b[0] {
                case 65 :
                    fmt.Println("up")
                    continue
                case 66 :
                    fmt.Println("down")
                    continue
                case 67 :
                    fmt.Println("right")
                    continue
                case 68 :
                    fmt.Println("left")
                    continue
                }
                
            }
        }

        if b[0] >= 65 && b[0] <= 90 {
            key := string(b[0] + 32)
            fmt.Println(key)
            continue
        }

        if b[0] >= 97 && b[0] <= 122 {
            key := string(b[0])
            fmt.Println(key)
            continue
        }

    }

    // 27, 91
    //  U: 65 
    //  D: 66
    //  R: 67
    //  L: 68


}