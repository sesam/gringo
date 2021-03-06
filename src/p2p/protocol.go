package p2p

import (
	"io"
	"consensus"
	"bufio"
	"github.com/sirupsen/logrus"
	"errors"
)

const (
	// userAgent is name of version of the software
	userAgent       = "gringo v0.0.1"
)

// Message defines methods for WriteMessage/ReadMessage functions
type Message interface {
	// Read reads from reader and fit self struct
	Read(r io.Reader) error

	// Bytes returns binary data of body message
	Bytes() []byte

	// Type says whats the message type should use in header
	Type() uint8
}

// WriteMessage writes to wr (net.conn) protocol message
func WriteMessage(w io.Writer, msg Message) (uint64, error) {
	data := msg.Bytes()

	header := Header{
		magic: consensus.MagicCode,
		Type:  msg.Type(),
		Len:   uint64(len(data)),
	}

	// use the buffered writer
	wr := bufio.NewWriter(w)
	if err := header.Write(wr); err != nil {
		return 0, err
	}

	if n, err := wr.Write(data); err != nil {
		return uint64(n) + consensus.HeaderLen, err
	} else {
		return uint64(n) + consensus.HeaderLen, wr.Flush()
	}
}

// ReadMessage reads from r (net.conn) protocol message
func ReadMessage(r io.Reader, msg Message) (uint64, error) {
	var header Header

	// get the msg header
	rh := io.LimitReader(r, int64(consensus.HeaderLen))
	if err := header.Read(rh); err != nil {
		return 0, err
	}
	logrus.Debug("got header: ", header)

	if header.Type != msg.Type() {
		return uint64(consensus.HeaderLen), errors.New("receive unexpected message type")
	}

	if header.Len > consensus.MaxMsgLen {
		return uint64(consensus.HeaderLen), errors.New("too big message size")
	}

	rb := io.LimitReader(r, int64(header.Len))
	return uint64(consensus.HeaderLen) + uint64(header.Len), msg.Read(rb)
}

// Protocol defines grin-node network communicates
type Protocol interface {
	// TransmittedBytes bytes sent and received
	// TransmittedBytes() uint64

	// SendPing sends a Ping message to the remote peer. Will panic if handle has never
	// been called on this protocol.
	SendPing()

	// SendBlock sends a block to our remote peer
	SendBlock()
	SendTransaction()
	SendHeaderRequest()
	SendBlockRequest()
	SendPeerRequest()

	// Close the connection to the remote peer
	Close()
}
