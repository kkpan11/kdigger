package syscalls

import (
	"fmt"
	"syscall"
	"time"

	"github.com/mtardy/kdigger/pkg/bucket"
	"golang.org/x/sys/unix"
)

const (
	bucketName        = "syscalls"
	bucketDescription = "Syscalls scans most of the syscalls to detect which are blocked and allowed."
)

var bucketAliases = []string{"syscall", "sys"}

type SyscallsBucket struct{}

func (n SyscallsBucket) Run() (bucket.Results, error) {
	// executes here the code of your plugin
	res := bucket.NewResults(bucketName)

	results := syscallIter()
	var allowed []string
	var blocked []string
	for _, r := range results {
		if r.allowed {
			allowed = append(allowed, r.name)
		} else {
			blocked = append(blocked, r.name)
		}
	}

	res.SetHeaders([]string{"blocked", "allowed"})
	res.AddContent([]interface{}{blocked, allowed})

	skippedNames := make([]string, 0)
	for _, s := range skippedSyscalls {
		skippedNames = append(skippedNames, syscallName(s))
	}
	res.SetComment(fmt.Sprint(skippedNames) + " were not scanned because they cause hang or for obvious reasons.")

	return *res, nil
}

// Register registers a plugin
func Register(b *bucket.Buckets) {
	b.Register(bucketName, bucketAliases, bucketDescription, true, func(config bucket.Config) (bucket.Interface, error) {
		return NewSyscallsBucket(config)
	})
}

func NewSyscallsBucket(config bucket.Config) (*SyscallsBucket, error) {
	return &SyscallsBucket{}, nil
}

type scanResult struct {
	name    string
	allowed bool
}

// syscallIter is modified copy of the amicontained code that you can find here:
// https://github.com/genuinetools/amicontained/blob/568b0d35e60cb2bfc228ecade8b0ba62c49a906a/main.go#L181
func syscallIter() []scanResult {
	var results []scanResult

	c := make(chan scanResult, unix.SYS_RSEQ-len(skippedSyscalls))
	for id := 0; id <= unix.SYS_RSEQ; id++ {
		go scanSyscall(id, c)
	}

	for i := 0; i < cap(c); i++ {
		results = append(results, <-c)
	}

	return results
}

var skippedSyscalls = []int{
	unix.SYS_RT_SIGRETURN,
	unix.SYS_SELECT,
	unix.SYS_PAUSE,
	unix.SYS_PSELECT6,
	unix.SYS_PPOLL,
	unix.SYS_WAITID,
	unix.SYS_EXIT,
	unix.SYS_EXIT_GROUP,
	unix.SYS_CLONE,
	unix.SYS_FORK,
	unix.SYS_VFORK,
	unix.SYS_SECCOMP,
	unix.SYS_PTRACE,
	unix.SYS_VHANGUP,
}

func scanSyscall(id int, c chan (scanResult)) {
	// these cause a hang, so just skip
	// rt_sigreturn, select, pause, pselect6, ppoll
	if id == unix.SYS_RT_SIGRETURN || id == unix.SYS_SELECT || id == unix.SYS_PAUSE || id == unix.SYS_PSELECT6 || id == unix.SYS_PPOLL || id == unix.SYS_WAITID {
		return
	}
	// exit_group and exit -- causes us to exit.. doh!
	if id == unix.SYS_EXIT || id == unix.SYS_EXIT_GROUP {
		return
	}

	// things currently break horribly if  CLONE, FORK or VFORK are called and the call succeeds
	// guess it should be straight forward to kill the forks
	if id == unix.SYS_CLONE || id == unix.SYS_FORK || id == unix.SYS_VFORK {
		return
	}

	// Skip seccomp itself.
	if id == unix.SYS_SECCOMP {
		return
	}

	// ptrace causes a weird race condition that can make the process hang
	if id == unix.SYS_PTRACE {
		return
	}

	// skip vhangup, it exits a container terminal and there is no interest to test it
	if id == unix.SYS_VHANGUP {
		return
	}

	// The call may block, so invoke asynchronously and rely on rejections being fast.
	errs := make(chan error)
	go func() {
		_, _, err := syscall.Syscall(uintptr(id), 0, 0, 0)
		errs <- err
	}()

	var err error
	select {
	case err = <-errs:
	case <-time.After(100 * time.Millisecond):
		// The syscall was allowed, but it didn't return
	}
	// fmt.Println(err.Error())

	if err == syscall.EPERM || err == syscall.EACCES {
		c <- scanResult{name: syscallName(id), allowed: false}
	} else if err != syscall.EOPNOTSUPP {
		c <- scanResult{name: syscallName(id), allowed: true}
	}
}

func syscallName(e int) string {
	switch e {
	case unix.SYS_READ:
		return "READ"
	case unix.SYS_WRITE:
		return "WRITE"
	case unix.SYS_OPEN:
		return "OPEN"
	case unix.SYS_CLOSE:
		return "CLOSE"
	case unix.SYS_STAT:
		return "STAT"
	case unix.SYS_FSTAT:
		return "FSTAT"
	case unix.SYS_LSTAT:
		return "LSTAT"
	case unix.SYS_POLL:
		return "POLL"
	case unix.SYS_LSEEK:
		return "LSEEK"
	case unix.SYS_MMAP:
		return "MMAP"
	case unix.SYS_MPROTECT:
		return "MPROTECT"
	case unix.SYS_MUNMAP:
		return "MUNMAP"
	case unix.SYS_BRK:
		return "BRK"
	case unix.SYS_RT_SIGACTION:
		return "RT_SIGACTION"
	case unix.SYS_RT_SIGPROCMASK:
		return "RT_SIGPROCMASK"
	case unix.SYS_RT_SIGRETURN:
		return "RT_SIGRETURN"
	case unix.SYS_IOCTL:
		return "IOCTL"
	case unix.SYS_PREAD64:
		return "PREAD64"
	case unix.SYS_PWRITE64:
		return "PWRITE64"
	case unix.SYS_READV:
		return "READV"
	case unix.SYS_WRITEV:
		return "WRITEV"
	case unix.SYS_ACCESS:
		return "ACCESS"
	case unix.SYS_PIPE:
		return "PIPE"
	case unix.SYS_SELECT:
		return "SELECT"
	case unix.SYS_SCHED_YIELD:
		return "SCHED_YIELD"
	case unix.SYS_MREMAP:
		return "MREMAP"
	case unix.SYS_MSYNC:
		return "MSYNC"
	case unix.SYS_MINCORE:
		return "MINCORE"
	case unix.SYS_MADVISE:
		return "MADVISE"
	case unix.SYS_SHMGET:
		return "SHMGET"
	case unix.SYS_SHMAT:
		return "SHMAT"
	case unix.SYS_SHMCTL:
		return "SHMCTL"
	case unix.SYS_DUP:
		return "DUP"
	case unix.SYS_DUP2:
		return "DUP2"
	case unix.SYS_PAUSE:
		return "PAUSE"
	case unix.SYS_NANOSLEEP:
		return "NANOSLEEP"
	case unix.SYS_GETITIMER:
		return "GETITIMER"
	case unix.SYS_ALARM:
		return "ALARM"
	case unix.SYS_SETITIMER:
		return "SETITIMER"
	case unix.SYS_GETPID:
		return "GETPID"
	case unix.SYS_SENDFILE:
		return "SENDFILE"
	case unix.SYS_SOCKET:
		return "SOCKET"
	case unix.SYS_CONNECT:
		return "CONNECT"
	case unix.SYS_ACCEPT:
		return "ACCEPT"
	case unix.SYS_SENDTO:
		return "SENDTO"
	case unix.SYS_RECVFROM:
		return "RECVFROM"
	case unix.SYS_SENDMSG:
		return "SENDMSG"
	case unix.SYS_RECVMSG:
		return "RECVMSG"
	case unix.SYS_SHUTDOWN:
		return "SHUTDOWN"
	case unix.SYS_BIND:
		return "BIND"
	case unix.SYS_LISTEN:
		return "LISTEN"
	case unix.SYS_GETSOCKNAME:
		return "GETSOCKNAME"
	case unix.SYS_GETPEERNAME:
		return "GETPEERNAME"
	case unix.SYS_SOCKETPAIR:
		return "SOCKETPAIR"
	case unix.SYS_SETSOCKOPT:
		return "SETSOCKOPT"
	case unix.SYS_GETSOCKOPT:
		return "GETSOCKOPT"
	case unix.SYS_CLONE:
		return "CLONE"
	case unix.SYS_FORK:
		return "FORK"
	case unix.SYS_VFORK:
		return "VFORK"
	case unix.SYS_EXECVE:
		return "EXECVE"
	case unix.SYS_EXIT:
		return "EXIT"
	case unix.SYS_WAIT4:
		return "WAIT4"
	case unix.SYS_KILL:
		return "KILL"
	case unix.SYS_UNAME:
		return "UNAME"
	case unix.SYS_SEMGET:
		return "SEMGET"
	case unix.SYS_SEMOP:
		return "SEMOP"
	case unix.SYS_SEMCTL:
		return "SEMCTL"
	case unix.SYS_SHMDT:
		return "SHMDT"
	case unix.SYS_MSGGET:
		return "MSGGET"
	case unix.SYS_MSGSND:
		return "MSGSND"
	case unix.SYS_MSGRCV:
		return "MSGRCV"
	case unix.SYS_MSGCTL:
		return "MSGCTL"
	case unix.SYS_FCNTL:
		return "FCNTL"
	case unix.SYS_FLOCK:
		return "FLOCK"
	case unix.SYS_FSYNC:
		return "FSYNC"
	case unix.SYS_FDATASYNC:
		return "FDATASYNC"
	case unix.SYS_TRUNCATE:
		return "TRUNCATE"
	case unix.SYS_FTRUNCATE:
		return "FTRUNCATE"
	case unix.SYS_GETDENTS:
		return "GETDENTS"
	case unix.SYS_GETCWD:
		return "GETCWD"
	case unix.SYS_CHDIR:
		return "CHDIR"
	case unix.SYS_FCHDIR:
		return "FCHDIR"
	case unix.SYS_RENAME:
		return "RENAME"
	case unix.SYS_MKDIR:
		return "MKDIR"
	case unix.SYS_RMDIR:
		return "RMDIR"
	case unix.SYS_CREAT:
		return "CREAT"
	case unix.SYS_LINK:
		return "LINK"
	case unix.SYS_UNLINK:
		return "UNLINK"
	case unix.SYS_SYMLINK:
		return "SYMLINK"
	case unix.SYS_READLINK:
		return "READLINK"
	case unix.SYS_CHMOD:
		return "CHMOD"
	case unix.SYS_FCHMOD:
		return "FCHMOD"
	case unix.SYS_CHOWN:
		return "CHOWN"
	case unix.SYS_FCHOWN:
		return "FCHOWN"
	case unix.SYS_LCHOWN:
		return "LCHOWN"
	case unix.SYS_UMASK:
		return "UMASK"
	case unix.SYS_GETTIMEOFDAY:
		return "GETTIMEOFDAY"
	case unix.SYS_GETRLIMIT:
		return "GETRLIMIT"
	case unix.SYS_GETRUSAGE:
		return "GETRUSAGE"
	case unix.SYS_SYSINFO:
		return "SYSINFO"
	case unix.SYS_TIMES:
		return "TIMES"
	case unix.SYS_PTRACE:
		return "PTRACE"
	case unix.SYS_GETUID:
		return "GETUID"
	case unix.SYS_SYSLOG:
		return "SYSLOG"
	case unix.SYS_GETGID:
		return "GETGID"
	case unix.SYS_SETUID:
		return "SETUID"
	case unix.SYS_SETGID:
		return "SETGID"
	case unix.SYS_GETEUID:
		return "GETEUID"
	case unix.SYS_GETEGID:
		return "GETEGID"
	case unix.SYS_SETPGID:
		return "SETPGID"
	case unix.SYS_GETPPID:
		return "GETPPID"
	case unix.SYS_GETPGRP:
		return "GETPGRP"
	case unix.SYS_SETSID:
		return "SETSID"
	case unix.SYS_SETREUID:
		return "SETREUID"
	case unix.SYS_SETREGID:
		return "SETREGID"
	case unix.SYS_GETGROUPS:
		return "GETGROUPS"
	case unix.SYS_SETGROUPS:
		return "SETGROUPS"
	case unix.SYS_SETRESUID:
		return "SETRESUID"
	case unix.SYS_GETRESUID:
		return "GETRESUID"
	case unix.SYS_SETRESGID:
		return "SETRESGID"
	case unix.SYS_GETRESGID:
		return "GETRESGID"
	case unix.SYS_GETPGID:
		return "GETPGID"
	case unix.SYS_SETFSUID:
		return "SETFSUID"
	case unix.SYS_SETFSGID:
		return "SETFSGID"
	case unix.SYS_GETSID:
		return "GETSID"
	case unix.SYS_CAPGET:
		return "CAPGET"
	case unix.SYS_CAPSET:
		return "CAPSET"
	case unix.SYS_RT_SIGPENDING:
		return "RT_SIGPENDING"
	case unix.SYS_RT_SIGTIMEDWAIT:
		return "RT_SIGTIMEDWAIT"
	case unix.SYS_RT_SIGQUEUEINFO:
		return "RT_SIGQUEUEINFO"
	case unix.SYS_RT_SIGSUSPEND:
		return "RT_SIGSUSPEND"
	case unix.SYS_SIGALTSTACK:
		return "SIGALTSTACK"
	case unix.SYS_UTIME:
		return "UTIME"
	case unix.SYS_MKNOD:
		return "MKNOD"
	case unix.SYS_USELIB:
		return "USELIB"
	case unix.SYS_PERSONALITY:
		return "PERSONALITY"
	case unix.SYS_USTAT:
		return "USTAT"
	case unix.SYS_STATFS:
		return "STATFS"
	case unix.SYS_FSTATFS:
		return "FSTATFS"
	case unix.SYS_SYSFS:
		return "SYSFS"
	case unix.SYS_GETPRIORITY:
		return "GETPRIORITY"
	case unix.SYS_SETPRIORITY:
		return "SETPRIORITY"
	case unix.SYS_SCHED_SETPARAM:
		return "SCHED_SETPARAM"
	case unix.SYS_SCHED_GETPARAM:
		return "SCHED_GETPARAM"
	case unix.SYS_SCHED_SETSCHEDULER:
		return "SCHED_SETSCHEDULER"
	case unix.SYS_SCHED_GETSCHEDULER:
		return "SCHED_GETSCHEDULER"
	case unix.SYS_SCHED_GET_PRIORITY_MAX:
		return "SCHED_GET_PRIORITY_MAX"
	case unix.SYS_SCHED_GET_PRIORITY_MIN:
		return "SCHED_GET_PRIORITY_MIN"
	case unix.SYS_SCHED_RR_GET_INTERVAL:
		return "SCHED_RR_GET_INTERVAL"
	case unix.SYS_MLOCK:
		return "MLOCK"
	case unix.SYS_MUNLOCK:
		return "MUNLOCK"
	case unix.SYS_MLOCKALL:
		return "MLOCKALL"
	case unix.SYS_MUNLOCKALL:
		return "MUNLOCKALL"
	case unix.SYS_VHANGUP:
		return "VHANGUP"
	case unix.SYS_MODIFY_LDT:
		return "MODIFY_LDT"
	case unix.SYS_PIVOT_ROOT:
		return "PIVOT_ROOT"
	case unix.SYS__SYSCTL:
		return "_SYSCTL"
	case unix.SYS_PRCTL:
		return "PRCTL"
	case unix.SYS_ARCH_PRCTL:
		return "ARCH_PRCTL"
	case unix.SYS_ADJTIMEX:
		return "ADJTIMEX"
	case unix.SYS_SETRLIMIT:
		return "SETRLIMIT"
	case unix.SYS_CHROOT:
		return "CHROOT"
	case unix.SYS_SYNC:
		return "SYNC"
	case unix.SYS_ACCT:
		return "ACCT"
	case unix.SYS_SETTIMEOFDAY:
		return "SETTIMEOFDAY"
	case unix.SYS_MOUNT:
		return "MOUNT"
	case unix.SYS_UMOUNT2:
		return "UMOUNT2"
	case unix.SYS_SWAPON:
		return "SWAPON"
	case unix.SYS_SWAPOFF:
		return "SWAPOFF"
	case unix.SYS_REBOOT:
		return "REBOOT"
	case unix.SYS_SETHOSTNAME:
		return "SETHOSTNAME"
	case unix.SYS_SETDOMAINNAME:
		return "SETDOMAINNAME"
	case unix.SYS_IOPL:
		return "IOPL"
	case unix.SYS_IOPERM:
		return "IOPERM"
	case unix.SYS_CREATE_MODULE:
		return "CREATE_MODULE"
	case unix.SYS_INIT_MODULE:
		return "INIT_MODULE"
	case unix.SYS_DELETE_MODULE:
		return "DELETE_MODULE"
	case unix.SYS_GET_KERNEL_SYMS:
		return "GET_KERNEL_SYMS"
	case unix.SYS_QUERY_MODULE:
		return "QUERY_MODULE"
	case unix.SYS_QUOTACTL:
		return "QUOTACTL"
	case unix.SYS_NFSSERVCTL:
		return "NFSSERVCTL"
	case unix.SYS_GETPMSG:
		return "GETPMSG"
	case unix.SYS_PUTPMSG:
		return "PUTPMSG"
	case unix.SYS_AFS_SYSCALL:
		return "AFS_SYSCALL"
	case unix.SYS_TUXCALL:
		return "TUXCALL"
	case unix.SYS_SECURITY:
		return "SECURITY"
	case unix.SYS_GETTID:
		return "GETTID"
	case unix.SYS_READAHEAD:
		return "READAHEAD"
	case unix.SYS_SETXATTR:
		return "SETXATTR"
	case unix.SYS_LSETXATTR:
		return "LSETXATTR"
	case unix.SYS_FSETXATTR:
		return "FSETXATTR"
	case unix.SYS_GETXATTR:
		return "GETXATTR"
	case unix.SYS_LGETXATTR:
		return "LGETXATTR"
	case unix.SYS_FGETXATTR:
		return "FGETXATTR"
	case unix.SYS_LISTXATTR:
		return "LISTXATTR"
	case unix.SYS_LLISTXATTR:
		return "LLISTXATTR"
	case unix.SYS_FLISTXATTR:
		return "FLISTXATTR"
	case unix.SYS_REMOVEXATTR:
		return "REMOVEXATTR"
	case unix.SYS_LREMOVEXATTR:
		return "LREMOVEXATTR"
	case unix.SYS_FREMOVEXATTR:
		return "FREMOVEXATTR"
	case unix.SYS_TKILL:
		return "TKILL"
	case unix.SYS_TIME:
		return "TIME"
	case unix.SYS_FUTEX:
		return "FUTEX"
	case unix.SYS_SCHED_SETAFFINITY:
		return "SCHED_SETAFFINITY"
	case unix.SYS_SCHED_GETAFFINITY:
		return "SCHED_GETAFFINITY"
	case unix.SYS_SET_THREAD_AREA:
		return "SET_THREAD_AREA"
	case unix.SYS_IO_SETUP:
		return "IO_SETUP"
	case unix.SYS_IO_DESTROY:
		return "IO_DESTROY"
	case unix.SYS_IO_GETEVENTS:
		return "IO_GETEVENTS"
	case unix.SYS_IO_SUBMIT:
		return "IO_SUBMIT"
	case unix.SYS_IO_CANCEL:
		return "IO_CANCEL"
	case unix.SYS_GET_THREAD_AREA:
		return "GET_THREAD_AREA"
	case unix.SYS_LOOKUP_DCOOKIE:
		return "LOOKUP_DCOOKIE"
	case unix.SYS_EPOLL_CREATE:
		return "EPOLL_CREATE"
	case unix.SYS_EPOLL_CTL_OLD:
		return "EPOLL_CTL_OLD"
	case unix.SYS_EPOLL_WAIT_OLD:
		return "EPOLL_WAIT_OLD"
	case unix.SYS_REMAP_FILE_PAGES:
		return "REMAP_FILE_PAGES"
	case unix.SYS_GETDENTS64:
		return "GETDENTS64"
	case unix.SYS_SET_TID_ADDRESS:
		return "SET_TID_ADDRESS"
	case unix.SYS_RESTART_SYSCALL:
		return "RESTART_SYSCALL"
	case unix.SYS_SEMTIMEDOP:
		return "SEMTIMEDOP"
	case unix.SYS_FADVISE64:
		return "FADVISE64"
	case unix.SYS_TIMER_CREATE:
		return "TIMER_CREATE"
	case unix.SYS_TIMER_SETTIME:
		return "TIMER_SETTIME"
	case unix.SYS_TIMER_GETTIME:
		return "TIMER_GETTIME"
	case unix.SYS_TIMER_GETOVERRUN:
		return "TIMER_GETOVERRUN"
	case unix.SYS_TIMER_DELETE:
		return "TIMER_DELETE"
	case unix.SYS_CLOCK_SETTIME:
		return "CLOCK_SETTIME"
	case unix.SYS_CLOCK_GETTIME:
		return "CLOCK_GETTIME"
	case unix.SYS_CLOCK_GETRES:
		return "CLOCK_GETRES"
	case unix.SYS_CLOCK_NANOSLEEP:
		return "CLOCK_NANOSLEEP"
	case unix.SYS_EXIT_GROUP:
		return "EXIT_GROUP"
	case unix.SYS_EPOLL_WAIT:
		return "EPOLL_WAIT"
	case unix.SYS_EPOLL_CTL:
		return "EPOLL_CTL"
	case unix.SYS_TGKILL:
		return "TGKILL"
	case unix.SYS_UTIMES:
		return "UTIMES"
	case unix.SYS_VSERVER:
		return "VSERVER"
	case unix.SYS_MBIND:
		return "MBIND"
	case unix.SYS_SET_MEMPOLICY:
		return "SET_MEMPOLICY"
	case unix.SYS_GET_MEMPOLICY:
		return "GET_MEMPOLICY"
	case unix.SYS_MQ_OPEN:
		return "MQ_OPEN"
	case unix.SYS_MQ_UNLINK:
		return "MQ_UNLINK"
	case unix.SYS_MQ_TIMEDSEND:
		return "MQ_TIMEDSEND"
	case unix.SYS_MQ_TIMEDRECEIVE:
		return "MQ_TIMEDRECEIVE"
	case unix.SYS_MQ_NOTIFY:
		return "MQ_NOTIFY"
	case unix.SYS_MQ_GETSETATTR:
		return "MQ_GETSETATTR"
	case unix.SYS_KEXEC_LOAD:
		return "KEXEC_LOAD"
	case unix.SYS_WAITID:
		return "WAITID"
	case unix.SYS_ADD_KEY:
		return "ADD_KEY"
	case unix.SYS_REQUEST_KEY:
		return "REQUEST_KEY"
	case unix.SYS_KEYCTL:
		return "KEYCTL"
	case unix.SYS_IOPRIO_SET:
		return "IOPRIO_SET"
	case unix.SYS_IOPRIO_GET:
		return "IOPRIO_GET"
	case unix.SYS_INOTIFY_INIT:
		return "INOTIFY_INIT"
	case unix.SYS_INOTIFY_ADD_WATCH:
		return "INOTIFY_ADD_WATCH"
	case unix.SYS_INOTIFY_RM_WATCH:
		return "INOTIFY_RM_WATCH"
	case unix.SYS_MIGRATE_PAGES:
		return "MIGRATE_PAGES"
	case unix.SYS_OPENAT:
		return "OPENAT"
	case unix.SYS_MKDIRAT:
		return "MKDIRAT"
	case unix.SYS_MKNODAT:
		return "MKNODAT"
	case unix.SYS_FCHOWNAT:
		return "FCHOWNAT"
	case unix.SYS_FUTIMESAT:
		return "FUTIMESAT"
	case unix.SYS_NEWFSTATAT:
		return "NEWFSTATAT"
	case unix.SYS_UNLINKAT:
		return "UNLINKAT"
	case unix.SYS_RENAMEAT:
		return "RENAMEAT"
	case unix.SYS_LINKAT:
		return "LINKAT"
	case unix.SYS_SYMLINKAT:
		return "SYMLINKAT"
	case unix.SYS_READLINKAT:
		return "READLINKAT"
	case unix.SYS_FCHMODAT:
		return "FCHMODAT"
	case unix.SYS_FACCESSAT:
		return "FACCESSAT"
	case unix.SYS_PSELECT6:
		return "PSELECT6"
	case unix.SYS_PPOLL:
		return "PPOLL"
	case unix.SYS_UNSHARE:
		return "UNSHARE"
	case unix.SYS_SET_ROBUST_LIST:
		return "SET_ROBUST_LIST"
	case unix.SYS_GET_ROBUST_LIST:
		return "GET_ROBUST_LIST"
	case unix.SYS_SPLICE:
		return "SPLICE"
	case unix.SYS_TEE:
		return "TEE"
	case unix.SYS_SYNC_FILE_RANGE:
		return "SYNC_FILE_RANGE"
	case unix.SYS_VMSPLICE:
		return "VMSPLICE"
	case unix.SYS_MOVE_PAGES:
		return "MOVE_PAGES"
	case unix.SYS_UTIMENSAT:
		return "UTIMENSAT"
	case unix.SYS_EPOLL_PWAIT:
		return "EPOLL_PWAIT"
	case unix.SYS_SIGNALFD:
		return "SIGNALFD"
	case unix.SYS_TIMERFD_CREATE:
		return "TIMERFD_CREATE"
	case unix.SYS_EVENTFD:
		return "EVENTFD"
	case unix.SYS_FALLOCATE:
		return "FALLOCATE"
	case unix.SYS_TIMERFD_SETTIME:
		return "TIMERFD_SETTIME"
	case unix.SYS_TIMERFD_GETTIME:
		return "TIMERFD_GETTIME"
	case unix.SYS_ACCEPT4:
		return "ACCEPT4"
	case unix.SYS_SIGNALFD4:
		return "SIGNALFD4"
	case unix.SYS_EVENTFD2:
		return "EVENTFD2"
	case unix.SYS_EPOLL_CREATE1:
		return "EPOLL_CREATE1"
	case unix.SYS_DUP3:
		return "DUP3"
	case unix.SYS_PIPE2:
		return "PIPE2"
	case unix.SYS_INOTIFY_INIT1:
		return "INOTIFY_INIT1"
	case unix.SYS_PREADV:
		return "PREADV"
	case unix.SYS_PWRITEV:
		return "PWRITEV"
	case unix.SYS_RT_TGSIGQUEUEINFO:
		return "RT_TGSIGQUEUEINFO"
	case unix.SYS_PERF_EVENT_OPEN:
		return "PERF_EVENT_OPEN"
	case unix.SYS_RECVMMSG:
		return "RECVMMSG"
	case unix.SYS_FANOTIFY_INIT:
		return "FANOTIFY_INIT"
	case unix.SYS_FANOTIFY_MARK:
		return "FANOTIFY_MARK"
	case unix.SYS_PRLIMIT64:
		return "PRLIMIT64"
	case unix.SYS_NAME_TO_HANDLE_AT:
		return "NAME_TO_HANDLE_AT"
	case unix.SYS_OPEN_BY_HANDLE_AT:
		return "OPEN_BY_HANDLE_AT"
	case unix.SYS_CLOCK_ADJTIME:
		return "CLOCK_ADJTIME"
	case unix.SYS_SYNCFS:
		return "SYNCFS"
	case unix.SYS_SENDMMSG:
		return "SENDMMSG"
	case unix.SYS_SETNS:
		return "SETNS"
	case unix.SYS_GETCPU:
		return "GETCPU"
	case unix.SYS_PROCESS_VM_READV:
		return "PROCESS_VM_READV"
	case unix.SYS_PROCESS_VM_WRITEV:
		return "PROCESS_VM_WRITEV"
	case unix.SYS_KCMP:
		return "KCMP"
	case unix.SYS_FINIT_MODULE:
		return "FINIT_MODULE"
	case unix.SYS_SCHED_SETATTR:
		return "SCHED_SETATTR"
	case unix.SYS_SCHED_GETATTR:
		return "SCHED_GETATTR"
	case unix.SYS_RENAMEAT2:
		return "RENAMEAT2"
	case unix.SYS_SECCOMP:
		return "SECCOMP"
	case unix.SYS_GETRANDOM:
		return "GETRANDOM"
	case unix.SYS_MEMFD_CREATE:
		return "MEMFD_CREATE"
	case unix.SYS_KEXEC_FILE_LOAD:
		return "KEXEC_FILE_LOAD"
	case unix.SYS_BPF:
		return "BPF"
	case unix.SYS_EXECVEAT:
		return "EXECVEAT"
	case unix.SYS_USERFAULTFD:
		return "USERFAULTFD"
	case unix.SYS_MEMBARRIER:
		return "MEMBARRIER"
	case unix.SYS_MLOCK2:
		return "MLOCK2"
	case unix.SYS_COPY_FILE_RANGE:
		return "COPY_FILE_RANGE"
	case unix.SYS_PREADV2:
		return "PREADV2"
	case unix.SYS_PWRITEV2:
		return "PWRITEV2"
	case unix.SYS_PKEY_MPROTECT:
		return "PKEY_MPROTECT"
	case unix.SYS_PKEY_ALLOC:
		return "PKEY_ALLOC"
	case unix.SYS_PKEY_FREE:
		return "PKEY_FREE"
	case unix.SYS_STATX:
		return "STATX"
	case unix.SYS_IO_PGETEVENTS:
		return "IO_PGETEVENTS"
	case unix.SYS_RSEQ:
		return "RSEQ"
	}
	return fmt.Sprintf("%d - ERR_UNKNOWN_SYSCALL", e)
}
