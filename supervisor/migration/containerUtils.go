package migration

import (
	"github.com/containerd/containerd/runtime"
	"github.com/containerd/containerd/specs"
	"os"
	"path/filepath"
	"encoding/json"
	"google.golang.org/grpc"
	"github.com/containerd/containerd/api/grpc/types"
	"google.golang.org/grpc/grpclog"
	"io/ioutil"
	"log"
	"time"
	"net"
	"fmt"
)

func LoadSpec(c runtime.Container) (*specs.Spec,error) {
	var spec specs.Spec
	f,err:=os.Open(filepath.Join(c.Path(),"config.json"))
	if err!=nil {
		return nil,err
	}
	defer f.Close()
	if err := json.NewDecoder(f).Decode(&spec); err != nil {
		return nil, err
	}
	return &spec,nil
}

func getClient(ip string,port uint32) (types.APIClient,error)  {
	bindSpec:=fmt.Sprintf("tcp://%v:%d",ip,port)
	grpclog.SetLogger(log.New(ioutil.Discard, "", log.LstdFlags))
	dialOpts := []grpc.DialOption{grpc.WithInsecure(), grpc.WithTimeout(1*time.Second)}
	dialOpts=append(dialOpts,
		grpc.WithDialer(func(s string, duration time.Duration) (net.Conn, error) {
			return net.DialTimeout("tcp",fmt.Sprintf("%v:%d",ip,port),duration)
		},
		))
	conn, err := grpc.Dial(bindSpec, dialOpts...)
	if err!=nil {
		return nil,err
	}
	return types.NewAPIClient(conn),nil

}
