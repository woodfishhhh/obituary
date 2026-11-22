package docker

import "testing"

func TestVolumeReferencesIncludeVisibleBindOverlaps(t *testing.T) {
 const point="/daemon/volumes/cache/_data"
 for _,tc:=range []struct{source string; used bool}{
 {point,true},{"/daemon/volumes/cache",true},{point+"/db",true},{"/",true},
 {"/work/app",false},{point+"-copy",false},{"/daemon/volumes/CACHE/_data",false},
 } { for _,running:=range []bool{true,false} {
  c:=Container{Running:running,Binds:[]string{tc.source}}
  if got,err:=volumeReference(c,"cache",point);err!=nil || got!=tc.used {t.Errorf("running=%t bind %s: used=%t err=%v",running,tc.source,got,err)}
 } }
}
