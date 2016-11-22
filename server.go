package httpsplitter

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"time"
)

// Some inspiration came from
// github.com/jonfk/golang-chat/blob/master/tcp/server-chat/main.go
// github.com/jonfk/golang-chat/blob/master/tcp/common/common.go

var (
	connections []net.Conn
	prefixes    = [][]byte{[]byte("GET"), []byte("get"), []byte("POST"), []byte("post")}
)

func init() {
	log.SetFlags(log.Lshortfile)
}

func main() {

	defer closeConnections()

	ListenOn := fmt.Sprintf("%v:%v", Conf.ListenOn.Host, Conf.ListenOn.Port)
	l, err := net.Listen(Conf.ConnType, ListenOn)
	checkErr(err, "src listener")
	log.Printf("Server\n")
	log.Printf("Listening on %v\n", ListenOn)

	for {
		inc, err := l.Accept() // accept incoming connection on port
		checkErr(err, "inc cn accept")
		connections = append(connections, inc)
		log.Printf("Incoming   conn from %q\n", inc.RemoteAddr())

		// We sniff or inspect the first chunk
		// of data; depending on the result we create
		// a connection to the first configured dest host ("http")
		// or the second ("non-http").
		// The connections then remain coupled till the end.
		// The probed chunk needs to be extra forwarded,
		// since it has already been "consumed".
		probe, err, isHttp := probeForHttp(inc)
		if err != nil && err != io.EOF {
			checkErr(err, "probing incoming data")
		} else if err != nil && err == io.EOF {
			log.Printf("Incoming data was EOF\n")
			continue
		}
		key := "http"
		if !isHttp {
			key = "non-http"
		}
		dest := fmt.Sprintf("%v:%v", Conf.DestHosts[key].Host, Conf.DestHosts[key].Port)

		forwd, err := net.Dial(Conf.ConnType, dest)
		checkErr(err, "dial forward cn")
		log.Printf("Forwarding bytes to  %q\n", forwd.RemoteAddr())

		go Relay(inc, forwd, probe)
	}

}

// Func Relay takes two connections.
// a.) It reads the  first and passes   on to the second.
// b.) It reads the second and passes back to the first.
// The four events are synced by for-select.
// Only a.) or b.) are executed at any given time.
// Errors or EOF messages lead to orderly closing
// of both connections.
func Relay(incom, forwd net.Conn, probe []byte) {

	defer log.Printf("Relay ended - connections closed\n")
	defer incom.Close() // implicitly sending EOF to source
	defer forwd.Close()

	incData := make(chan []byte) // data channel incoming
	rspData := make(chan []byte) // data channel response from forward
	incErro := make(chan error)  // error channel response from forward
	rspErro := make(chan error)  // error channel incoming

	go func() {
		incData <- probe
		// rspData <- []byte("its me - the splitter\n")
	}()
	time.Sleep(5 * time.Millisecond)

	go ChannelizeCnReads("inc from src  ", incom, incData, incErro)
	go ChannelizeCnReads("rsp from forwd", forwd, rspData, rspErro)

	keepAlive := time.After(Conf.KeepAliveSecs * time.Second)

LoopInOut:
	for {
		select {
		case data := <-incData:
			keepAlive = time.After(Conf.KeepAliveSecs * time.Second) // renew
			// log.Printf("incom data\n%s---", data)
			// fmt.Fprint(forwd, data) // seems to be buffered somehow; does not got through immediately
			n1, err := forwd.Write(data)
			if err != nil {
				log.Printf("err writing bytes into forwd: %v\n", err)
				continue LoopInOut
			}
			log.Printf("%3v eq %3v bytes forwarded -%s-", n1, len(data), bytes.TrimSpace(data))

		case data := <-rspData:
			keepAlive = time.After(Conf.KeepAliveSecs * time.Second) // renew
			// excerpt := strings.Replace(string(data), "\n", "", -1)
			// log.Printf("reply data %v bytes", len(data))
			n2, err := incom.Write(data)
			if err != nil {
				log.Printf("err writing back to incom: %v\n", err)
				continue LoopInOut
			}
			log.Printf("%3v eq %3v bytes written back to incom", n2, len(data))

		case err := <-incErro:
			log.Printf("incoming error %v\n", err)
			break LoopInOut
		case err := <-rspErro:
			if err == io.EOF {
				log.Printf("forward response EOF: %v\n", err)
				break LoopInOut
			}
			log.Printf("forward response error %v\n", err)
			break LoopInOut
		case <-keepAlive:
			log.Printf("No data for %v secs.", Conf.KeepAliveSecs)
			break LoopInOut
		}
	}

}

// Function reads from connection conn
// and puts results into data channel and
// error channel.
func ChannelizeCnReads(lbl string, conn net.Conn, ch chan []byte, eCh chan error) {
	for i := 0; ; i++ {
		b := make([]byte, Conf.ChunkSize)
		n, err := conn.Read(b)
		if err != nil {
			if err == io.EOF {
				// log.Printf("%v %v remote address signalled end of session by cloing remote conn; %v", lbl, i, err)
			}
			eCh <- err // EOF is also forwarded via err channel
			return
		}
		b = b[:n] // shorten last chunk to actual data size
		ch <- b   // regular data send

		if i < 10 || i%10 == 0 {
			// log.Printf("   incoming chunk %5v  \n%v---", i, string(b))
		}
		if false && i > 2000 {
			log.Printf("%v %v Iter max reached", lbl, i)
			return
		}
	}
}

func probeForHttp(conn net.Conn) ([]byte, error, bool) {
	b := make([]byte, Conf.ChunkSize)
	n, err := conn.Read(b)
	b = b[:n] // shorten last chunk to actual data size
	for _, pref := range prefixes {
		if bytes.HasPrefix(b, pref) {
			return b, err, true
		}
	}
	return b, err, false
}

func checkErr(err error, desc string) {
	if err != nil {
		log.Fatalf("err %v -  %v", desc, err)
	}
}

func closeConnections() {
	for i := 0; i < len(connections); i++ {
		err := connections[i].Close()
		if err != nil {
			log.Printf("%2v err %v on closing conn %+v ", i, err, connections[i])
		} else {
			log.Printf("%2v conn closed down: %+v ", i, connections[i])
		}
	}
}

// http://stackoverflow.com/questions/12741386
// ??? still does consume one byte??
func isOpen(c net.Conn) bool {

	if c == nil {
		return false
	}

	one := make([]byte, 1)
	c.SetReadDeadline(time.Now())
	if _, err := c.Read(one); err == io.EOF {
		// remotely closed
		c.Close()
		c = nil
		return false
	}

	// c.SetReadDeadline(time.Now().Add(10 * time.Millisecond))
	var zero time.Time
	c.SetReadDeadline(zero)
	return true

}
