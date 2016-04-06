package tests

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"testing"
	"time"

	proxybinary "github.com/GoogleCloudPlatform/cloudsql-proxy/cmd/cloud_sql_proxy"

	"golang.org/x/crypto/ssh"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	compute "google.golang.org/api/compute/v1"
)

var (
	runProxy = flag.Bool("proxy_mode", false, "When set, this binary will invoke the cloud_sql_proxy process instead of just running tests")
	project  = flag.String("project", "", "Project to create the GCE test VM in")
	zone     = flag.String("zone", "us-central1-f", "Zone in which to create the VM")
	osImage  = flag.String("os", defaultOS, "OS image to use when creating a VM")
	vmName   = flag.String("vm_name", "proxy-test-gce", "Name of VM to create")
)

const defaultOS = "https://www.googleapis.com/compute/v1/projects/debian-cloud/global/images/debian-8-jessie-v20160329"

// TestGCE provisions a new GCE VM and verifies that the proxy works on it.
// It uses application default credentials.
func TestGCE(t *testing.T) {
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Minute)
	cl, err := google.DefaultClient(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		t.Fatal(err)
	}

	ssh, err := newOrReuseVM(t, cl)
	if err != nil {
		t.Fatal(err)
	}

	log.Printf("Copy binary to %v...", *vmName)
	// Upload a copy of this binary to the remote host
	this, err := os.Open(os.Args[0])
	if err != nil {
		t.Fatalf("Couldn't open %v for reading: %v", os.Args[0])
	}
	if err := sshRun(ssh, "bash -c 'cat >cloud_sql_proxy; chmod +x cloud_sql_proxy; mkdir -p cloudsql'", this, nil, nil); err != nil {
		t.Fatalf("couldn't scp to remote machine: %v", err)
	}
	this.Close()

	logs, err := startProxy(ssh, "./cloud_sql_proxy -proxy_mode -dir cloudsql -instances speckle-dogfood-chowski:perf")
	if err != nil {
		t.Fatal(err)
	}
	go io.Copy(ioutil.Discard, logs)

	log.Printf("Install mysql client...")
	defer logs.Close()

	if err := sshRun(ssh, "sudo apt-get install -y mysql-client-5.5", nil, nil, nil); err != nil {
		t.Fatal(err)
	}

	var sout, serr bytes.Buffer
	err = sshRun(ssh, `mysql -uroot --password=asdf -S cloudsql/speckle-dogfood-chowski:perf -e "select * from mysql.user\\G"`, nil, &sout, &serr)
	if err != nil {
		t.Fatalf("Error running mysql: %v\n\nstandard out:\n%s\nstandard err:\n%s", err, &sout, &serr)
	}
	t.Log(&sout)
}

var _ io.ReadCloser = (*process)(nil)

// process wraps a remotely executing process, turning it into an
// io.ReadCloser.
type process struct {
	io.Reader
	sess *ssh.Session
}

// TODO: Return the value of 'Wait'ing on the process. ssh.Session.Signal
// doesn't seem to have an effect, so calling it and then doing Wait doesn't do
// anything. Closing the session is the only way to clean up until I figure out
// what's wrong.
func (p *process) Close() error {
	return p.sess.Close()
}

func startProxy(ssh *ssh.Client, args string) (io.ReadCloser, error) {
	sess, err := ssh.NewSession()
	if err != nil {
		return nil, fmt.Errorf("couldn't open new session: %v", err)
	}
	pr, pw := io.Pipe()
	sess.Stderr = pw
	log.Printf("Running proxy...")
	if err := sess.Start(args); err != nil {
		return nil, err
	}

	in := bufio.NewReader(pr)
	buf := new(bytes.Buffer)
	errCh := make(chan error, 1)
	go func() {
		for {
			bs, err := in.ReadBytes('\n')
			if err != nil {
				if err == io.EOF {
					log.Print("reading stderr gave EOF")
					err = sess.Wait()
				}
				errCh <- fmt.Errorf("failed to run `%s`: %v", args, sess.Wait())
				return
			}
			buf.Write(bs)
			if bytes.Contains(bs, []byte("Ready for new connections")) {
				errCh <- nil
				return
			}
		}
	}()

	select {
	case err := <-errCh:
		if err != nil {
			return nil, err
		}
	case <-time.After(3 * time.Second):
		err := sess.Close()
		select {
		case waitErr := <-errCh:
			if err == nil {
				err = waitErr
			}
		case <-time.After(2 * time.Second):
			log.Printf("Timeout while waiting for process")
		}
		log.Printf("Timeout starting up `%v`", args)
		return nil, fmt.Errorf("timeout waiting for `%v`: error from close: %v; output was:\n\n%s", args, sess.Close(), buf)
	}
	return &process{
		io.MultiReader(buf, in),
		sess,
	}, nil
}

func sshRun(ssh *ssh.Client, cmd string, stdin io.Reader, stdout, stderr io.Writer) error {
	sess, err := ssh.NewSession()
	if err != nil {
		return err
	}

	sess.Stdin = stdin
	if stderr == nil && stdout == nil {
		if out, err := sess.CombinedOutput(cmd); err != nil {
			return fmt.Errorf("`%v`: %v; combined output was:\n%s", cmd, err, out)
		}
		return nil
	}
	sess.Stdout = stdout
	sess.Stderr = stderr

	return sess.Run(cmd)
}

func newOrReuseVM(t *testing.T, cl *http.Client) (*ssh.Client, error) {
	c, err := compute.New(cl)
	if err != nil {
		t.Fatal(err)
	}

	user := "test-user"
	pub, auth := sshKey()
	sshPubKey := user + ":" + pub

	var op *compute.Operation

	if inst, err := c.Instances.Get(*project, *zone, *vmName).Do(); err != nil {
		t.Logf("Get instance %v (in project %v and zone %v) failed: %v", *vmName, *project, *zone, err)
		t.Logf("Creating new instance...")
		instProto := &compute.Instance{
			Name:        *vmName,
			MachineType: "zones/" + *zone + "/machineTypes/g1-small",
			Disks: []*compute.AttachedDisk{{
				AutoDelete: true,
				Boot:       true,
				InitializeParams: &compute.AttachedDiskInitializeParams{
					SourceImage: *osImage,
					DiskSizeGb:  10,
				}},
			},
			NetworkInterfaces: []*compute.NetworkInterface{{
				Network:       "projects/" + *project + "/global/networks/default",
				AccessConfigs: []*compute.AccessConfig{{Name: "External NAT", Type: "ONE_TO_ONE_NAT"}},
			}},
			Metadata: &compute.Metadata{
				Items: []*compute.MetadataItems{{
					Key: "sshKeys", Value: &sshPubKey,
				}},
			},
			Tags: &compute.Tags{Items: []string{"ssh"}},
			ServiceAccounts: []*compute.ServiceAccount{{
				Email:  "default",
				Scopes: []string{"https://www.googleapis.com/auth/cloud-platform"},
			}},
		}
		op, err = c.Instances.Insert(*project, *zone, instProto).Do()
	} else {
		t.Logf("attempting to reuse instance %v (in project %v and zone %v)...", *vmName, *project, *zone)
		set := false
		md := inst.Metadata
		for _, v := range md.Items {
			if v.Key == "sshKeys" {
				v.Value = &sshPubKey
				set = true
				break
			}
		}
		if !set {
			md.Items = append(md.Items, &compute.MetadataItems{Key: "sshKeys", Value: &sshPubKey})
		}
		op, err = c.Instances.SetMetadata(*project, *zone, *vmName, md).Do()
	}

	for {
		if err == nil && op.Error != nil && len(op.Error.Errors) > 0 {
			err = fmt.Errorf("errors: %v", op.Error.Errors)
		}
		if err != nil {
			t.Fatalf("Could not set up instance: %v\n\n%v", err, op)
		}

		log.Printf("%v %v (%v)", op.OperationType, op.TargetLink, op.Status)
		if op.Status == "DONE" {
			break
		}
		time.Sleep(5 * time.Second)

		op, err = c.ZoneOperations.Get(*project, *zone, op.Name).Do()
	}

	inst, err := c.Instances.Get(*project, *zone, *vmName).Do()
	if err != nil {
		t.Fatalf("Error getting instance after it's created: %v", err)
	}
	ip := inst.NetworkInterfaces[0].AccessConfigs[0].NatIP

	ssh, err := ssh.Dial("tcp", ip+":22", &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{auth},
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't ssh to %v (IP=%v): %v", *vmName, ip, err)
	}
	return ssh, nil
}

func sshKey() (pubKey string, auth ssh.AuthMethod) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		panic(err)
	}
	pub, err := ssh.NewPublicKey(&key.PublicKey)
	if err != nil {
		panic(err)
	}
	return string(ssh.MarshalAuthorizedKey(pub)), ssh.PublicKeys(signer)
}

func TestMain(m *testing.M) {
	flag.Parse()
	if *runProxy {
		proxybinary.Main()
		return
	}
	fmt.Println(os.Args)
	if *project == "" {
		log.Fatal("Must set -project")
	}

	os.Exit(m.Run())
}
