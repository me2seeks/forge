package proxy

import (
	"context"
	"net"
	"net/url"
	"os"

	"github.com/me2seeks/forge/ctxcache"
	"github.com/me2seeks/forge/logs"
	"github.com/me2seeks/forge/types/consts"
)

type (
	RequestSchemeKeyInCtx struct{}
	HostKeyInCtx          struct{}
)

func CheckIfNeedReplaceHost(ctx context.Context, originURLStr string) (ok bool, proxyURL string) {
	// url parse
	originURL, err := url.Parse(originURLStr)
	if err != nil {
		logs.CtxWarnf(ctx, "[CheckIfNeedReplaceHost] url parse failed, err: %v", err)
		return false, ""
	}

	proxyPort := os.Getenv(consts.MinIOProxyEndpoint) // :8889
	if proxyPort == "" {
		return false, ""
	}

	currentHost, ok := ctxcache.Get[string](ctx, HostKeyInCtx{})
	if !ok {
		return false, ""
	}

	currentScheme, ok := ctxcache.Get[string](ctx, RequestSchemeKeyInCtx{})
	if !ok {
		return false, ""
	}

	host, _, err := net.SplitHostPort(currentHost)
	if err != nil {
		host = currentHost
	}

	minioProxyHost := host + proxyPort
	originURL.Host = minioProxyHost
	originURL.Scheme = currentScheme
	logs.CtxDebugf(ctx, "[CheckIfNeedReplaceHost] reset originURL.String = %s", originURL.String())
	return true, originURL.String()
}
