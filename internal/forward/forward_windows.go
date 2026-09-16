//go:build windows

package forward

import (
	"os"
	"os/exec"
	"syscall"
	"unsafe"
)

var (
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	procCreateJobObject    = kernel32.NewProc("CreateJobObjectW")
	procSetInformationJob  = kernel32.NewProc("SetInformationJobObject")
	procAssignProcessToJob = kernel32.NewProc("AssignProcessToJobObject")
	procGetConsoleWindow   = kernel32.NewProc("GetConsoleWindow")
	procOpenProcess        = kernel32.NewProc("OpenProcess")
	procCloseHandle        = kernel32.NewProc("CloseHandle")
)

const (
	jobObjectExtendedLimitInformation = 9
	jobObjectLimitKillOnJobClose      = 0x00002000
	createNoWindow                    = 0x08000000
	processSetQuota                   = 0x0100
	processTerminate                  = 0x0001
)

// job is the kill-on-close job holding the child; kept open until the child
// exits so that killing the shim also terminates the CLI process tree.
var job syscall.Handle

func prepareChild(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	// When the shim has no console (spawned by a daemon or service), start
	// the child without one too: avoids a console flash and matches the
	// semantics of a hidden daemon spawn.
	if h, _, _ := procGetConsoleWindow.Call(); h == 0 {
		cmd.SysProcAttr.CreationFlags |= createNoWindow
	}
}

// afterStart puts the child into a KILL_ON_JOB_CLOSE job object. If the shim
// is force-killed (task cancel / timeout), the OS closes the handle and the
// whole process tree goes with it. Assignment happens right after start;
// Node.js takes over a second to boot, so the pre-assignment race window is
// smaller than the child's own startup time. Failures are deliberately
// ignored: a job-less run still works, it just loses the cleanup guarantee.
func afterStart(p *os.Process) {
	if p == nil {
		return
	}
	obj, _, _ := procCreateJobObject.Call(0, 0, 0)
	if obj == 0 {
		return
	}
	var info jobExtendedLimitInformation
	info.BasicLimitInformation.LimitFlags = jobObjectLimitKillOnJobClose
	r, _, _ := procSetInformationJob.Call(obj, jobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info))
	if r == 0 {
		closeHandle(syscall.Handle(obj))
		return
	}
	ph, _, _ := procOpenProcess.Call(processSetQuota|processTerminate, 0, uintptr(p.Pid))
	if ph == 0 {
		closeHandle(syscall.Handle(obj))
		return
	}
	r, _, _ = procAssignProcessToJob.Call(obj, ph)
	closeHandle(syscall.Handle(ph))
	if r == 0 {
		closeHandle(syscall.Handle(obj))
		return
	}
	job = syscall.Handle(obj)
}

func cleanupAfterWait() {
	if job != 0 {
		closeHandle(job)
		job = 0
	}
}

func closeHandle(h syscall.Handle) {
	procCloseHandle.Call(uintptr(h))
}

type jobBasicLimitInformation struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

type ioCounters struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

type jobExtendedLimitInformation struct {
	BasicLimitInformation jobBasicLimitInformation
	IoInfo                ioCounters
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}
