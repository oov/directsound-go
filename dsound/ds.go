package dsound

import (
	"syscall"
	"unsafe"
)

var (
	dsoundDLL                       = syscall.NewLazyDLL("dsound")
	procDirectSoundEnumerate        = dsoundDLL.NewProc("DirectSoundEnumerateW")
	procDirectSoundCaptureEnumerate = dsoundDLL.NewProc("DirectSoundCaptureEnumerateW")
	procDirectSoundCreate           = dsoundDLL.NewProc("DirectSoundCreate")
	//procDirectSoundCreate8          = dsoundDLL.NewProc("DirectSoundCreate8")
	//procDirectSoundCaptureCreate = dsoundDLL.NewProc("DirectSoundCaptureCreate")
	//procDirectSoundCaptureCreate8   = dsoundDLL.NewProc("DirectSoundCaptureCreate8")
)

func DirectSoundEnumerate(dsEnumCallback func(guid *GUID, description string, module string) bool) error {
	return dllDSResult(procDirectSoundEnumerate.Call(syscall.NewCallback(func(guid *GUID, description uintptr, module uintptr, context uintptr) int {
		b := dsEnumCallback(
			guid,
			syscall.UTF16ToString((*[1024]uint16)(unsafe.Pointer(description))[:]),
			syscall.UTF16ToString((*[1024]uint16)(unsafe.Pointer(module))[:]),
		)
		if b {
			return 1
		}
		return 0
	}), 0))
}

func DirectSoundCaptureEnumerate(dsEnumCallback func(guid *GUID, description string, module string) bool) error {
	return dllDSResult(procDirectSoundCaptureEnumerate.Call(syscall.NewCallback(func(guid *GUID, description uintptr, module uintptr, context uintptr) int {
		b := dsEnumCallback(
			guid,
			syscall.UTF16ToString((*[1024]uint16)(unsafe.Pointer(description))[:]),
			syscall.UTF16ToString((*[1024]uint16)(unsafe.Pointer(module))[:]),
		)
		if b {
			return 1
		}
		return 0
	}), 0))
}

func DirectSoundCreate(guid *GUID) (*IDirectSound, error) {
	var ds *IDirectSound
	err := dllDSResult(procDirectSoundCreate.Call(
		uintptr(unsafe.Pointer(guid)),
		uintptr(unsafe.Pointer(&ds)),
		0,
	))
	if err != nil {
		return nil, err
	}
	return ds, nil
}

/*
func DirectSoundCreate8(guid *GUID) (*IDirectSound8, error) {
	var ds *IDirectSound8
	err := dllDSResult(procDirectSoundCreate8.Call(
		uintptr(unsafe.Pointer(guid)),
		uintptr(unsafe.Pointer(&ds)),
		0,
	))
	if err != nil {
		return nil, err
	}
	return ds, nil
}

func DirectSoundCaptureCreate(guid *GUID) (*DirectSoundCapture, error) {
	return nil, nil
}

func DirectSoundCaptureCreate8(guid *GUID) (*IDirectSoundCapture8, error) {
	return nil, nil
}

func DirectSoundFullDuplexCreate8(){
}

func GetDeviceID(){
}
*/
