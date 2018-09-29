package main

import (
	"fmt"
    "os/exec"
	"github.com/googollee/go-socket.io"
	"strconv"
	"log"
	"net/http"
	"regexp"
	"strings"
)

var users = make([]string, 0)

var volumeChange = make(chan int)

func callAmixer (value int, operator string, parser *regexp.Regexp) int {
	var masterLevel = fmt.Sprintf("%d%%%s", value, operator)
	if result, err := exec.Command("amixer", "-D", "pulse", "sset", "Master", masterLevel).Output (); err == nil {
		//var n = len(result)
		var bytestring = string(result[:]);
		var rawvolume = string(parser.FindString(bytestring))
		var l = len(rawvolume)
		if value, err := strconv.Atoi (rawvolume[1:l-2]); err == nil {
			return value
		}
	} else {
		log.Println ("could not run amixer!")		
	}
	return -1
}

func executeVolumeCommand (cmd string, parser *regexp.Regexp) {
	var volume = -1
	if len(cmd) > 1 && (cmd[0] == '+' || cmd[0] == '-') {
		operator := cmd[:1]
		rawvalue := cmd[1:]
		if value, err := strconv.Atoi (rawvalue); err == nil {
			volume = callAmixer (value, operator, parser)
		}
	} else {
		if value, err := strconv.Atoi (cmd); err == nil {
			volume = callAmixer (value, "", parser)
		}
	}

	//log.Println("sending", volume, "to channel")
	volumeChange <- volume;
}

func broadcastVolume (server *socketio.Server, volume int) {
	//log.Println("Sending volume", volume)
	server.BroadcastTo("controller", "volume:set", volume)
}

func listenForVolumeChange (server *socketio.Server) {
	for volume := range volumeChange {
		//log.Println("reading", volume, "off channel")
		broadcastVolume (server, volume)
	}
}

func serve () {
	server, err := socketio.NewServer(nil)
	if err != nil {
		log.Fatal(err)
	}

	volumeparser, _ := regexp.Compile("\\[([0-9]+\\%)\\]")

	var firefoxpid = getWindowPid ("firefox")
	var ids = getWindowIdsByPid (firefoxpid)
	var l = len(ids)
	if l == 0 {
		panic ("oh oh")
	}
	var firefoxId = ids[l-1]

	server.On("connection", func(so socketio.Socket) {
		var newUser = so.Id()
		users = append(users, newUser)
		log.Println("user connected: ", newUser)
		so.Join("controller")
		so.On("volume:change", func(msg string) {
			//log.Println("received message from ", newUser, " ", msg)
			//so.Emit("chat message", msg)
			if (len(msg) > 0) {
				go executeVolumeCommand (msg, volumeparser)
				//broadcastVolume(server, volume)
			}
		})
		so.On("playback:togglepause", func (msg string) {
			go pauseFirefoxVideo (firefoxId)
		})
		so.On("disconnection", func() {
			log.Println("user disconnected: ", newUser)
			user_index := -1
			for i := 0; i < len(users); i++ {
				if users[i] == newUser {
					user_index = i
					break
				}
			}

			if user_index > -1 {
				users = append(users[:user_index], users[user_index:]...)
			}
		})
	})

	server.On("error", func(so socketio.Socket, err error) {
		log.Println("error:", err)
	})

	go listenForVolumeChange (server)

	http.Handle("/socket.io/", server)
	http.Handle("/", http.FileServer(http.Dir("./html")))
	log.Println("Serving at localhost:5555...")
	log.Fatal(http.ListenAndServe("0.0.0.0:5555", nil))
}

func runCommand (cmd *exec.Cmd) (bool, string) {
	if pidSearch, err := cmd.Output (); err == nil {
		return true, string(pidSearch[:])
	}
	return false, ""
}

func getactivewindow () int {
	var pidCmd = exec.Command("xdotool", "getactivewindow")
	if success, result := runCommand (pidCmd); success {
		var l = len(result)
		if id, err := strconv.Atoi (result[:l-1]); err == nil {
			return id
		}
	}
	return -1
}

func sendkeyto (id int, key string) string {
	var idstring = strconv.Itoa (id)
	var pidCmd = exec.Command("xdotool", "windowactivate", "--sync", idstring, "key", key)
	if success, result := runCommand (pidCmd); success {
		return result
	}
	return ""
}

func activatewindow (id int) string {
	var idstring = strconv.Itoa (id)
	var pidCmd = exec.Command("xdotool", "windowactivate", idstring)
	if success, result := runCommand (pidCmd); success {
		return result
	}
	return ""	
}

func trimEnd (s string) string {
	return s[:len(s)-1]
}

func getWindowPid (name string) int {
	var pidCmd = exec.Command("xdotool", "search", "--name", name, "getwindowpid")
	if success, result := runCommand (pidCmd); success {
		if pid, err := strconv.Atoi (trimEnd (result)); err == nil {
			return pid
		}
	}
	return -1
}

func getWindowIdsByPid (pid int) []int {
	var ids = make ([]int, 0)
	var pidstring = strconv.Itoa (pid)
	log.Printf (pidstring)
	var pidCmd = exec.Command("xdotool", "search", "--pid", pidstring)
	if success, result := runCommand (pidCmd); success {
		var idStack = strings.Split(result, "\n")
		idStack = idStack[:len(idStack)-1]
		for _, rawid := range idStack {
			if id, err := strconv.Atoi (rawid); err == nil {
				ids = append(ids, id)
			}
		}
	}
	return ids
}

func pauseFirefoxVideo (firefoxId int) {
	var startid = getactivewindow()
	sendkeyto (firefoxId, "space")
	activatewindow (startid)
}

func main () {
	serve ();
	//CUR_WIN=$(xdotool getactivewindow) && xdotool windowactivate --sync 60817672 key space && xdotool windowactivate $CUR_WIN
}