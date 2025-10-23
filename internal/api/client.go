package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/deeprpa/fuck-gpu/internal/daemon"
	"github.com/sirupsen/logrus"
)

const (
	ServeAddr = ":35700"
)

type Client struct {
	httpCli *http.Client
}

func NewClient() *Client {
	return &Client{
		httpCli: &http.Client{},
	}
}

func (c *Client) Ping() (*http.Response, error) {
	resp, err := c.Get("ping")
	if err != nil {
		logrus.Errorf("ping failed, %s", err)
		return nil, err
	}

	bs, _ := ioutil.ReadAll(resp.Body)
	logrus.Infof("body %s", bs)
	return resp, err
}

func (c *Client) Status() (*daemon.DaemonStatus, error) {
	resp, err := c.Get("status")
	if err != nil {
		logrus.Errorf("status failed, %s", err)
		return nil, err
	}

	ds := &daemon.DaemonStatus{}
	err = json.NewDecoder(resp.Body).Decode(ds)

	return ds, err
}

func (c *Client) StopUpgrader() error {
	_, err := c.Get("stop_upgrader")
	return err
}
func (c *Client) StartUpgrader() error {
	_, err := c.Get("start_upgrader")
	return err
}
func (c *Client) ExitSpare() error {
	_, err := c.Get("exit_spare")
	return err
}
func (c *Client) Restart() error {
	_, err := c.Get("restart")
	return err
}

func (c *Client) Upgrade() error {
	_, err := c.Get("upgrade")
	return err
}

func (c *Client) Get(path string) (*http.Response, error) {
	url := fmt.Sprintf("http://127.0.0.1%s/%s", ServeAddr, path)
	return c.httpCli.Get(url)
}
