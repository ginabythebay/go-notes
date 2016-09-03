package main

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"sync"
	"time"
)

// Stealing just enough of uuid v1 code from
// https://github.com/satori/go.uuid/blob/master/uuid.go to play with
// some ideas about performance

// UUID v1/v2 storage.
var (
	storageMutex  sync.Mutex
	clockSequence uint16
	lastTime      uint64
	hardwareAddr  [6]byte
)

func init() {
	initStorage(&clockSequence, hardwareAddr)
}

var ch = make(chan UUID, 10)

func init() {
	go produceLockFreeUUIDs()
}

// Difference in 100-nanosecond intervals between
// UUID epoch (October 15, 1582) and Unix epoch (January 1, 1970).
const epochStart = 122192928000000000

// Used in string method conversion
const dash byte = '-'

func initClockSequence() uint16 {
	buf := make([]byte, 2)
	safeRandom(buf)
	return binary.BigEndian.Uint16(buf)
}

func initHardwareAddr(addr [6]byte) {
	interfaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range interfaces {
			if len(iface.HardwareAddr) >= 6 {
				copy(hardwareAddr[:], iface.HardwareAddr)
				return
			}
		}
	}

	// Initialize hardwareAddr randomly in case
	// of real network interfaces absence
	safeRandom(hardwareAddr[:])

	// Set multicast bit as recommended in RFC 4122
	hardwareAddr[0] |= 0x01
}

func initStorage(seq *uint16, addr [6]byte) {
	*seq = initClockSequence()
	initHardwareAddr(addr)
}

func safeRandom(dest []byte) {
	if _, err := rand.Read(dest); err != nil {
		panic(err)
	}
}

// Returns UUID v1/v2 storage state.
// Returns epoch timestamp, clock sequence, and hardware address.
func getStorageLockFree() (uint64, uint16, []byte) {
	timeNow := unixTimeFunc()
	// Clock changed backwards since last UUID generation.
	// Should increase clock sequence.
	if timeNow <= lastTime {
		clockSequence++
	}
	lastTime = timeNow

	return timeNow, clockSequence, hardwareAddr[:]
}

// Returns UUID v1/v2 storage state.
// Returns epoch timestamp, clock sequence, and hardware address.
func getStorage() (uint64, uint16, []byte) {
	storageMutex.Lock()
	defer storageMutex.Unlock()

	timeNow := unixTimeFunc()
	// Clock changed backwards since last UUID generation.
	// Should increase clock sequence.
	if timeNow <= lastTime {
		clockSequence++
	}
	lastTime = timeNow

	return timeNow, clockSequence, hardwareAddr[:]
}

// SatoriGenerator knows how to generate V1 UUIDs in the same way that
// it is done here:
// https://github.com/satori/go.uuid/blob/master/uuid.go
type SatoriGenerator struct {
	storageMutex  sync.Mutex
	clockSequence uint16
	lastTime      uint64
	hardwareAddr  [6]byte
}

func NewSatoriGenerator() *SatoriGenerator {
	gen := SatoriGenerator{}
	initStorage(&gen.clockSequence, gen.hardwareAddr)
	return &gen
}

// Returns UUID v1/v2 storage state.
// Returns epoch timestamp, clock sequence, and hardware address.
func (g *SatoriGenerator) getStorage() (uint64, uint16, []byte) {
	g.storageMutex.Lock()
	defer g.storageMutex.Unlock()

	timeNow := unixTimeFunc()
	// Clock changed backwards since last UUID generation.
	// Should increase clock sequence.
	if timeNow <= lastTime {
		g.clockSequence++
	}
	g.lastTime = timeNow

	return timeNow, clockSequence, hardwareAddr[:]
}

// NewV1 returns UUID based on current timestamp and MAC address.
func (g *SatoriGenerator) NewV1() UUID {
	u := UUID{}

	timeNow, clockSeq, hardwareAddr := g.getStorage()

	binary.BigEndian.PutUint32(u[0:], uint32(timeNow))
	binary.BigEndian.PutUint16(u[4:], uint16(timeNow>>32))
	binary.BigEndian.PutUint16(u[6:], uint16(timeNow>>48))
	binary.BigEndian.PutUint16(u[8:], clockSeq)

	copy(u[10:], hardwareAddr)

	u.SetVersion(1)
	u.SetVariant()

	return u
}

// ChannelGenerator follows the same general outline as
// Satorigenerator, but instead of locking, it uses a goroutine which
// communicates over a channel
type ChanneledGenerator struct {
	ch            chan UUID
	clockSequence uint16
	lastTime      uint64
	hardwareAddr  [6]byte
}

func NewChanneledGenerator(chanSize int) *ChanneledGenerator {
	gen := ChanneledGenerator{}
	gen.ch = make(chan UUID, chanSize)
	initStorage(&gen.clockSequence, gen.hardwareAddr)
	go gen.produceUUIDs()
	return &gen
}

// Returns UUID v1/v2 storage state.
// Returns epoch timestamp, clock sequence, and hardware address.
func (g *ChanneledGenerator) getStorage() (uint64, uint16, []byte) {
	timeNow := unixTimeFunc()
	// Clock changed backwards since last UUID generation.
	// Should increase clock sequence.
	if timeNow <= lastTime {
		g.clockSequence++
	}
	g.lastTime = timeNow

	return timeNow, clockSequence, hardwareAddr[:]
}

// NewV1 returns UUID based on current timestamp and MAC address.
func (g *ChanneledGenerator) produceUUIDs() {
	for {
		u := UUID{}

		timeNow, clockSeq, hardwareAddr := g.getStorage()

		binary.BigEndian.PutUint32(u[0:], uint32(timeNow))
		binary.BigEndian.PutUint16(u[4:], uint16(timeNow>>32))
		binary.BigEndian.PutUint16(u[6:], uint16(timeNow>>48))
		binary.BigEndian.PutUint16(u[8:], clockSeq)

		copy(u[10:], hardwareAddr)

		u.SetVersion(1)
		u.SetVariant()

		ch <- u
	}
}

func (g *ChanneledGenerator) NewV1() UUID {
	return <-ch
}

// UUID representation compliant with specification
// described in RFC 4122.
type UUID [16]byte

// NewV1 returns UUID based on current timestamp and MAC address.
func NewV1() UUID {
	u := UUID{}

	timeNow, clockSeq, hardwareAddr := getStorage()

	binary.BigEndian.PutUint32(u[0:], uint32(timeNow))
	binary.BigEndian.PutUint16(u[4:], uint16(timeNow>>32))
	binary.BigEndian.PutUint16(u[6:], uint16(timeNow>>48))
	binary.BigEndian.PutUint16(u[8:], clockSeq)

	copy(u[10:], hardwareAddr)

	u.SetVersion(1)
	u.SetVariant()

	return u
}

// NewV1LockFree returns UUID based on current timestamp and MAC
// address, without taking any locks.
func NewV1LockFree() UUID {
	return <-ch
}

func produceLockFreeUUIDs() {
	for {
		u := UUID{}

		timeNow, clockSeq, hardwareAddr := getStorageLockFree()

		binary.BigEndian.PutUint32(u[0:], uint32(timeNow))
		binary.BigEndian.PutUint16(u[4:], uint16(timeNow>>32))
		binary.BigEndian.PutUint16(u[6:], uint16(timeNow>>48))
		binary.BigEndian.PutUint16(u[8:], clockSeq)

		copy(u[10:], hardwareAddr)

		u.SetVersion(1)
		u.SetVariant()

		ch <- u
	}
}

// SetVersion sets version bits.
func (u *UUID) SetVersion(v byte) {
	u[6] = (u[6] & 0x0f) | (v << 4)
}

// SetVariant sets variant bits as described in RFC 4122.
func (u *UUID) SetVariant() {
	u[8] = (u[8] & 0xbf) | 0x80
}

// Returns canonical string representation of UUID:
// xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx.
func (u UUID) String() string {
	buf := make([]byte, 36)

	hex.Encode(buf[0:8], u[0:4])
	buf[8] = dash
	hex.Encode(buf[9:13], u[4:6])
	buf[13] = dash
	hex.Encode(buf[14:18], u[6:8])
	buf[18] = dash
	hex.Encode(buf[19:23], u[8:10])
	buf[23] = dash
	hex.Encode(buf[24:], u[10:])

	return string(buf)
}

// Returns difference in 100-nanosecond intervals between
// UUID epoch (October 15, 1582) and current time.
// This is default epoch calculation function.
func unixTimeFunc() uint64 {
	return epochStart + uint64(time.Now().UnixNano()/100)
}

func main() {
	fmt.Printf("V1: %s\n", NewV1())
	fmt.Printf("V1: %s\n", NewV1())
	fmt.Println()
	fmt.Printf("V1 lock free: %s\n", NewV1LockFree())
	fmt.Printf("V1 lock free: %s\n", NewV1LockFree())
}
