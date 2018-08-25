package main

import (
	"bufio"
	"fmt"
	"os"
	"sync"

	"github.com/googollee/go-socket.io"
	"github.com/pions/webrtc"
	"github.com/pions/webrtc/pkg/datachannel"
	"github.com/pions/webrtc/pkg/ice"
)

// TODO: Eventually replace this with some sort of in-memory database.
// For now let's be lazy and just use a global variable :D

var r RoomMap
var rLock sync.Mutex

func initDB() {
	// in the future this would handle DB connection
	r = make(RoomMap, 0)
}

type RoomID string
type UserID string

type RoomMap map[RoomID]*Room
type UserMap map[UserID]*UserConnection
type SongQueue struct {
	sync.RWMutex
	Chan    chan *Song
	Current *Song
	skip    bool // if true, aborts transmission of current song
}
type Room struct {
	ID        RoomID
	Users     UserMap
	Queue     *SongQueue
	UsersLock sync.Mutex
}

func NewRoom(id RoomID) *Room {
	r := Room{}
	r.ID = id
	r.Users = make(UserMap, 0)
	r.Queue = NewSongQueue()
	go r.Run()
	return &r
}

func (room *Room) Run() {
queue:
	for {
		song := room.Queue.WaitForSong()
		file, err := os.Open(song.FilePath)
		if err != nil {
			// wait for next song
			fmt.Println(err)
			continue
		}
		defer file.Close()
		const chunksize = 1024
		reader := bufio.NewReader(file)
		buffer := make([]byte, chunksize)

		for {
			if room.Queue.skip {
				continue queue
			}

			// read bytes from file into buffer
			if _, err = reader.Read(buffer); err != nil {
				// EOF
				break
			}
			rtcPayload := datachannel.PayloadBinary{buffer}

			for _, user := range room.GetUsers() {
				if user.Ready() {
					go func() {
						user.Lock()

						err := user.DataChannel.Send(rtcPayload)
						if err != nil {
							fmt.Println(err)
						}
						user.Unlock()
					}()
				}
			}
		}
	}
}

func GetRoom(id RoomID) *Room {
	rLock.Lock()
	defer rLock.Unlock()
	return r[id]
}

func GetRooms() []*Room {
	rLock.Lock()
	defer rLock.Unlock()
	rms := make([]*Room, len(r), len(r))
	i := 0
	for _, val := range r {
		rms[i] = val
		i++
	}
	return rms
}

func AddRoom(ro *Room) *Room {
	rLock.Lock()
	defer rLock.Unlock()
	r[ro.ID] = ro
	return ro
}

func DelRoom(ro *Room) {
	rLock.Lock()
	defer rLock.Unlock()
	DelRoomID(ro.ID)
}

func DelRoomID(id RoomID) {
	rLock.Lock()
	defer rLock.Unlock()
	delete(r, id)
}

func (ro *Room) GetUsers() UserMap {
	ro.UsersLock.Lock()
	defer ro.UsersLock.Unlock()
	return ro.Users
}

func (ro *Room) GetUser(id UserID) *UserConnection {
	ro.UsersLock.Lock()
	defer ro.UsersLock.Unlock()
	return ro.Users[id]
}

func (ro *Room) AddUser(u *UserConnection) {
	ro.UsersLock.Lock()
	defer ro.UsersLock.Unlock()
	ro.Users[u.ID] = u
}

func (ro *Room) DelUser(u *UserConnection) {
	ro.UsersLock.Lock()
	defer ro.UsersLock.Unlock()
	ro.DelUserID(u.ID)
}

func (ro *Room) DelUserID(id UserID) {
	ro.UsersLock.Lock()
	defer ro.UsersLock.Unlock()
	delete(ro.Users, id)
	if len(ro.Users) == 0 {
		// if room is empty, delete it
		DelRoom(ro)
	}
}

type UserConnection struct {
	sync.RWMutex

	ID          UserID
	Room        RoomID
	WebSocket   *socketio.Socket
	RTC         *webrtc.RTCPeerConnection
	DataChannel *webrtc.RTCDataChannel
	ReadyChan   chan int
}

func (u *UserConnection) Ready() bool {
	return u.RTC.IceConnectionState == ice.ConnectionStateConnected
}

type Song struct {
	FilePath    string
	RequestedBy *UserConnection
	CurrentTime int
}

func (s *Song) Delete() {
	os.Remove(s.FilePath)
}

func NewSongQueue() *SongQueue {
	q := SongQueue{}
	q.Chan = make(chan *Song, 0)
	return &q
}

func (q *SongQueue) Push(s *Song) {
	q.Chan <- s
}

func (q *SongQueue) WaitForSong() *Song {
	s := <-q.Chan
	if q.Current != nil {
		q.Current.Delete()
	}
	q.Current = s
	return s
}

func (q *SongQueue) Skip() {
	q.skip = true
}
