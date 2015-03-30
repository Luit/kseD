package kseD

import (
	"log"

	"github.com/pkg/term"
)

const (
	baudRate = 9600
)

type Device struct {
	t     *term.Term
	debug bool

	readErr  error
	card     chan []byte
	readCard chan []byte
}

func New(file string) (*Device, error) {
	t, err := term.Open(file, term.Speed(baudRate), term.RawMode)
	if err != nil {
		return nil, err
	}
	d := &Device{
		t:        t,
		card:     make(chan []byte),
		readCard: make(chan []byte),
	}
	// if err := d.identify(); err != nil {
	// 	t.Restore()
	// 	return nil, err
	// }
	go d.cardLoop()
	go d.readLoop()
	return d, nil
}

func (d *Device) cardLoop() {
	var uid []byte
	var ok = true
	for {
		if uid != nil {
			select {
			case d.card <- uid:
				uid = nil
				if !ok {
					close(d.card)
					return
				}
			case uid, ok = <-d.readCard:
				if !ok {
					d.readCard = nil
				}
			}
		} else {
			uid = <-d.readCard
		}
	}
}

func (d *Device) read() (byte, error) {
	b := [1]byte{0}
	_, err := d.t.Read(b[:])
	return b[0], err
}

func (d *Device) readLoop() {
	for {
		kind, err := d.read()
		if err != nil {
			d.readErr = err
			close(d.readCard)
			return
		}
		switch kind {
		case 'C':
			l, err := d.read()
			if err != nil {
				d.readErr = err
				close(d.readCard)
				return
			}
			uid := make([]byte, l)
			for i := 0; i < int(l); i += 1 {
				b, err := d.read()
				if err != nil {
					d.readErr = err
					close(d.readCard)
					return
				}
				uid[i] = b
			}
			d.readCard <- uid
		case 'P':
			upper, err := d.read()
			if err != nil {
				d.readErr = err
				close(d.readCard)
				return
			}
			lower, err := d.read()
			if err != nil {
				d.readErr = err
				close(d.readCard)
				return
			}
			pos := uint16(upper)<<8 | uint16(lower)
			log.Printf("Position reported: %d (%dmm)",
				pos, ToMilli(pos))
		}
	}
}

// ToMilli calculates the height in millimeters from a position value.
func ToMilli(pos uint16) int {
	f := float64(pos - 4096)
	f *= 661. / 807.
	return int(f + 630)
}

// FromMilli calculates a position value for a given height in millimeters.
func FromMilli(mm int) uint16 {
	f := float64(mm - 630)
	f *= 807. / 661.
	return uint16(f + 4096)
}

// ReadCard blocks until a Card is presented to the device, and returns the UID.
func (d *Device) ReadCard() (uid []byte, err error) {
	var ok bool
	uid, ok = <-d.card
	if !ok {
		return nil, d.readErr
	}
	return uid, nil
}

// Move sends the device a command to move to a new position.
func (d *Device) Move(position uint16) error {
	_, err := d.t.Write([]byte{
		'G',
		byte((position & 0xFF00) >> 8),
		byte(position & 0x00FF),
	})
	// TODO: block until position reached
	return err
}

// Close will close the connection to the device.
func (d *Device) Close() error {
	err := d.t.Restore()
	return err
}
