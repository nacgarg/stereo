import Vue from 'vue'
import App from './App.vue'
import io from 'socket.io-client'
var Decoder = require('libopus.js').Decoder;

var dec = new Decoder({ rate: 48000, channels: 2 });

window.socket = io(window.location.origin)

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

window.conn = new RTCPeerConnection(null);
window.d = conn.createDataChannel(null);

console.log('a')
var audioCtx = new (window.AudioContext || window.webkitAudioContext)();
let sources = []

d.onmessage = (payload) => {
  // setTimeout(a => console.log(payload.data), 1000)
  let data = payload.data
  var result = dec.decodeFloat32(Buffer.from(data));

  var myArrayBuffer = audioCtx.createBuffer(2, result.length/2, 48000);
  let chan = myArrayBuffer.getChannelData(0)
  for (let i = 0; i<result.length/2; i++){
    chan[i] = result[i*2]
  }
  // myArrayBuffer.copyToChannel(result, 0)
  // myArrayBuffer.copyToChannel(result, 1)

  // Get an AudioBufferSourceNode.
  // This is the AudioNode to use when we want to play an AudioBuffer
  var source = audioCtx.createBufferSource();
  // set the buffer in the AudioBufferSourceNode
  source.buffer = myArrayBuffer;
  sources.push(source)
  if (source.length == 1) {
    source.connect(audioCtx.destination);
    // start the source playing
    source.start()
  } else {
    setTimeout(() => {
      source.connect(audioCtx.destination);
      // start the source playing
      source.start();
    }, (myArrayBuffer.duration * (sources.length - 1))/1000)
   }
  // connect the AudioBufferSourceNode to the
  // destination so we can hear the sound
}


conn.ondatachannel = e => {
  console.log("OOOOF")
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

conn.onsignalingstatechange = e => console.log(conn.signalingState)
conn.oniceconnectionstatechange = e => {
  console.log(conn.iceConnectionState)
}
conn.onicecandidate = event => {
  if (event.candidate === null) {
    console.log('sent answer')
    socket.emit("answer", btoa(conn.localDescription.sdp))
  } else {
    conn.addIceCandidate(new RTCIceCandidate(event.candidate));
    console.log("added candidate")
  }
}


socket.on('connect', function () {
  console.log("connected")
  setTimeout(() => socket.emit('room', window.location.hash || randomRoom()), 100)
});

socket.on("offer", (sdpEnc) => {
  const sdp = atob(sdpEnc)

  conn.setRemoteDescription(new RTCSessionDescription({ type: 'offer', sdp })).catch(logError)
    .then(conn.createAnswer().then(d => {
      conn.setLocalDescription(d).catch(logError)
    })).catch(logError)

})

window.play = (request) => {
  socket.emit("request", request)
}

new Vue({
  el: '#app',
  render: h => h(App)
})
