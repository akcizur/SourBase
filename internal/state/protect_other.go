//go:build !windows
package state

var stateMagic=[]byte("WAVEPLAIN1\n")
func encodeState(data []byte)([]byte,error){return append(append([]byte{},stateMagic...),data...),nil}
func decodeState(blob []byte)([]byte,bool,error){
	if len(blob)>=len(stateMagic)&&string(blob[:len(stateMagic)])==string(stateMagic){return blob[len(stateMagic):],false,nil}
	return blob,true,nil
}
