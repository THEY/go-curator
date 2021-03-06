package curator

import (
	"github.com/cenkalti/backoff"
	"github.com/davecgh/go-spew/spew"
	"github.com/talbright/go-zookeeper/zk"

	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

const MaxWaitToExistTime = 2 * time.Minute

//NullLogger can be used to silence output from the client connection. Only
//recommended for tests.
type NullLogger struct{}

//Printf is the only method that is part of the connection logger interface
func (NullLogger) Printf(format string, a ...interface{}) {}

//ErrConnectionTimedOut occurs when initial connection attempt to zk fails
var ErrConnectionTimedOut = errors.New("connection to zookeeper timed out")

//ErrInvalidPath occurs when the provided path in zk is malformed
var ErrInvalidPath = errors.New("provided path is invalid")

//Client connects to and interacts with zk.
type Client struct {
	*zk.Conn
	mutex *sync.RWMutex
}

//NewClient creates a client that can interact with zk
func NewClient() *Client {
	return &Client{mutex: &sync.RWMutex{}}
}

//Connect creates a connection to zookeeper for the client
func (c *Client) Connect(settings *Settings, options ...zk.ConnOption) (evnt <-chan zk.Event, err error) {
	c.Conn, evnt, err = zk.Connect(settings.ZkServers, settings.ZkSessionTimeout, options...)
	if settings.ZkWaitForSession && err == nil {
		timeout := make(chan bool, 1)
		if settings.ZkWaitForSessionTimeout > 0 {
			go func() {
				time.Sleep(settings.ZkWaitForSessionTimeout)
				timeout <- true
			}()
		}
		for {
			select {
			case <-timeout:
				c.Conn.Close()
				err = ErrConnectionTimedOut
				return
			case event := <-evnt:
				if event.Type == zk.EventSession {
					switch event.State {
					case zk.StateHasSession:
						return
					}
				}
			}
		}
	}
	return
}

/*
CreatePath will create the full path in zookeeper (emulates 'mkdir -p'). Each
node will be assigned the same data and acl permissions. Only non-ephemeral
nodes can have children.

The path parameter must begin with '/'
*/
func (c *Client) CreatePath(path string, data []byte, acl []zk.ACL) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if !strings.HasPrefix(path, "/") {
		return ErrInvalidPath
	}
	segments := strings.Split(path, "/")
	segments = segments[1:len(segments)]
	var slice []string
	for i := range segments {
		slice = append(slice, segments[i])
		segment := fmt.Sprintf("/%s", strings.Join(slice, "/"))
		exists, _, err := c.Exists(segment)
		if err != nil {
			return err
		}
		if exists != true {
			_, err := c.Create(segment, data, 0, acl)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *Client) WaitToExist(path string, maxWaitTime time.Duration) (err error) {
	retryCount := 0
	operation := func() error {
		spew.Printf("[curator] Client#WaitToExist: path %v (retry=%d)\n", path, retryCount)
		exists, _, err := c.Exists(path)
		if err == nil && !exists {
			err = ErrInvalidPath
		}
		retryCount++
		return err
	}
	expBackoff := backoff.NewExponentialBackOff()
	expBackoff.MaxElapsedTime = maxWaitTime
	backoff.Retry(operation, expBackoff)
	return err
}
