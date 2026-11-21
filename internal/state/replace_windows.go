//go:build windows
package state
import("fmt";"syscall";"unsafe")
var moveFileEx=syscall.NewLazyDLL("kernel32.dll").NewProc("MoveFileExW")
func replaceFile(oldPath,newPath string)error{
	oldPtr,err:=syscall.UTF16PtrFromString(oldPath);if err!=nil{return err}
	newPtr,err:=syscall.UTF16PtrFromString(newPath);if err!=nil{return err}
	ok,_,callErr:=moveFileEx.Call(uintptr(unsafe.Pointer(oldPtr)),uintptr(unsafe.Pointer(newPtr)),0x1|0x8)
	if ok==0{return fmt.Errorf("replace local state file: %w",callErr)}
	return nil
}
