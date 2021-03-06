package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/containerd/console"
	"github.com/containerd/containerd/api/grpc/types"
	"github.com/containerd/containerd/specs"
	"github.com/golang/protobuf/ptypes"
	"github.com/urfave/cli"
	netcontext "golang.org/x/net/context"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/transport"
)

// TODO: parse flags and pass opts
func getClient(ctx *cli.Context) types.APIClient {
	// Parse proto://address form addresses.
	bindSpec := ctx.GlobalString("address")
	bindParts := strings.SplitN(bindSpec, "://", 2)
	if len(bindParts) != 2 {
		fatal(fmt.Sprintf("bad bind address format %s, expected proto://address", bindSpec), 1)
	}

	// reset the logger for grpc to log to dev/null so that it does not mess with our stdio
	grpclog.SetLogger(log.New(ioutil.Discard, "", log.LstdFlags))
	dialOpts := []grpc.DialOption{grpc.WithInsecure(), grpc.WithTimeout(ctx.GlobalDuration("conn-timeout"))}
	dialOpts = append(dialOpts,
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout(bindParts[0], bindParts[1], timeout)
		},
		))
	conn, err := grpc.Dial(bindSpec, dialOpts...)
	if err != nil {
		fatal(err.Error(), 1)
	}
	return types.NewAPIClient(conn)
}

var contSubCmds = []cli.Command{
	execCommand,
	killCommand,
	listCommand,
	pauseCommand,
	resumeCommand,
	startCommand,
	stateCommand,
	statsCommand,
	watchCommand,
	updateCommand,
}

var containersCommand = cli.Command{
	Name:        "containers",
	Usage:       "interact with running containers",
	ArgsUsage:   "COMMAND [arguments...]",
	Subcommands: contSubCmds,
	Description: func() string {
		desc := "\n    COMMAND:\n"
		for _, command := range contSubCmds {
			desc += fmt.Sprintf("    %-10.10s%s\n", command.Name, command.Usage)
		}
		return desc
	}(),
	Action: listContainers,
}

var stateCommand = cli.Command{
	Name:  "state",
	Usage: "get a raw dump of the containerd state",
	Action: func(context *cli.Context) error {
		c := getClient(context)
		resp, err := c.State(netcontext.Background(), &types.StateRequest{
			Id: context.Args().First(),
		})
		if err != nil {
			return err
		}
		data, err := json.Marshal(resp)
		if err != nil {
			return err
		}
		fmt.Print(string(data))
		return nil
	},
}

var listCommand = cli.Command{
	Name:   "list",
	Usage:  "list all running containers",
	Action: listContainers,
}

func listContainers(context *cli.Context) error {
	c := getClient(context)
	resp, err := c.State(netcontext.Background(), &types.StateRequest{
		Id: context.Args().First(),
	})
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 20, 1, 3, ' ', 0)
	fmt.Fprint(w, "ID\tPATH\tSTATUS\tPROCESSES\n")
	sortContainers(resp.Containers)
	for _, c := range resp.Containers {
		procs := []string{}
		for _, p := range c.Processes {
			procs = append(procs, p.Pid)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", c.Id, c.BundlePath, c.Status, strings.Join(procs, ","))
	}
	return w.Flush()
}

var startCommand = cli.Command{
	Name:      "start",
	Usage:     "start a container",
	ArgsUsage: "ID BundlePath",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "checkpoint,c",
			Value: "",
			Usage: "checkpoint to start the container from",
		},
		cli.StringFlag{
			Name:  "checkpoint-dir",
			Value: "",
			Usage: "path to checkpoint directory",
		},
		cli.BoolFlag{
			Name:  "attach,a",
			Usage: "connect to the stdio of the container",
		},
		cli.StringSliceFlag{
			Name:  "label,l",
			Value: &cli.StringSlice{},
			Usage: "set labels for the container",
		},
		cli.BoolFlag{
			Name:  "no-pivot",
			Usage: "do not use pivot root",
		},
		cli.StringFlag{
			Name:  "runtime,r",
			Value: "runc",
			Usage: "name or path of the OCI compliant runtime to use when executing containers",
		},
		cli.StringSliceFlag{
			Name:  "runtime-args",
			Value: &cli.StringSlice{},
			Usage: "specify additional runtime args",
		},
	},
	Action: func(context *cli.Context) error {
		var (
			id   = context.Args().Get(0)
			path = context.Args().Get(1)
		)
		if path == "" {
			fatal("bundle path cannot be empty", ExitStatusMissingArg)
		}
		if id == "" {
			fatal("container id cannot be empty", ExitStatusMissingArg)
		}
		bpath, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("cannot get the absolute path of the bundle: %v", err)
		}
		s, tmpDir, err := createStdio()
		defer func() {
			if tmpDir != "" {
				os.RemoveAll(tmpDir)
			}
		}()
		if err != nil {
			return err
		}
		var (
			con console.Console
			tty bool
			c   = getClient(context)
			r   = &types.CreateContainerRequest{
				Id:            id,
				BundlePath:    bpath,
				Checkpoint:    context.String("checkpoint"),
				CheckpointDir: context.String("checkpoint-dir"),
				Stdin:         s.stdin,
				Stdout:        s.stdout,
				Stderr:        s.stderr,
				Labels:        context.StringSlice("label"),
				NoPivotRoot:   context.Bool("no-pivot"),
				Runtime:       context.String("runtime"),
				RuntimeArgs:   context.StringSlice("runtime-args"),
			}
		)
		if context.Bool("attach") {
			mkterm, err := readTermSetting(bpath)
			if err != nil {
				return err
			}
			tty = mkterm
			if mkterm {
				con = console.Current()
				defer func() {
					con.Reset()
					con.Close()
				}()
				if err := con.SetRaw(); err != nil {
					return err
				}
			}
			if err := attachStdio(s); err != nil {
				return err
			}
		}
		events, err := c.Events(netcontext.Background(), &types.EventsRequest{})
		if err != nil {
			return err
		}
		if _, err := c.CreateContainer(netcontext.Background(), r); err != nil {
			return err
		}
		if context.Bool("attach") {
			go func() {
				io.Copy(stdin, os.Stdin)
				if _, err := c.UpdateProcess(netcontext.Background(), &types.UpdateProcessRequest{
					Id:         id,
					Pid:        "init",
					CloseStdin: true,
				}); err != nil {
					fatal(err.Error(), 1)
				}
				con.Reset()
				con.Close()
			}()
			if tty {
				resize(id, "init", c, con)
				go func() {
					s := make(chan os.Signal, 64)
					signal.Notify(s, syscall.SIGWINCH)
					for range s {
						if err := resize(id, "init", c, con); err != nil {
							log.Println(err)
						}
					}
				}()
			}
			time.Sleep(2 * time.Second)
			log.Println("closing stdin now")
			if _, err := c.UpdateProcess(netcontext.Background(), &types.UpdateProcessRequest{
				Id:         id,
				Pid:        "init",
				CloseStdin: true,
			}); err != nil {
				fatal(err.Error(), 1)
			}

			waitForExit(c, events, id, "init", con)
		}
		return nil
	},
}

func resize(id, pid string, c types.APIClient, con console.Console) error {
	ws, err := con.Size()
	if err != nil {
		return err
	}
	if _, err := c.UpdateProcess(netcontext.Background(), &types.UpdateProcessRequest{
		Id:     id,
		Pid:    "init",
		Width:  uint32(ws.Width),
		Height: uint32(ws.Height),
	}); err != nil {
		return err
	}
	return nil
}

var (
	stdin io.WriteCloser
)

// readTermSetting reads the Terminal option out of the specs configuration
// to know if ctr should allocate a pty
func readTermSetting(path string) (bool, error) {
	f, err := os.Open(filepath.Join(path, "config.json"))
	if err != nil {
		return false, err
	}
	defer f.Close()
	var spec specs.Spec
	if err := json.NewDecoder(f).Decode(&spec); err != nil {
		return false, err
	}
	return spec.Process.Terminal, nil
}

func attachStdio(s stdio) error {
	stdinf, err := os.OpenFile(s.stdin, syscall.O_RDWR, 0)
	if err != nil {
		return err
	}
	// FIXME: assign to global
	stdin = stdinf
	stdoutf, err := os.OpenFile(s.stdout, syscall.O_RDWR, 0)
	if err != nil {
		return err
	}
	go io.Copy(os.Stdout, stdoutf)
	stderrf, err := os.OpenFile(s.stderr, syscall.O_RDWR, 0)
	if err != nil {
		return err
	}
	go io.Copy(os.Stderr, stderrf)
	return nil
}

var watchCommand = cli.Command{
	Name:  "watch",
	Usage: "print container events",
	Action: func(context *cli.Context) error {
		c := getClient(context)
		id := context.Args().First()
		if id != "" {
			resp, err := c.State(netcontext.Background(), &types.StateRequest{Id: id})
			if err != nil {
				return err
			}
			for _, c := range resp.Containers {
				if c.Id == id {
					break
				}
			}
			if id == "" {
				fatal("Invalid container id", 1)
			}
		}
		events, reqErr := c.Events(netcontext.Background(), &types.EventsRequest{})
		if reqErr != nil {
			return reqErr
		}

		for {
			e, err := events.Recv()
			if err != nil {
				return err
			}

			if id == "" || e.Id == id {
				fmt.Printf("%#v\n", e)
			}
			return nil
		}
	},
}

var pauseCommand = cli.Command{
	Name:  "pause",
	Usage: "pause a container",
	Action: func(context *cli.Context) error {
		id := context.Args().First()
		if id == "" {
			fatal("container id cannot be empty", ExitStatusMissingArg)
		}
		c := getClient(context)
		_, err := c.UpdateContainer(netcontext.Background(), &types.UpdateContainerRequest{
			Id:     id,
			Pid:    "init",
			Status: "paused",
		})
		return err
	},
}

var resumeCommand = cli.Command{
	Name:  "resume",
	Usage: "resume a paused container",
	Action: func(context *cli.Context) error {
		id := context.Args().First()
		if id == "" {
			fatal("container id cannot be empty", ExitStatusMissingArg)
		}
		c := getClient(context)
		_, err := c.UpdateContainer(netcontext.Background(), &types.UpdateContainerRequest{
			Id:     id,
			Pid:    "init",
			Status: "running",
		})
		return err
	},
}

var killCommand = cli.Command{
	Name:  "kill",
	Usage: "send a signal to a container or its processes",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "pid,p",
			Value: "init",
			Usage: "pid of the process to signal within the container",
		},
		cli.IntFlag{
			Name:  "signal,s",
			Value: 15,
			Usage: "signal to send to the container",
		},
	},
	Action: func(context *cli.Context) error {
		id := context.Args().First()
		if id == "" {
			fatal("container id cannot be empty", ExitStatusMissingArg)
		}
		c := getClient(context)
		_, err := c.Signal(netcontext.Background(), &types.SignalRequest{
			Id:     id,
			Pid:    context.String("pid"),
			Signal: uint32(context.Int("signal")),
		})
		return err
	},
}

var execCommand = cli.Command{
	Name:  "exec",
	Usage: "exec another process in an existing container",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "id",
			Usage: "container id to add the process to",
		},
		cli.StringFlag{
			Name:  "pid",
			Usage: "process id for the new process",
		},
		cli.BoolFlag{
			Name:  "attach,a",
			Usage: "connect to the stdio of the container",
		},
		cli.StringFlag{
			Name:  "cwd",
			Usage: "current working directory for the process",
			Value: "/",
		},
		cli.BoolFlag{
			Name:  "tty,t",
			Usage: "create a terminal for the process",
		},
		cli.StringSliceFlag{
			Name:  "env,e",
			Value: &cli.StringSlice{},
			Usage: "environment variables for the process",
		},
		cli.IntFlag{
			Name:  "uid,u",
			Usage: "user id of the user for the process",
		},
		cli.IntFlag{
			Name:  "gid,g",
			Usage: "group id of the user for the process",
		},
	},
	Action: func(context *cli.Context) error {
		p := &types.AddProcessRequest{
			Id:       context.String("id"),
			Pid:      context.String("pid"),
			Args:     context.Args(),
			Cwd:      context.String("cwd"),
			Terminal: context.Bool("tty"),
			Env:      context.StringSlice("env"),
			User: &types.User{
				Uid: uint32(context.Int("uid")),
				Gid: uint32(context.Int("gid")),
			},
		}
		s, tmpDir, err := createStdio()
		defer func() {
			if tmpDir != "" {
				os.RemoveAll(tmpDir)
			}
		}()
		if err != nil {
			return err
		}
		p.Stdin = s.stdin
		p.Stdout = s.stdout
		p.Stderr = s.stderr

		var con console.Console
		if context.Bool("attach") {
			if context.Bool("tty") {
				con = console.Current()
				defer func() {
					con.Reset()
					con.Close()
				}()
				if err := con.SetRaw(); err != nil {
					return err
				}
			}
			if err := attachStdio(s); err != nil {
				return err
			}
		}
		c := getClient(context)
		events, err := c.Events(netcontext.Background(), &types.EventsRequest{})
		if err != nil {
			return err
		}
		if _, err := c.AddProcess(netcontext.Background(), p); err != nil {
			return err
		}
		if context.Bool("attach") {
			go func() {
				io.Copy(stdin, os.Stdin)
				if _, err := c.UpdateProcess(netcontext.Background(), &types.UpdateProcessRequest{
					Id:         p.Id,
					Pid:        p.Pid,
					CloseStdin: true,
				}); err != nil {
					log.Println(err)
				}
				con.Reset()
				con.Close()
			}()
			if context.Bool("tty") {
				resize(p.Id, p.Pid, c, con)
				go func() {
					s := make(chan os.Signal, 64)
					signal.Notify(s, syscall.SIGWINCH)
					for range s {
						if err := resize(p.Id, p.Pid, c, con); err != nil {
							log.Println(err)
						}
					}
				}()
			}
			waitForExit(c, events, context.String("id"), context.String("pid"), con)
		}
		return nil
	},
}

var statsCommand = cli.Command{
	Name:  "stats",
	Usage: "get stats for running container",
	Action: func(context *cli.Context) error {
		req := &types.StatsRequest{
			Id: context.Args().First(),
		}
		c := getClient(context)
		stats, err := c.Stats(netcontext.Background(), req)
		if err != nil {
			return err
		}
		data, err := json.Marshal(stats)
		if err != nil {
			return err
		}
		fmt.Print(string(data))
		return nil
	},
}

func getUpdateCommandInt64Flag(context *cli.Context, name string) int64 {
	str := context.String(name)
	if str == "" {
		return 0
	}

	val, err := strconv.ParseInt(str, 0, 64)
	if err != nil {
		fatal(err.Error(), 1)
	}

	return val
}

func getUpdateCommandUInt64Flag(context *cli.Context, name string) uint64 {
	str := context.String(name)
	if str == "" {
		return 0
	}

	val, err := strconv.ParseUint(str, 0, 64)
	if err != nil {
		fatal(err.Error(), 1)
	}

	return val
}

var updateCommand = cli.Command{
	Name:  "update",
	Usage: "update a containers resources",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name: "memory-limit",
		},
		cli.StringFlag{
			Name: "memory-reservation",
		},
		cli.StringFlag{
			Name: "memory-swap",
		},
		cli.StringFlag{
			Name: "cpu-quota",
		},
		cli.StringFlag{
			Name: "cpu-period",
		},
		cli.StringFlag{
			Name: "kernel-limit",
		},
		cli.StringFlag{
			Name: "kernel-tcp-limit",
		},
		cli.StringFlag{
			Name: "blkio-weight",
		},
		cli.StringFlag{
			Name: "cpuset-cpus",
		},
		cli.StringFlag{
			Name: "cpuset-mems",
		},
		cli.StringFlag{
			Name: "pids-limit",
		},
		cli.StringFlag{
			Name: "cpu-realtime-period",
		},
		cli.StringFlag{
			Name: "cpu-realtime-runtime",
		},
	},
	Action: func(context *cli.Context) error {
		req := &types.UpdateContainerRequest{
			Id: context.Args().First(),
		}
		req.Resources = &types.UpdateResource{}
		req.Resources.MemoryLimit = getUpdateCommandUInt64Flag(context, "memory-limit")
		req.Resources.MemoryReservation = getUpdateCommandUInt64Flag(context, "memory-reservation")
		req.Resources.MemorySwap = getUpdateCommandUInt64Flag(context, "memory-swap")
		req.Resources.BlkioWeight = getUpdateCommandUInt64Flag(context, "blkio-weight")
		req.Resources.CpuPeriod = getUpdateCommandUInt64Flag(context, "cpu-period")
		req.Resources.CpuQuota = getUpdateCommandUInt64Flag(context, "cpu-quota")
		req.Resources.CpuShares = getUpdateCommandUInt64Flag(context, "cpu-shares")
		req.Resources.CpusetCpus = context.String("cpuset-cpus")
		req.Resources.CpusetMems = context.String("cpuset-mems")
		req.Resources.KernelMemoryLimit = getUpdateCommandUInt64Flag(context, "kernel-limit")
		req.Resources.KernelTCPMemoryLimit = getUpdateCommandUInt64Flag(context, "kernel-tcp-limit")
		req.Resources.PidsLimit = getUpdateCommandUInt64Flag(context, "pids-limit")
		req.Resources.CpuRealtimePeriod = getUpdateCommandUInt64Flag(context, "cpu-realtime-period")
		req.Resources.CpuRealtimeRuntime = getUpdateCommandInt64Flag(context, "cpu-realtime-runtime")
		c := getClient(context)
		_, err := c.UpdateContainer(netcontext.Background(), req)
		return err
	},
}

func waitForExit(c types.APIClient, events types.API_EventsClient, id, pid string, con console.Console) {
	timestamp := time.Now()
	for {
		e, err := events.Recv()
		if err != nil {
			if grpc.ErrorDesc(err) == transport.ErrConnClosing.Desc {
				if con != nil {
					con.Reset()
					con.Close()
				}
				os.Exit(128 + int(syscall.SIGHUP))
			}
			time.Sleep(1 * time.Second)
			tsp, err := ptypes.TimestampProto(timestamp)
			if err != nil {
				if con != nil {
					con.Reset()
					con.Close()
				}
				fmt.Fprintf(os.Stderr, "%s", err.Error())
				os.Exit(1)
			}
			events, _ = c.Events(netcontext.Background(), &types.EventsRequest{Timestamp: tsp})
			continue
		}
		timestamp, err = ptypes.Timestamp(e.Timestamp)
		if e.Id == id && e.Type == "exit" && e.Pid == pid {
			if con != nil {
				con.Reset()
				con.Close()
			}
			os.Exit(int(e.Status))
		}
	}
}

type stdio struct {
	stdin  string
	stdout string
	stderr string
}

func createStdio() (s stdio, tmp string, err error) {
	tmp, err = ioutil.TempDir("", "ctr-")
	if err != nil {
		return s, tmp, err
	}
	// create fifo's for the process
	for _, pair := range []struct {
		name string
		fd   *string
	}{
		{"stdin", &s.stdin},
		{"stdout", &s.stdout},
		{"stderr", &s.stderr},
	} {
		path := filepath.Join(tmp, pair.name)
		if err := unix.Mkfifo(path, 0755); err != nil && !os.IsExist(err) {
			return s, tmp, err
		}
		*pair.fd = path
	}
	return s, tmp, nil
}
