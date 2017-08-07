package freki

import (
	"fmt"
	"net"
	"runtime/debug"

	"github.com/pkg/errors"
	"time"
	"github.com/mushorg/go-dpi/types"
	"strconv"
)

type UserConnServer struct {
	port      uint
	processor *Processor
	listener  net.Listener
}

func NewUserConnServer(port uint) *UserConnServer {
	return &UserConnServer{
		port: port,
	}
}

func (h *UserConnServer) Port() uint {
	return h.port
}

func (h *UserConnServer) Type() string {
	return "user.tcp"
}

func (h *UserConnServer) Start(processor *Processor) error {
	h.processor = processor

	var err error
	// TODO: can I be more specific with the bind addr?
	h.listener, err = net.Listen("tcp", fmt.Sprintf(":%d", h.port))

	if err != nil {
		return err
	}

	for {
		conn, err := h.listener.Accept()
		if err != nil {
			logger.Errorf("[user.tcp] %v", err)
			continue
		}

		ck := NewConnKeyFromNetConn(conn)
		md := h.processor.Connections.GetByFlow(ck)

		if md == nil {
			logger.Warnf("[user.tcp] untracked connection: %s", conn.RemoteAddr().String())
			conn.Close()
			continue
		}

		// TODO: there is no connection between freki and the handler
		// once freki starts to shutdown, handlers are not notified.
		// maybe use a Context?
		time.Sleep(3 * time.Second)
		if md.Flow.DetectedProtocol == types.Unknown {
			logger.Debugf("[godpi   ] We have %d packets", len(md.Flow.Packets))
			conn.Write([]byte("220 HELLO WORLDDD\r\n"))
			time.Sleep(6 * time.Second)
			logger.Debugf("[godpi   ] We have %d packets", len(md.Flow.Packets))
			logger.Debugf("[godpi   ] new detection %v", md.Flow.DetectedProtocol)
		}
		logger.Debugf("[godpi   ] Target %v Detected %v", md.Rule.Target, md.Flow.DetectedProtocol)
		godpiMap := map[string]string {"SSH": "proxy_ssh", "HTTP": "default"}
		portsMap := map[string]string {"SSH": "22", "HTTP": "80"}
		logger.Infof("[godpi   ] DETECTED %v by %v on port %v!", md.Flow.DetectedProtocol, md.Flow.ClassificationSource, md.TargetPort)
		nextProto, _ := godpiMap[string(md.Flow.DetectedProtocol)]
		logger.Debug(h.processor.connHandlers)
		if hfunc, ok := h.processor.connHandlers[nextProto]; ok {
			if normalPort, ok := portsMap[string(md.Flow.DetectedProtocol)]; ok && normalPort == strconv.Itoa(int(md.TargetPort)) {
				go func() {
					defer func() {
						if r := recover(); r != nil {
							logger.Errorf("[user.tcp] panic: %+v", r)
							logger.Errorf("[user.tcp] stacktrace:\n%v", string(debug.Stack()))
							conn.Close()
						}
					}()
					err := hfunc(conn, md)
					if err != nil {
						logger.Error(errors.Wrap(err, h.Type()))
					}
				}()
			} else {
				logger.Errorf("Connection to non-standard port! %v in port %v", md.Flow.DetectedProtocol, md.TargetPort)
				conn.Close()
			}
		} else {
			logger.Errorf("[user.tcp] %v", fmt.Errorf("no handler found for %s", md.Rule.Target))
			conn.Close()
			continue
		}
	}
}

func (h *UserConnServer) Shutdown() error {
	if h.listener != nil {
		return h.listener.Close()
	}
	return nil
}
