package vmbackend

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// Golden fc-config.json bytes and launchConfigHash values captured from
// vmmanager.buildFirecrackerConfig at 980ffaf4e (StateDir /state, VM
// vm-golden-0001, host port 9001 with balloon and credential disk; 9002
// without). If vmmanager's config changes, these goldens and the migration
// equivalence test must change in the same commit, with a note that every
// committed snapshot will be refused once (launch_config_mismatch).
const (
	goldenFCConfig1 = `{"balloon":{"amount_mib":0,"deflate_on_oom":true,"free_page_reporting":true,"stats_polling_interval_s":5},"boot-source":{"boot_args":"root=fstab init=/nix/store/x/init regInfo=/nix/store/y console=ttyS0,115200 reboot=k panic=1 i8042.noaux i8042.nomux i8042.nopnp i8042.dumbkbd guest_port=8085 persistent=/mnt/persistent vm_id=vm-golden-0001 epoch=7 choir.gateway_url=http://10.200.1.1:8084 choir.vmctl_url=http://10.200.1.1:8083 choir.maild_url=http://10.200.1.1:8087 choir.platform_url=http://10.200.1.1:8086 choir.proxy_url=http://10.200.1.1:8082 ip=10.200.1.2::10.200.1.1:255.255.255.252::eth0:off choir.computer_id=computer-1","initrd_path":"/g/initrd","kernel_image_path":"/g/vmlinux"},"drives":[{"drive_id":"store","is_read_only":true,"is_root_device":false,"path_on_host":"/g/store.erofs"},{"drive_id":"data","is_read_only":false,"is_root_device":false,"path_on_host":"/state/vm-golden-0001/data.img"},{"drive_id":"credential","is_read_only":false,"is_root_device":false,"path_on_host":"/state/vm-golden-0001/credential.img"}],"machine-config":{"mem_size_mib":8192,"vcpu_count":4},"network-interfaces":[{"guest_mac":"AA:FC:00:00:00:29","host_dev_name":"vm-fwp4r7mxbe3r","iface_id":"eth0"}]}`
	goldenFCHash1   = "e8ae0e45797c298f28b4defa0c39983e0c145a4c20365c12896cf3cde1853cd5"
	goldenFCConfig2 = `{"boot-source":{"boot_args":"root=fstab init=/nix/store/x/init regInfo=/nix/store/y console=ttyS0,115200 reboot=k panic=1 i8042.noaux i8042.nomux i8042.nopnp i8042.dumbkbd guest_port=8085 persistent=/mnt/persistent vm_id=vm-golden-0001 epoch=7 choir.gateway_url=http://10.200.2.1:8084 choir.vmctl_url=http://10.200.2.1:8083 choir.maild_url=http://10.200.2.1:8087 choir.platform_url=http://10.200.2.1:8086 choir.proxy_url=http://10.200.2.1:8082 ip=10.200.2.2::10.200.2.1:255.255.255.252::eth0:off choir.computer_id=computer-1","kernel_image_path":"/g/vmlinux"},"drives":[{"drive_id":"store","is_read_only":true,"is_root_device":false,"path_on_host":"/g/store.erofs"},{"drive_id":"data","is_read_only":false,"is_root_device":false,"path_on_host":"/state/vm-golden-0001/data.img"}],"machine-config":{"mem_size_mib":8192,"vcpu_count":4},"network-interfaces":[{"guest_mac":"AA:FC:00:00:00:2A","host_dev_name":"vm-fwp4r7mxbe3r","iface_id":"eth0"}]}`
	goldenFCHash2   = "484a596fa69f53c4f56b7ba6644329962e94f4e9f3d18a1750643e274d477b7e"
)

func goldenBootArgs(port int) string {
	subnet := map[int]string{9001: "10.200.1", 9002: "10.200.2"}[port]
	return "root=fstab init=/nix/store/x/init regInfo=/nix/store/y console=ttyS0,115200 reboot=k panic=1 i8042.noaux i8042.nomux i8042.nopnp i8042.dumbkbd guest_port=8085 persistent=/mnt/persistent vm_id=vm-golden-0001 epoch=7 " +
		"choir.gateway_url=http://" + subnet + ".1:8084 choir.vmctl_url=http://" + subnet + ".1:8083 choir.maild_url=http://" + subnet + ".1:8087 choir.platform_url=http://" + subnet + ".1:8086 choir.proxy_url=http://" + subnet + ".1:8082 " +
		"ip=" + subnet + ".2::" + subnet + ".1:255.255.255.252::eth0:off choir.computer_id=computer-1"
}

func TestFirecrackerConfigMatchesVMManagerGolden(t *testing.T) {
	spec := MachineSpec{
		VMID:     "vm-golden-0001",
		StateDir: "/state/vm-golden-0001",
		Kernel:   "/g/vmlinux",
		Initrd:   "/g/initrd",
		Cmdline:  goldenBootArgs(9001),
		VCPUs:    4,
		MemMiB:   8192,
		Drives: []Drive{
			{ID: "store", Path: "/g/store.erofs", ReadOnly: true},
			{ID: "data", Path: "/state/vm-golden-0001/data.img"},
			{ID: "credential", Path: "/state/vm-golden-0001/credential.img"},
		},
		Net:     &NetIface{ID: "eth0", TapName: "vm-fwp4r7mxbe3r", MAC: "AA:FC:00:00:00:29"},
		Balloon: &Balloon{AmountMiB: 0, DeflateOnOOM: true, FreePageReporting: true, StatsPollingIntervalS: 5},
	}
	plan, err := FirecrackerBackend{}.PlanLaunch(spec)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(plan.WriteFiles[0].Data); got != goldenFCConfig1 {
		t.Fatalf("config drifted from vmmanager golden:\n got %s\nwant %s", got, goldenFCConfig1)
	}
	if plan.ConfigDigest != goldenFCHash1 {
		t.Fatalf("digest %s, want vmmanager launchConfigHash %s", plan.ConfigDigest, goldenFCHash1)
	}

	spec.Initrd = ""
	spec.Cmdline = goldenBootArgs(9002)
	spec.Drives = spec.Drives[:2]
	spec.Net.MAC = "AA:FC:00:00:00:2A"
	spec.Balloon = nil
	plan, err = FirecrackerBackend{}.PlanLaunch(spec)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(plan.WriteFiles[0].Data); got != goldenFCConfig2 {
		t.Fatalf("config drifted from vmmanager golden:\n got %s\nwant %s", got, goldenFCConfig2)
	}
	if plan.ConfigDigest != goldenFCHash2 {
		t.Fatalf("digest %s, want %s", plan.ConfigDigest, goldenFCHash2)
	}
}

func TestFirecrackerLaunchArgsMatchVMManager(t *testing.T) {
	spec := baseSpec(t, t.TempDir(), "vm-a")
	spec.Net = &NetIface{ID: "eth0", TapName: "vm-test", MAC: "AA:FC:00:00:00:01"}
	cfgPath := filepath.Join(spec.StateDir, "fc-config.json")

	plan, err := FirecrackerBackend{}.PlanLaunch(spec)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(plan.Args, " "), "--no-api --enable-pci --id vm-a --config-file "+cfgPath; got != want {
		t.Fatalf("kill-mode args %q, want %q", got, want)
	}
	if plan.WriteFiles[0].Path != cfgPath || plan.WriteFiles[0].Mode != 0o644 {
		t.Fatalf("config file %+v", plan.WriteFiles[0])
	}

	spec.APISocketPath = filepath.Join(spec.StateDir, "api", "fc.sock")
	plan, err = FirecrackerBackend{}.PlanLaunch(spec)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(plan.Args, " "), "--api-sock "+spec.APISocketPath+" --enable-pci --id vm-a --config-file "+cfgPath; got != want {
		t.Fatalf("snapshot-mode args %q, want %q", got, want)
	}

	restore, err := FirecrackerBackend{}.PlanRestore(spec, filepath.Join(spec.StateDir, "snapshot"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(restore.Args, " "), "--api-sock "+spec.APISocketPath+" --enable-pci --id vm-a"; got != want {
		t.Fatalf("restore args %q, want %q", got, want)
	}

	spec.APISocketPath = ""
	if _, err := (FirecrackerBackend{}).PlanRestore(spec, "/snap"); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("restore without API socket: %v, want ErrUnsupported", err)
	}
}

// The same cases as vmmanager's TestFirecrackerCmdlineBytesMatchVM.
func TestFirecrackerVMIDFromCmdline(t *testing.T) {
	fc := FirecrackerBackend{}
	for _, raw := range []string{
		"/nix/store/firecracker/bin/firecracker\x00--no-api\x00--id\x00vm-existing-user\x00--config-file\x00/state/fc-config.json",
		"firecracker --no-api --id=vm-existing-user --config-file /state/fc-config.json",
	} {
		if id, ok := fc.VMIDFromCmdline(SplitCmdline([]byte(raw))); !ok || id != "vm-existing-user" {
			t.Fatalf("%q -> %q,%v", raw, id, ok)
		}
	}
	for _, raw := range []string{
		"/bin/sleep\x0060",
		"",
		"firecracker --no-api --config-file /state/fc-config.json",
		"/usr/bin/cloud-hypervisor\x00--cmdline\x00vm_id=vm-existing-user",
	} {
		if id, ok := fc.VMIDFromCmdline(SplitCmdline([]byte(raw))); ok {
			t.Fatalf("%q identified as %q", raw, id)
		}
	}
}

func TestFirecrackerAPIWithoutSocketUnsupported(t *testing.T) {
	h := Handle{Backend: Firecracker, VMID: "vm-a", PID: 1}
	if err := (FirecrackerBackend{}).Pause(context.Background(), h); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("Pause with --no-api: %v", err)
	}
	if _, err := (FirecrackerBackend{}).CreateSnapshot(context.Background(), h, "/x"); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("CreateSnapshot with --no-api: %v", err)
	}
}

type apiCall struct {
	Method string
	Path   string
	Body   map[string]any
}

// fakeAPIServer serves canned answers on a unix socket and records calls.
func fakeAPIServer(t *testing.T, handle func(w http.ResponseWriter, call apiCall)) (string, func() []apiCall) {
	t.Helper()
	dir, err := os.MkdirTemp("", "vb")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	sock := filepath.Join(dir, "api.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	var (
		mu    sync.Mutex
		calls []apiCall
	)
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		call := apiCall{Method: r.Method, Path: r.URL.Path}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &call.Body)
		}
		mu.Lock()
		calls = append(calls, call)
		mu.Unlock()
		handle(w, call)
	})}
	go srv.Serve(ln)
	t.Cleanup(func() { srv.Close() })
	return sock, func() []apiCall {
		mu.Lock()
		defer mu.Unlock()
		return append([]apiCall(nil), calls...)
	}
}

func TestFirecrackerSnapshotAPICalls(t *testing.T) {
	sock, calls := fakeAPIServer(t, func(w http.ResponseWriter, call apiCall) {
		w.WriteHeader(http.StatusNoContent)
	})
	h := Handle{Backend: Firecracker, VMID: "vm-a", APISocket: sock}
	fc := FirecrackerBackend{}
	ctx := context.Background()
	if err := fc.Pause(ctx, h); err != nil {
		t.Fatal(err)
	}
	files, err := fc.CreateSnapshot(ctx, h, "/snap")
	if err != nil {
		t.Fatal(err)
	}
	if len(files.Files) != 2 || files.Files[0] != "/snap/vmstate" || files.Files[1] != "/snap/mem" {
		t.Fatalf("snapshot files %+v", files)
	}
	if err := fc.CompleteRestore(ctx, h, "/snap"); err != nil {
		t.Fatal(err)
	}
	got := calls()
	want := []struct{ method, path string }{
		{"PATCH", "/vm"}, {"PUT", "/snapshot/create"}, {"GET", "/"}, {"PUT", "/snapshot/load"},
	}
	if len(got) != len(want) {
		t.Fatalf("calls %+v", got)
	}
	for i, w := range want {
		if got[i].Method != w.method || got[i].Path != w.path {
			t.Fatalf("call %d = %s %s, want %s %s", i, got[i].Method, got[i].Path, w.method, w.path)
		}
	}
	if got[0].Body["state"] != "Paused" {
		t.Fatalf("pause body %+v", got[0].Body)
	}
	if got[1].Body["snapshot_type"] != "Full" || got[1].Body["mem_file_path"] != "/snap/mem" {
		t.Fatalf("create body %+v", got[1].Body)
	}
	backend, _ := got[3].Body["mem_backend"].(map[string]any)
	if got[3].Body["resume_vm"] != true || backend["backend_type"] != "File" || backend["backend_path"] != "/snap/mem" {
		t.Fatalf("load body %+v", got[3].Body)
	}
}

func TestFirecrackerAPIErrorCarriesFaultMessage(t *testing.T) {
	sock, _ := fakeAPIServer(t, func(w http.ResponseWriter, call apiCall) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"fault_message":"not paused"}`))
	})
	err := FirecrackerBackend{}.Pause(context.Background(), Handle{APISocket: sock})
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != 400 || apiErr.Message != "not paused" {
		t.Fatalf("err = %v", err)
	}
}
