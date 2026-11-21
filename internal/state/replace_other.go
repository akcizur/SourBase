//go:build !windows
package state
import "os"
func replaceFile(oldPath,newPath string)error{return os.Rename(oldPath,newPath)}
