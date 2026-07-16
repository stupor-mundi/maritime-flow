package main

import (
	"fmt"

	"github.com/BertoldVdb/go-ais"
	"github.com/BertoldVdb/go-ais/aisnmea"
)

var nmeaCodec = aisnmea.NMEACodecNew(ais.CodecNew(false, false))

func ExtractMmsi(sentence string) (mmsi string) {
	defer func() {
		if recover() != nil {
			mmsi = ""
		}
	}()

	decoded, err := nmeaCodec.ParseSentence(sentence)
	if err != nil || decoded.Packet == nil {
		return ""
	}

	header := decoded.Packet.GetHeader()
	if header == nil {
		return ""
	}

	return fmt.Sprintf("%09d", header.UserID)
}
