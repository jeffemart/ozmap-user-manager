//go:build ignore
package main
import("fmt";"os";"github.com/example/ozmap-user-manager/internal/security")
func main(){if len(os.Args)!=2{panic("usage: go run ./cmd/hashadmin.go 'senha'")};x,e:=security.HashAdmin(os.Args[1]);if e!=nil{panic(e)};fmt.Println(x)}
