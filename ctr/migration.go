package main

import (
	"fmt"
	"github.com/containerd/containerd/api/grpc/types"
	"github.com/urfave/cli"
	netcontext "golang.org/x/net/context"
	"os"
)

var migrationCommand = cli.Command{
	Name:      "migration",
	Usage:     "migration containers",
	ArgsUsage: "<container-id> <ip> <port> || mysql 192.168.18.2 9001",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "host,H",
			Usage: "host ip address",
		},
		cli.UintFlag{
			Name:  "port,p",
			Usage: "host port",
		},
	},
	Action: func(context *cli.Context) error {
		if err := checkArgs(context, 1); err != nil {
			return err
		}

		id := context.Args().First()
		ip := context.String("host")
		port := context.Uint("port")
		fmt.Printf("id:%v, ip %v, port:%v\n", id, ip, port)
		c := getClient(context)
		machine := &types.TargetMachine{
			Ip:   ip,
			Port: uint32(port),
		}
		_, err := c.Migration(netcontext.Background(), &types.MigrationRequest{
			Id:            id,
			Targetmachine: machine,
		})
		if err != nil {
			fmt.Println(err)
			return err
		}
		fmt.Println("after rpc!")
		return nil
	},
}

func checkArgs(context *cli.Context, expected int) error {
	var err error
	cmdName := context.Command.Name
	fmt.Printf("nums is %v\n", context.NArg())
	if context.NArg() != expected {
		err = fmt.Errorf("%s: %q requires exactly %d argument(s)", os.Args[0], cmdName, expected)
	}
	if err != nil {
		fmt.Printf("Incorrect Usage.\n\n")
		return err
	}
	return nil
}
