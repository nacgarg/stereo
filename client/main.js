const socket = io("http://localhost:5000/")

let logError = (err) => {
    console.error(err)
}

let randomRoom = () => {
    var text = "";
    var possible = "ABCDEFGHIJKLMNOPQRSTUVWXYZ";

    for (var i = 0; i < 5; i++)
        text += possible.charAt(Math.floor(Math.random() * possible.length));

    return text
}

socket.on('connect', function () {
    console.log("connected")
    setTimeout(() => socket.emit('room', window.location.hash || randomRoom()), 1000)
});

socket.on("offer", (sdpEnc) => {
    const sdp = atob(sdpEnc)
    window.conn = new RTCPeerConnection(null);
    window.d = conn.createDataChannel(null);

    d.onmessage = (payload) => {
        console.log(payload)
    }

    // conn.onicecandidate = function (event) {
    //     if (event.candidate) {
    //         conn.addIceCandidate(new RTCIceCandidate(event.candidate),
    //             noAction, errorHandler('AddIceCandidate'));
    //     }
    // };
    conn.onsignalingstatechange = e => console.log(conn.signalingState)
    conn.oniceconnectionstatechange = e => console.log(conn.iceConnectionState)
    conn.onicecandidate = event => {
        if (event.candidate === null) {
            socket.emit("answer", btoa(conn.localDescription.sdp))
        }
    }

    conn.ondatachannel = e => {
        let dc = e.channel
        log('New DataChannel ' + dc.label)
        dc.onclose = () => console.log('dc has closed')
        dc.onopen = () => console.log('dc has opened')
        dc.onmessage = e => log(`Message from DataChannel '${dc.label}' payload '${e.data}'`)
        window.sendMessage = () => {
            let message = document.getElementById('message').value
            if (message === '') {
                return alert('Message must not be empty')
            }

            dc.send(message)
        }
    }

    conn.setRemoteDescription(new RTCSessionDescription({ type: 'offer', sdp })).catch(logError)
    conn.createAnswer().then(d => {
        conn.setLocalDescription(d)
    }).catch(logError)
})