package main

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"

	"strings"

	"errors"

	"github.com/BrianAllred/goydl"
	"github.com/dchest/uniuri"
	"github.com/googollee/go-socket.io"
	"github.com/pions/webrtc"
	"github.com/pions/webrtc/pkg/datachannel"
	"github.com/pions/webrtc/pkg/ice"
	"mvdan.cc/xurls"
)

func HandleSocketConnection(so socketio.Socket) {

	// Client will want to join a room. We maintain a map of rooms to users in them for WebRTC
	so.On("room", func(room string) {
		fmt.Println(room)
		if isRoomValid(room) {
			so.Join(room)
			readyChan := make(chan int)
			conn, sdp, dataChannel := NewOffer(readyChan)
			so.Emit("offer", sdp)

			user := UserConnection{}
			user.ID = UserID(so.Id())
			user.WebSocket = &so
			user.ReadyChan = readyChan
			user.Room = RoomID(room)
			user.RTC = conn
			user.DataChannel = dataChannel
			roomStruct, ok := r[RoomID(room)]
			if ok {
				roomStruct.Users[UserID(so.Id())] = &user
			} else {
				ro := NewRoom(RoomID(room))
				AddRoom(ro)
				ro.AddUser(&user)
			}

		} else {
			so.Disconnect()
		}

	})
	so.On("answer", func(answerEncoded string) {
		log.Println("answer:", answerEncoded)
		if (len(so.Rooms())) == 0 {
			fmt.Println("no room")
			return
		}
		user := GetRoom(RoomID(so.Rooms()[0])).GetUser(UserID(so.Id()))
		sdp, err := base64.StdEncoding.DecodeString(answerEncoded)
		if err != nil {
			fmt.Println("error decoding sdp", err)
			so.Disconnect()
		}
		err = user.RTC.SetRemoteDescription(webrtc.RTCSessionDescription{
			Type: webrtc.RTCSdpTypeAnswer,
			Sdp:  string(sdp),
		})
		if err != nil {
			fmt.Println("error setting sdp", err)
		}
		<-user.ReadyChan
	})

	so.On("disconnection", func() {
		if (len(so.Rooms())) > 0 {
			GetRoom(RoomID(so.Rooms()[0])).DelUserID(UserID(so.Id()))
		}
	})

	so.On("request", func(requestStr string) {
		if (len(so.Rooms())) == 0 {
			fmt.Println("no room")
			return
		}
		room := GetRoom(RoomID(so.Rooms()[0]))
		user := room.GetUser(UserID(so.Id()))
		filePath, err := downloadSong(requestStr)
		if err != nil {
			log.Println(err)
			return
		}
		song := &Song{filePath, user, 0}
		room.Queue.Push(song)
	})

}

func downloadSong(requestStr string) (filePath string, err error) {
	// if request isn't link, search
	var url string
	if xurls.Strict().MatchString(requestStr) {
		url = xurls.Strict().FindString(requestStr)
	} else {
		url = "ytsearch1:" + requestStr
	}

	ytdl := goydl.NewYoutubeDl()
	// goydl.FileSizeRateOption
	ytdl.Options.ExtractAudio.Value = true
	ytdl.Options.AudioFormat.Value = "wav"
	ytdl.Options.MaxFilesize.Value = goydl.FileSizeRateFromString("200M")

	ytdlFilepath := "downloads/" + uniuri.New() + ".wav" // it's not always a wav but who cares
	ytdl.Options.Output.Value = ytdlFilepath

	cmd, err := ytdl.Download(url)
	if err != nil {
		return "", err
	}

	cmd.Wait()

	// make sure file exists. if not there probably was an error
	if _, err := os.Stat(ytdlFilepath); os.IsNotExist(err) {
		return "", errors.New("File doesn't exist after downloading")
	}
	// encode file into opus
	opusFilepath := ytdlFilepath + ".opus"
	bash := "ffmpeg -i " + ytdlFilepath + " -f wav -acodec pcm_s16le -ac 2 - | opusenc --hard-cbr --bitrate 128 --comp 5 - " + opusFilepath + "; rm " + ytdlFilepath
	_, err = exec.Command("bash", "-c", bash).Output()
	if err != nil {
		return "", fmt.Errorf("Failed to execute command: %s", bash)
	}
	// delete ytdl wav
	os.Remove(ytdlFilepath)
	return opusFilepath, nil
}

func HandleSocketError(so socketio.Socket, err error) {
	log.Println("error:", err)
}

func isRoomValid(room string) bool {
	// Rooms are the format 'abcde'
	roomRe := regexp.MustCompile(`^[a-z]{5}$`)
	return roomRe.MatchString(strings.ToLower(room))
}

func NewOffer(ready chan int) (*webrtc.RTCPeerConnection, string, *webrtc.RTCDataChannel) {
	peerConnection, err := webrtc.New(webrtc.RTCConfiguration{
		ICEServers: []webrtc.RTCICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	})
	if err != nil {
		panic(err)
	}

	peerConnection.Ondatachannel = func(d *webrtc.RTCDataChannel) {
		fmt.Printf("New DataChannel %s %d\n", d.Label, d.ID)

		d.Lock()
		defer d.Unlock()
		d.Onmessage = func(payload datachannel.Payload) {
			switch p := payload.(type) {
			case *datachannel.PayloadString:
				fmt.Printf("Message '%s' from DataChannel '%s' payload '%s'\n", p.PayloadType().String(), d.Label, string(p.Data))
			case *datachannel.PayloadBinary:
				fmt.Printf("Message '%s' from DataChannel '%s' payload '% 02x'\n", p.PayloadType().String(), d.Label, p.Data)
			default:
				fmt.Printf("Message '%s' from DataChannel '%s' no payload \n", p.PayloadType().String(), d.Label)
			}
		}
	}

	dataChannel, err := peerConnection.CreateDataChannel("data", nil)
	if err != nil {
		fmt.Println("error creating datachannel", err)
	}
	peerConnection.OnICEConnectionStateChange = func(connectionState ice.ConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
		if connectionState == ice.ConnectionStateConnected {
			fmt.Println("sending openchannel")
			err := dataChannel.SendOpenChannelMessage()
			if err != nil {
				fmt.Println("faild to send openchannel", err)
			}
			ready <- 1
		}
	}
	dataChannel.Lock()
	dataChannel.Onmessage = func(payload datachannel.Payload) {
		switch p := payload.(type) {
		case *datachannel.PayloadString:
			fmt.Printf("Message '%s' from DataChannel '%s' payload '%s'\n", p.PayloadType().String(), dataChannel.Label, string(p.Data))
		case *datachannel.PayloadBinary:
			fmt.Printf("Message '%s' from DataChannel '%s' payload '% 02x'\n", p.PayloadType().String(), dataChannel.Label, p.Data)
		default:
			fmt.Printf("Message '%s' from DataChannel '%s' no payload \n", p.PayloadType().String(), dataChannel.Label)
		}
	}
	dataChannel.Unlock()

	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		panic(err)
	}

	return peerConnection, base64.StdEncoding.EncodeToString([]byte(offer.Sdp)), dataChannel

}
