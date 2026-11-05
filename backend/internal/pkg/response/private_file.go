package response

import (
	"mime"
	"net/http"

	"github.com/gin-gonic/gin"
)

// PrivateFile 以附件形式返回需要登录才能取的文件：强制下载、禁止类型嗅探、禁止任何缓存。
// 文件名走 RFC 2231 编码，中文名不会乱码。
func PrivateFile(c *gin.Context, filename, contentType string, data []byte) {
	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": filename})
	if disposition == "" {
		disposition = "attachment"
	}
	c.Header("Content-Disposition", disposition)
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "private, no-store")
	c.Data(http.StatusOK, contentType, data)
}
