package main

import (
	"encoding/base64"
	"fmt"
	"log"

	"regexp"
	"strings"

	"github.com/googollee/go-socket.io"
	"github.com/pions/webrtc"
	"github.com/pions/webrtc/pkg/datachannel"
	"github.com/pions/webrtc/pkg/ice"
)

type RoomID string
type UserID string

type RoomMap map[RoomID]*Room

type Room struct {
	Users       map[UserID]*UserConnection
	CurrentSong string // eventualy
}

func NewRoom() *Room {
	r := Room{}
	r.Users = make(map[UserID]*UserConnection, 0)
	return &r
}

type UserConnection struct {
	Room        RoomID
	WebSocket   *socketio.Socket
	RTC         *webrtc.RTCPeerConnection
	DataChannel *webrtc.RTCDataChannel
	ReadyChan   chan int
}

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
			user.WebSocket = &so
			user.ReadyChan = readyChan
			user.Room = RoomID(room)
			user.RTC = conn
			user.DataChannel = dataChannel
			roomStruct, ok := r[RoomID(room)]
			if ok {
				roomStruct.Users[UserID(so.Id())] = &user
			} else {
				r[RoomID(room)] = NewRoom()
				r[RoomID(room)].Users[UserID(so.Id())] = &user
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
		user := r[RoomID(so.Rooms()[0])].Users[UserID(so.Id())]
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
		user.DataChannel.Send(datachannel.PayloadString{Data: []byte("hello")})
	})

	so.On("disconnection", func() {
		if (len(so.Rooms())) > 0 {
			delete(r[RoomID(so.Rooms()[0])].Users, UserID(so.Id()))
		}
	})

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
