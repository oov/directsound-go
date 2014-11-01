package main

import (
	"encoding/binary"
	"github.com/oov/directsound-go/dsound"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"unsafe"
)

var (
	kernel32               = syscall.MustLoadDLL("kernel32")
	CreateEvent            = kernel32.MustFindProc("CreateEventW")
	WaitForMultipleObjects = kernel32.MustFindProc("WaitForMultipleObjects")

	user32           = syscall.MustLoadDLL("user32")
	GetDesktopWindow = user32.MustFindProc("GetDesktopWindow")
)

const (
	WAIT_OBJECT_0  = 0x00000000
	WAIT_ABANDONED = 0x00000080
	WAIT_TIMEOUT   = 0x00000102
)

type Note float64

func randomNoteGenerator(octave int, Notes []Note) <-chan Note {
	ch := make(chan Note)
	go func() {
		for {
			ch <- Notes[rand.Intn(len(Notes))] + 12.0*Note(rand.Intn(octave))
		}
	}()
	return ch
}

func sineArpeggiator(sampleRate float64, noteLen float64, noteGenerator <-chan Note) <-chan float64 {
	ch := make(chan float64)
	go func() {
		for {
			step := 2.0 * math.Pi * 440.0 * math.Pow(2.0, float64(<-noteGenerator)/12.0) / sampleRate
			for p, ln := 0.0, step*sampleRate*noteLen; p < ln; p += step {
				ch <- math.Sin(p) * (ln - p) / ln
			}
		}
	}()
	return ch
}

func infiniteFileReader() <-chan float64 {
	f, err := os.Open("48hz16bit2ch.wav")
	if err != nil {
		panic(err)
	}

	b := make([]byte, 4)
	_, err = f.ReadAt(b, 40)
	if err != nil {
		panic(err)
	}

	sz := binary.LittleEndian.Uint32(b)

	ch := make(chan float64)
	go func() {
		var err error
		size := sz
		for {
			_, err = f.Seek(44, os.SEEK_SET)
			if err != nil {
				panic(err)
			}
			for i := uint32(0); i < size; i += 4 {
				_, err = f.Read(b)
				if err != nil {
					panic(err)
				}
				ch <- float64(int16(binary.LittleEndian.Uint16(b[:2]))) / 32768
				ch <- float64(int16(binary.LittleEndian.Uint16(b[2:]))) / 32768
			}
		}
	}()
	return ch
}

func fillBuffer(dsb *dsound.IDirectSoundBuffer, blockPos int, blockSize uint32, base, foreL, foreR <-chan float64) {
	buf1, buf2, err := dsb.LockInt16s(uint32(blockPos)*blockSize, blockSize, 0)
	if err != nil {
		panic(err)
	}

	defer dsb.UnlockInt16s(buf1, buf2)

	var w float64
	for i := 0; i < len(buf1); i += 2 {
		w = (<-base + <-foreL) * 0.5
		buf1[i] = int16(w * 32767)
		w = (<-base + <-foreR) * 0.5
		buf1[i+1] = int16(w * 32767)
	}
	for i := 0; i < len(buf2); i += 2 {
		w = (<-base + <-foreL) * 0.5
		buf2[i] = int16(w * 32767)
		w = (<-base + <-foreR) * 0.5
		buf2[i+1] = int16(w * 32767)
	}
}

const (
	SampleRate  = 48000
	Bits        = 16
	Channels    = 2
	BlockAlign  = Channels * Bits / 8
	BytesPerSec = SampleRate * BlockAlign
	NumBlock    = 8
	BlockSize   = (SampleRate / NumBlock) * BlockAlign
)

func main() {
	ds, err := dsound.DirectSoundCreate(nil)
	if err != nil {
		panic(err)
	}

	desktopWindow, _, err := GetDesktopWindow.Call()
	err = ds.SetCooperativeLevel(syscall.Handle(desktopWindow), dsound.DSSCL_PRIORITY)
	if err != nil {
		panic(err)
	}

	defer ds.Release()

	// primary buffer

	primaryBuf, err := ds.CreateSoundBuffer(&dsound.BufferDesc{
		Flags:       dsound.DSBCAPS_PRIMARYBUFFER,
		BufferBytes: 0,
		Format:      nil,
	})
	if err != nil {
		panic(err)
	}

	err = primaryBuf.SetFormatWaveFormatEx(&dsound.WaveFormatEx{
		FormatTag:      dsound.WAVE_FORMAT_PCM,
		Channels:       Channels,
		SamplesPerSec:  SampleRate,
		BitsPerSample:  Bits,
		BlockAlign:     BlockAlign,
		AvgBytesPerSec: BytesPerSec,
		ExtSize:        0,
	})
	if err != nil {
		panic(err)
	}

	primaryBuf.Release()

	// secondary buffer

	secondaryBuf, err := ds.CreateSoundBuffer(&dsound.BufferDesc{
		Flags:       dsound.DSBCAPS_GLOBALFOCUS | dsound.DSBCAPS_GETCURRENTPOSITION2 | dsound.DSBCAPS_CTRLPOSITIONNOTIFY,
		BufferBytes: BlockSize * NumBlock,
		Format: &dsound.WaveFormatEx{
			FormatTag:      dsound.WAVE_FORMAT_PCM,
			Channels:       Channels,
			SamplesPerSec:  SampleRate,
			BitsPerSample:  Bits,
			BlockAlign:     Channels * Bits / 8,
			AvgBytesPerSec: BytesPerSec,
			ExtSize:        0,
		},
	})
	if err != nil {
		panic(err)
	}
	defer secondaryBuf.Release()

	wave := infiniteFileReader()
	arp := sineArpeggiator(
		SampleRate,
		1.0/4.0,
		randomNoteGenerator(2, []Note{3, 5, 7, 10}),
	)
	arp2 := sineArpeggiator(
		SampleRate,
		1.0/8.0,
		randomNoteGenerator(2, []Note{0, 7, 12}),
	)

	// fill buffer without last block

	for blockPos := 0; blockPos < NumBlock-1; blockPos++ {
		fillBuffer(secondaryBuf, blockPos, BlockSize, wave, arp, arp2)
	}

	// setup notify event

	notifies := make([]dsound.DSBPOSITIONNOTIFY, NumBlock)
	events := make([]syscall.Handle, 0)
	for i := range notifies {
		h, _, _ := CreateEvent.Call(0, 0, 0, 0)
		notifies[i].EventNotify = syscall.Handle(h)
		notifies[i].Offset = uint32(i * BlockSize)
		events = append(events, syscall.Handle(h))
	}

	notif, err := secondaryBuf.QueryInterfaceIDirectSoundNotify()
	if err != nil {
		panic(err)
	}
	defer notif.Release()

	err = notif.SetNotificationPositions(notifies)
	if err != nil {
		panic(err)
	}

	err = secondaryBuf.Play(0, dsound.DSBPLAY_LOOPING)
	if err != nil {
		panic(err)
	}

	go func() {
	outer:
		for {
			r, _, _ := WaitForMultipleObjects.Call(
				uintptr(uint32(len(events))),
				uintptr(unsafe.Pointer(&events[0])),
				0,
				0xFFFFFFFF,
			)
			switch {
			case WAIT_OBJECT_0 <= r && r < WAIT_OBJECT_0+uintptr(len(events)):
				idx := int(r - WAIT_OBJECT_0)
				blockPos := (idx - 1 + NumBlock) % NumBlock
				fillBuffer(secondaryBuf, blockPos, BlockSize, wave, arp, arp2)

			case WAIT_ABANDONED <= r && r < WAIT_ABANDONED+uintptr(len(events)):
				break outer

			case r == WAIT_TIMEOUT:
				break outer
			}
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	for _ = range sig {
		return
	}
}
