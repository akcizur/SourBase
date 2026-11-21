//go:build windows
package state

import("errors";"runtime";"syscall";"unsafe")
type dataBlob struct{size uint32;data *byte}
var crypt32=syscall.NewLazyDLL("crypt32.dll")
var kernel32=syscall.NewLazyDLL("kernel32.dll")
var protectProc=crypt32.NewProc("CryptProtectData")
var unprotectProc=crypt32.NewProc("CryptUnprotectData")
var localFree=kernel32.NewProc("LocalFree")
var stateMagic=[]byte("WAVEDPAPI1\n")
func dpapi(input []byte,protect bool)([]byte,error){
	if len(input)==0{return []byte{},nil}
	in:=dataBlob{size:uint32(len(input)),data:&input[0]};var out dataBlob;proc:=protectProc;if !protect{proc=unprotectProc}
	if err:=proc.Find();err!=nil{return nil,err}
	ok,_,callErr:=proc.Call(uintptr(unsafe.Pointer(&in)),0,0,0,0,1,uintptr(unsafe.Pointer(&out)));runtime.KeepAlive(input)
	if ok==0{if callErr!=syscall.Errno(0){return nil,callErr};return nil,errors.New("DPAPI operation failed")}
	if out.data==nil||out.size==0{return nil,errors.New("DPAPI returned empty data")}
	defer localFree.Call(uintptr(unsafe.Pointer(out.data)))
	return append([]byte(nil),unsafe.Slice(out.data,int(out.size))...),nil
}
func encodeState(data []byte)([]byte,error){encrypted,err:=dpapi(data,true);if err!=nil{return nil,err};return append(append([]byte{},stateMagic...),encrypted...),nil}
func decodeState(blob []byte)([]byte,bool,error){
	if len(blob)>=len(stateMagic)&&string(blob[:len(stateMagic)])==string(stateMagic){plain,err:=dpapi(blob[len(stateMagic):],false);return plain,false,err}
	return nil,false,errors.New("legacy plaintext state is not supported by this Windows build")
}
