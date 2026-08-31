package terminal

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/gorilla/websocket"
)

const (
	// wsMsgBuffer is the size of the WebSocket message buffer.
	wsMsgBuffer = 4096
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  wsMsgBuffer,
	WriteBufferSize: wsMsgBuffer,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// TerminalMessage is the JSON control message format for non-binary messages.
type TerminalMessage struct {
	Type string `json:"type"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
	Data string `json:"data,omitempty"`
}

// WinSize mirrors the kernel winsize structure for TIOCSWINSZ.
type WinSize struct {
	Rows   uint16
	Cols   uint16
	Xpixel uint16
	Ypixel uint16
}

// Handler serves the WebSocket terminal endpoint.
type Handler struct {
	// Shell is the shell binary to execute (defaults to /bin/bash or /bin/sh).
	Shell string
}

// NewHandler returns a terminal handler using the given shell.
func NewHandler(shell string) *Handler {
	if shell == "" {
		shell = "/bin/bash"
	}
	return &Handler{Shell: shell}
}

// ServeHTTP upgrades the connection and proxies a PTY session.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[terminal] upgrade failed: %v", err)
		return
	}

	shell := h.Shell
	if _, err := exec.LookPath(shell); err != nil {
		shell = "/bin/sh"
	}

	cmd, master, err := startPTY(shell)
	if err != nil {
		log.Printf("[terminal] openPTY failed: %v", err)
		conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "failed to allocate pty"),
			time.Now().Add(time.Second))
		conn.Close()
		return
	}
	defer master.Close()
	defer conn.Close()

	setWinSize(master, 24, 80)

	var wg sync.WaitGroup
	done := make(chan struct{})
	var closeOnce sync.Once
	signalDone := func() { closeOnce.Do(func() { close(done) }) }

	// PTY -> WebSocket
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer signalDone()
		buf := make([]byte, 4096)
		for {
			n, err := master.Read(buf)
			if n > 0 {
				if werr := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); werr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// WebSocket -> PTY
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer signalDone()
		for {
			msgType, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			switch msgType {
			case websocket.TextMessage:
				var msg TerminalMessage
				if err := json.Unmarshal(data, &msg); err != nil {
					continue
				}
				switch msg.Type {
				case "resize":
					if msg.Cols > 0 && msg.Rows > 0 {
						setWinSize(master, uint16(msg.Rows), uint16(msg.Cols))
					}
				case "input":
					if msg.Data != "" {
						master.Write([]byte(msg.Data))
					}
				}
			case websocket.BinaryMessage:
				master.Write(data)
			}
		}
	}()

	// Wait until either the client disconnects or the shell exits.
	<-done

	// Clean up the process.
	cmd.Process.Signal(syscall.SIGINT)
	select {
	case <-time.After(time.Second):
		cmd.Process.Kill()
	default:
	}
	// Close both ends to unblock the I/O goroutines.
	master.Close()
	conn.Close()
	wg.Wait()
	cmd.Wait()
}

// startPTY allocates a pty, starts the given shell inside it, and returns the
// running command and the master *os.File. It first tries to grant a
// controlling terminal (for full job control); if the environment forbids it,
// it transparently falls back to a session without a controlling tty.
func startPTY(shell string) (*exec.Cmd, *os.File, error) {
	cmd, master, err := buildAndStart(shell, true)
	if err == nil {
		return cmd, master, nil
	}
	// Fall back: rebuild with a fresh command and no controlling tty.
	return buildAndStart(shell, false)
}

// buildAndStart creates a shell command wired to a fresh pty and starts it.
func buildAndStart(shell string, ctty bool) (*exec.Cmd, *os.File, error) {
	cmd := exec.Command(shell)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor")

	master, slave, err := openPTY()
	if err != nil {
		return nil, nil, err
	}
	cmd.Stdin = slave
	cmd.Stdout = slave
	cmd.Stderr = slave
	if ctty {
		// The slave is wired to stdin, so fd 0 is the controlling tty.
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	} else {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	}
	if err := cmd.Start(); err != nil {
		master.Close()
		slave.Close()
		return nil, nil, err
	}
	return cmd, master, nil
}

// openPTY allocates a pty pair and returns the master and slave *os.File.
func openPTY() (*os.File, *os.File, error) {
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		return nil, nil, err
	}

	var unlock int32
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, master.Fd(), syscall.TIOCSPTLCK, uintptr(unsafe.Pointer(&unlock))); errno != 0 {
		master.Close()
		return nil, nil, errno
	}

	var n int
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, master.Fd(), syscall.TIOCGPTN, uintptr(unsafe.Pointer(&n))); errno != 0 {
		master.Close()
		return nil, nil, errno
	}

	slaveName := "/dev/pts/" + itoa(n)
	slave, err := os.OpenFile(slaveName, os.O_RDWR, 0)
	if err != nil {
		master.Close()
		return nil, nil, err
	}
	return master, slave, nil
}

// setWinSize applies a window size via TIOCSWINSZ.
func setWinSize(f *os.File, rows, cols uint16) {
	if f == nil {
		return
	}
	ws := &WinSize{Rows: rows, Cols: cols}
	syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(syscall.TIOCSWINSZ), uintptr(unsafe.Pointer(ws)))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
