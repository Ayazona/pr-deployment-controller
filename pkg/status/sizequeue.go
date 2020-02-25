package status

import (
	"k8s.io/client-go/tools/remotecommand"
)

// sizeQueue is used to notify the terminal about size changes
type sizeQueue struct {
	resizeChan chan remotecommand.TerminalSize
}

func newSizeQueue() *sizeQueue {
	return &sizeQueue{
		resizeChan: make(chan remotecommand.TerminalSize, 1),
	}
}

func (s *sizeQueue) Close() {
	close(s.resizeChan)
}

func (s *sizeQueue) Resize(newSize remotecommand.TerminalSize) {
	s.resizeChan <- newSize
}

func (s *sizeQueue) Next() *remotecommand.TerminalSize {
	size, ok := <-s.resizeChan
	if !ok {
		return nil
	}
	return &size
}
