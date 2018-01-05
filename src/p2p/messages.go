package p2p

import (
	"io"
	"encoding/binary"
	"github.com/sirupsen/logrus"
	"consensus"
	"bytes"
	"net"
	"errors"
)

// Header is header of any protocol message, used to identify incoming messages
type Header struct {
	// magic number
	magic [2]byte
	// Type typo of the message.
	Type uint8
	// Len length of the message in bytes.
	Len uint64
}

func (h *Header) Write(wr io.Writer) error {
	if _, err := wr.Write(h.magic[:]); err != nil {
		return err
	}
	if err := binary.Write(wr, binary.BigEndian, h.Type); err != nil {
		return err
	}

	return binary.Write(wr, binary.BigEndian, h.Len)
}

func (h *Header) Read(r io.Reader) error {
	if _, err := io.ReadFull(r, h.magic[:]); err != nil {
		return err
	}

	logrus.Debug("readed magic: ", h.magic[:])

	if !h.ValidateMagic() {
		return errors.New("invalid magic code")
	}

	if err := binary.Read(r, binary.BigEndian, &h.Type); err != nil {
		return err
	}

	return binary.Read(r, binary.BigEndian, &h.Len)
}

func (h Header) ValidateMagic() bool {
	return h.magic[0] == 0x1e && h.magic[1] == 0xc5
}

// Ping request
type Ping struct {
	// total difficulty accumulated by the sender, used to check whether sync
	// may be needed
	TotalDifficulty consensus.Difficulty
	// total height
	Height uint64
}

func (p Ping) Bytes() []byte {
	logrus.Info("Ping/Pong struct to bytes")
	buff := new(bytes.Buffer)

	if err := binary.Write(buff, binary.BigEndian, uint64(p.TotalDifficulty)); err != nil {
		logrus.Fatal(err)
	}

	if err := binary.Write(buff, binary.BigEndian, uint64(p.Height)); err != nil {
		logrus.Fatal(err)
	}

	return buff.Bytes()
}

func (p Ping) Type() uint8 {
	return msgTypePing
}

func (p *Ping) Read(r io.Reader) error {

	if err := binary.Read(r, binary.BigEndian, (*uint64)(&p.TotalDifficulty)); err != nil {
		return err
	}

	if err := binary.Read(r, binary.BigEndian, (*uint64)(&p.Height)); err != nil {
		return err
	}

	return nil
}

// Pong response same as Ping
type Pong struct {
	Ping
}

func (p Pong) Type() uint8 {
	return msgTypePong
}

// Ask for other peers addresses, required for network discovery.
type GetPeerAddrs struct {
	// filters on the capabilities we'd like the peers to have
	Capabilities capabilities
}

func (p GetPeerAddrs) Bytes() []byte {
	logrus.Info("GetPeerAddrs struct to bytes")
	buff := new(bytes.Buffer)

	if err := binary.Write(buff, binary.BigEndian, uint32(p.Capabilities)); err != nil {
		logrus.Fatal(err)
	}

	return buff.Bytes()
}

func (p GetPeerAddrs) Type() uint8 {
	return msgTypeGetPeerAddrs
}

func (p *GetPeerAddrs) Read(r io.Reader) error {

	return binary.Read(r, binary.BigEndian, (*uint32)(&p.Capabilities))
}

// Sending an error back (usually followed  by closing conn)
type PeerError struct {
	// error code
	Code uint32
	// slightly more user friendly message
	Message string
}

func (p PeerError) Bytes() []byte {
	logrus.Info("GetPeerAddrs struct to bytes")
	buff := new(bytes.Buffer)

	if err := binary.Write(buff, binary.BigEndian, uint32(p.Code)); err != nil {
		logrus.Fatal(err)
	}

	// Write user agent [len][string]
	if err := binary.Write(buff, binary.BigEndian, uint64(len(p.Message))); err != nil {
		logrus.Fatal(err)
	}
	buff.WriteString(p.Message)
	return buff.Bytes()
}

func (p PeerError) Type() uint8 {
	return msgTypeError
}

func (p *PeerError) Read(r io.Reader) error {

	if err := binary.Read(r, binary.BigEndian, (*uint32)(&p.Code)); err != nil {
		return err
	}

	var messageLen uint64
	if err := binary.Read(r, binary.BigEndian, &messageLen); err != nil {
		return err
	}

	logrus.Debug("messageLen: ", messageLen)
	if messageLen > maxStringLength {
		logrus.Warn("too big messageLen len value")
		return errors.New("invalid messageLen len value")
	}

	buff := make([]byte, messageLen)
	if _, err := io.ReadFull(r, buff); err != nil {
		return err
	}

	p.Message = string(buff)
	return nil
}

// PeerAddrs we know of that are fresh enough, in response to GetPeerAddrs
type PeerAddrs struct {
	peers []*net.TCPAddr
}

func (p PeerAddrs) Bytes() []byte {
	logrus.Info("GetPeerAddrs struct to bytes")
	buff := new(bytes.Buffer)

	if err := binary.Write(buff, binary.BigEndian, uint32(len(p.peers))); err != nil {
		logrus.Fatal(err)
	}

	for _, peerAddr := range p.peers {
		// Write Sender addr
		switch len(peerAddr.IP) {
		case net.IPv4len:
			{
				if _, err := buff.Write([]byte{0}); err != nil {
					logrus.Fatal(err)
				}
			}
		case net.IPv6len:
			{
				if _, err := buff.Write([]byte{1}); err != nil {
					logrus.Fatal(err)
				}
			}
		default:
			logrus.Fatal("invalid netaddr")
		}

		if _, err := buff.Write(peerAddr.IP); err != nil {
			logrus.Fatal(err)
		}

		binary.Write(buff, binary.BigEndian, uint16(peerAddr.Port))
	}

	return buff.Bytes()
}

func (p PeerAddrs) Type() uint8 {
	return msgTypePeerAddrs
}

func (p *PeerAddrs) Read(r io.Reader) error {

	var peersCount uint32
	var ipFlag int8

	if err := binary.Read(r, binary.BigEndian, &peersCount); err != nil {
		return err
	}

	if peersCount > maxPeerAddresses {
		logrus.Warn("too big peersCount value")
		return errors.New("invalid peersCount value")
	}

	for i := uint32(0); i < peersCount; i++ {
		if err := binary.Read(r, binary.BigEndian, &ipFlag); err != nil {
			return err
		}

		var ipAddr []byte
		var ipPort uint16

		if ipFlag == 0 {
			// for ipv4 addr
			ipAddr = make([]byte, net.IPv4len)
		} else {
			// for ipv6 addr
			ipAddr = make([]byte, net.IPv6len)
		}

		if _, err := io.ReadFull(r, ipAddr); err != nil {
			return err
		}

		if err := binary.Read(r, binary.BigEndian, &ipPort); err != nil {
			return err
		}

		addr := &net.TCPAddr{
			IP: ipAddr,
			Port: int(ipPort),
		}

		p.peers = append(p.peers, addr)
	}

	return nil
}