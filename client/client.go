package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/zew/util"
)

type TRes struct {
	Method   string
	Resource string
	Host     string
	Port     string
}

func init() {
	log.SetFlags(log.Lshortfile)
}

func (t *TRes) Req() string {
	if t.Method == "" {
		t.Method = "GET"
	}
	ret := fmt.Sprintf("%v %v HTTP/1.1\n", t.Method, t.Resource)
	ret += fmt.Sprintf("Host: %v\n", t.Host)
	ret += "Connection: close\n"
	ret += "\n"
	return ret
}
func (t *TRes) HostPort() string {
	if t.Port == "" {
		t.Port = ":80"
	}
	return fmt.Sprintf("%v%v", t.Host, t.Port)
}

var reqs = []TRes{
	TRes{Host: "Zero-dummy"},
	TRes{
		Host:     "127.0.0.1",
		Resource: "/dashboard/",
	},
	TRes{
		Host:     "127.0.0.1",
		Resource: "/dashboard/?direct=access",
		Port:     ":81",
	},
	TRes{
		Host:     "exceldb.zew.de",
		Resource: "/",
	},
	TRes{
		Host:     "exceldb.zew.de",
		Resource: "/exceldb",
	},
	TRes{
		Host:     "google.de",
		Resource: "/",
	},
	// Notice of Disconnection: 1.3.6.1.4.1.1466.20036
	TRes{
		Host:     "node01.zew-private.de",
		Port:     ":389",
		Method:   "ldapSearch",
		Resource: "/",
	},
}

func main() {

	for {

		time.Sleep(100 * time.Millisecond) // wait for previous response being dumped
		log.Printf(" ")
		log.Printf("Press")
		for k, v := range reqs {
			if k == 0 {
				continue
			}
			log.Printf("    %v + ENTER to request %q\n", k, v.HostPort())
		}
		log.Printf("Your choice: ...")

		reader := bufio.NewReader(os.Stdin) // read in input from stdin
		strInp, _ := reader.ReadString('\n')
		strInp = strings.TrimSpace(strInp)
		ipt, _ := strconv.Atoi(strInp)

		// connect to this socket
		conn, err := net.Dial("tcp", reqs[ipt].HostPort())
		if err != nil {
			log.Printf("connection failed to %v; %v\n", reqs[ipt].HostPort(), err)
			continue
		}
		log.Printf("connected to %q\n", reqs[ipt].HostPort())

		log.Printf("sending text: \n%v\n", reqs[ipt].Req())
		// fmt.Fprintf(conn, "%v", reqs[ipt].Req()) // send to socket
		// fmt.Fprintf(conn, io.EOF)                // impossible

		n2, err := conn.Write([]byte(reqs[ipt].Req()))
		if err != nil {
			log.Printf("err writing into conn: %v", err)
		}
		log.Printf("%2v bytes written into conn", n2)

		go func() {

			defer conn.Close()

			rdr := bufio.NewReader(conn)
			lines := []string{}

			// collecting lines in buffer
			for i := 0; ; i++ {
				message, err := rdr.ReadString('\n') //listen for reply
				lines = append(lines, message)
				if err != nil {
					lines = append(lines, fmt.Sprintf("error %v", err))
					break
				}
			}

			log.Printf("Message from server...\n")
			for i := 0; i < len(lines); i++ {
				if i < 15 || i > len(lines)-10 {
					if strings.TrimSpace(lines[i]) != "" {
						lines[i] = util.Ellipsoider(lines[i], 90)
						log.Printf("%4v:%v", i, lines[i])
					}
				}
			}
		}()

	}
}
