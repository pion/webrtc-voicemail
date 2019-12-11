package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/pion/webrtc/v2"
	"github.com/pion/webrtc/v2/pkg/media/oggwriter"
)

func createVoicemail(w http.ResponseWriter, r *http.Request) {
	sdp := webrtc.SessionDescription{}
	if err := json.NewDecoder(r.Body).Decode(&sdp); err != nil {
		panic(err)
	}

	// Create a MediaEngine object to configure the supported codec
	m := webrtc.MediaEngine{}
	m.RegisterCodec(webrtc.NewRTPOpusCodec(webrtc.DefaultPayloadTypeOpus, 48000))

	peerConnection, err := webrtc.NewAPI(webrtc.WithMediaEngine(m)).NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		panic(err)
	}

	peerConnection.OnTrack(func(track *webrtc.Track, receiver *webrtc.RTPReceiver) {
		if track.Codec().Name != webrtc.Opus {
			return
		}

		uuid, err := generateUUID()
		if err != nil {
			panic(err)
		}
		fileName := fmt.Sprintf("voicemails/%s.ogg", uuid)

		oggFile, err := oggwriter.New(fileName, 48000, 2)
		if err != nil {
			panic(err)
		}

		fmt.Printf("Got Opus track, saving to disk as %s (48 kHz, 2 channels) \n", fileName)

		for {
			rtpPacket, err := track.ReadRTP()
			if err != nil {
				panic(err)
			}
			if err := oggFile.WriteRTP(rtpPacket); err != nil {
				panic(err)
			}
		}
	})

	// Allow us to receive 1 audio track, and 1 video track
	if _, err = peerConnection.AddTransceiver(webrtc.RTPCodecTypeAudio); err != nil {
		panic(err)
	}

	if err = peerConnection.SetRemoteDescription(sdp); err != nil {
		panic(err)
	}

	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	}

	if err = peerConnection.SetLocalDescription(answer); err != nil {
		panic(err)
	}

	output, err := json.MarshalIndent(answer, "", "  ")
	if err != nil {
		panic(err)
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(output); err != nil {
		panic(err)
	}
}

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, r.URL.Path[1:]) })
	http.HandleFunc("/create-voicemail", createVoicemail)

	panic(http.ListenAndServe(":8080", nil))
}

func generateUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.New(rand.NewSource(time.Now().UnixNano())).Read(b); err != nil {
		return "", err
	}

	return fmt.Sprintf("%X-%X-%X-%X-%X", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}
