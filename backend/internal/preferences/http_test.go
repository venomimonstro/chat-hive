package preferences
import "testing"
func TestValidLanguage(t *testing.T) {
 for _,value:=range []string{"ru","en"} {if !validLanguage(value){t.Fatalf("valid language rejected %s",value)}}
 for _,value:=range []string{"RU","EN","es","","en-US","ru-RU","../../etc/passwd"} {if validLanguage(value){t.Fatalf("unexpected language accepted %s",value)}}
}
