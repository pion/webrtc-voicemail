package main

import (
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media/oggwriter"
)

func createVoicemail(w http.ResponseWriter, r *http.Request) {
	// Read the offer from HTTP Request
	offer, err := io.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}

	// Create a MediaEngine object to configure the supported codec
	m := &webrtc.MediaEngine{}
	if err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus},
		PayloadType:        111,
	}, webrtc.RTPCodecTypeAudio); err != nil {
		panic(err)
	}

	// Create a new PeerConnection
	peerConnection, err := webrtc.NewAPI(webrtc.WithMediaEngine(m)).NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		panic(err)
	}

	// Set a handler for when a new remote track starts, this handler saves buffers to disk as
	peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		if track.Codec().MimeType != webrtc.MimeTypeOpus {
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

		fmt.Printf("Got %s track, saving to disk as %s (48 kHz, 2 channels) \n", track.Codec().MimeType, fileName)

		for {
			rtpPacket, _, readErr := track.ReadRTP()
			if errors.Is(readErr, io.EOF) {
				fmt.Printf("Done saving to disk as %s (48 kHz, 2 channels) \n", fileName)
				return
			}
			if readErr != nil {
				panic(readErr)
			}
			if err := oggFile.WriteRTP(rtpPacket); err != nil {
				panic(err)
			}
		}
	})

	// Allow us to receive 1 audio track.
	if _, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio); err != nil {
		panic(err)
	}

	if err = peerConnection.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  string(offer),
	}); err != nil {
		panic(err)
	}

	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	}

	if err = peerConnection.SetLocalDescription(answer); err != nil {
		panic(err)
	}

	// Block until ICE Gathering is complete, disabling trickle ICE
	// we do this because we only can exchange one signaling message
	// in a production application you should exchange ICE Candidates via OnICECandidate
	<-gatherComplete

	w.Header().Set("Content-Type", "application/sdp")
	if _, err := w.Write([]byte(peerConnection.LocalDescription().SDP)); err != nil {
		panic(err)
	}
}

func main() {
	if _, err := os.Stat("voicemails"); os.IsNotExist(err) {
		if err = os.Mkdir("voicemails", 0755); err != nil {
			panic(err)
		}
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, r.URL.Path[1:]) })
	http.HandleFunc("/create-voicemail", createVoicemail)

	fmt.Println("Server has started on http://localhost:8080")
	panic(http.ListenAndServe(":8080", nil))
}

func generateUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.New(rand.NewSource(time.Now().UnixNano())).Read(b); err != nil {
		return "", err
	}

	return fmt.Sprintf("%X-%X-%X-%X-%X", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}
