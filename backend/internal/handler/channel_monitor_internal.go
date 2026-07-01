package handler

import (
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func channelMonitorRequestContext(c *gin.Context, signer *service.ChannelMonitorSigner) (map[int64]struct{}, bool) {
	if c == nil || signer == nil || c.Request == nil {
		return nil, false
	}
	ids, ok := signer.VerifyRequest(c.Request.Header, c.Request.RemoteAddr, time.Now())
	if !ok {
		return nil, false
	}
	if len(ids) == 0 {
		return nil, true
	}
	out := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id > 0 {
			out[id] = struct{}{}
		}
	}
	return out, true
}

func writeChannelMonitorSelectedAccountHeader(c *gin.Context, accountID int64) {
	if c == nil || accountID <= 0 {
		return
	}
	c.Header(service.ChannelMonitorHeaderSelectedAccount, strconv.FormatInt(accountID, 10))
}
