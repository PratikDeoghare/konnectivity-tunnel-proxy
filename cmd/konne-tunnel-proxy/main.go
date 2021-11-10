package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/elazarl/goproxy"
	"google.golang.org/grpc"
	"sigs.k8s.io/apiserver-network-proxy/konnectivity-client/pkg/client"
)

func main() {

	var proxyUDSName string
	flag.StringVar(&proxyUDSName, "proxy-uds", "/etc/kubernetes/konnectivity-server/konnectivity-server.socket", "konnectivity-benchmate socket name of konnectivity proxy")

	var nodeIP string
	flag.StringVar(&nodeIP, "node-ip", "127.0.0.1", "ip of node where benchmate server is running")

	flag.Parse()

	dialOption := grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		c, err := net.DialTimeout("unix", proxyUDSName, 0)
		if err != nil {
			return nil, fmt.Errorf("failed to create connection to %s: %+v", proxyUDSName, err)
		}
		return c, err
	})

	ctx := context.Background()
	tunnel, err := client.CreateSingleUseGrpcTunnel(ctx, proxyUDSName, dialOption, grpc.WithInsecure(), grpc.WithUserAgent("o.userAgent"))
	if err != nil {
		panic(err)
	}


	c := http.Client{
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           tunnel.DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}

	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = true
	proxy.OnRequest().DoFunc(
		func(r *http.Request,ctx *goproxy.ProxyCtx)(*http.Request,*http.Response) {
			resp, err := c.Do(r)
			if err != nil {
				log.Printf("error: %+v", err)
				return r, nil
			}
			return r, resp
		})

	log.Fatal(http.ListenAndServe(":8080", proxy))

}
