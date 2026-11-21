//go:build windows

package main

import (
	"errors"
	"runtime"
	"syscall"
	"unsafe"
)

const cryptProtectUIForbidden = 0x1

type dataBlob struct {
	cbData uint32
	pbData *byte
}

var (
	crypt32DLL             = syscall.NewLazyDLL("crypt32.dll")
	kernel32DLL            = syscall.NewLazyDLL("kernel32.dll")
	cryptProtectDataProc   = crypt32DLL.NewProc("CryptProtectData")
	cryptUnprotectDataProc = crypt32DLL.NewProc("CryptUnprotectData")
	localFreeProc          = kernel32DLL.NewProc("LocalFree")
)

// protectSecret uses Windows DPAPI with the current Windows user scope. The
// encrypted value can only be decrypted by the same user account on this PC.
func protectSecret(plain []byte) ([]byte, error) {
	return dpapiTransform(plain, true)
}

func unprotectSecret(ciphertext []byte) ([]byte, error) {
	return dpapiTransform(ciphertext, false)
}

func dpapiTransform(input []byte, protect bool) ([]byte, error) {
	if len(input) == 0 {
		return []byte{}, nil
	}
	in := dataBlob{cbData: uint32(len(input)), pbData: &input[0]}
	var out dataBlob
	var proc *syscall.LazyProc
	if protect {
		proc = cryptProtectDataProc
	} else {
		proc = cryptUnprotectDataProc
	}
	if err := proc.Find(); err != nil {
		return nil, err
	}
	result, _, callErr := proc.Call(
		uintptr(unsafe.Pointer(&in)),
		0, 0, 0, 0,
		cryptProtectUIForbidden,
		uintptr(unsafe.Pointer(&out)),
	)
	runtime.KeepAlive(input)
	if result == 0 {
		if callErr != syscall.Errno(0) {
			return nil, callErr
		}
		return nil, errors.New("Windows DPAPI operation failed")
	}
	if out.pbData == nil || out.cbData == 0 {
		return nil, errors.New("Windows DPAPI returned empty data")
	}
	defer localFreeProc.Call(uintptr(unsafe.Pointer(out.pbData)))
	resultBytes := append([]byte(nil), unsafe.Slice(out.pbData, int(out.cbData))...)
	return resultBytes, nil
}
